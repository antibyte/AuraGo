var Qi={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},es={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},Cd=0,mh=1,Pd=2;var Io=1,pl=2,Er=3,Tn=0,jt=1,At=2,On=0,ys=1,kt=2,gh=3,xh=4,Id=5;var Yi=100,Ld=101,Dd=102,Nd=103,Ud=104,Fd=200,Od=201,Bd=202,zd=203,Da=204,Na=205,kd=206,Hd=207,Vd=208,Gd=209,Wd=210,Xd=211,qd=212,Yd=213,Zd=214,Ua=0,Fa=1,Oa=2,Ms=3,Ba=4,za=5,ka=6,Ha=7,ml=0,Kd=1,jd=2,jn=0,Lo=1,Do=2,No=3,Ls=4,Uo=5,Fo=6,Oo=7,ih="attached",Jd="detached",_h=300,ts=301,Ds=302,gl=303,xl=304,Bo=306,ri=1e3,Un=1001,or=1002,Wt=1003,_l=1004;var Ns=1005;var Ft=1006,Tr=1007;var Jn=1008;var yn=1009,vh=1010,yh=1011,Ar=1012,vl=1013,$n=1014,Cn=1015,Jt=1016,yl=1017,Ml=1018,Rr=1020,Mh=35902,bh=35899,Sh=1021,wh=1022,Pn=1023,oi=1026,ns=1027,bl=1028,Sl=1029,is=1030,wl=1031;var El=1033,zo=33776,ko=33777,Ho=33778,Vo=33779,Tl=35840,Al=35841,Rl=35842,Cl=35843,Pl=36196,Il=37492,Ll=37496,Dl=37488,Nl=37489,Go=37490,Ul=37491,Fl=37808,Ol=37809,Bl=37810,zl=37811,kl=37812,Hl=37813,Vl=37814,Gl=37815,Wl=37816,Xl=37817,ql=37818,Yl=37819,Zl=37820,Kl=37821,jl=36492,Jl=36494,$l=36495,Ql=36283,ec=36284,Wo=36285,tc=36286,nc=2200,$d=2201,Qd=2202,bs=2300,Ss=2301,La=2302,sh=2303,xs=2400,_s=2401,to=2402,ic=2500,ef=2501,Eh=0,Xo=1,Cr=2,tf=3200;var qo=0,nf=1,Bn="",Ut="srgb",dn="srgb-linear",no="linear",yt="srgb";var gs=7680;var rh=519,sf=512,rf=513,of=514,sc=515,af=516,lf=517,rc=518,cf=519,Va=35044;var Th="300 es",qn=2e3,ar=2001;function Vp(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Gp(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function lr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function hf(){let s=lr("canvas");return s.style.display="block",s}var Hu={},cr=null;function io(...s){let e="THREE."+s.shift();cr?cr("log",e,...s):console.log(e,...s)}function uf(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function Ze(...s){s=uf(s);let e="THREE."+s.shift();if(cr)cr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function $e(...s){s=uf(s);let e="THREE."+s.shift();if(cr)cr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function vs(...s){let e=s.join(" ");e in Hu||(Hu[e]=!0,Ze(...s))}function df(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var ff={[Ua]:Fa,[Oa]:ka,[Ba]:Ha,[Ms]:za,[Fa]:Ua,[ka]:Oa,[Ha]:Ba,[za]:Ms},Fn=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,o=i.length;r<o;r++)i[r].call(this,e);e.target=null}}},rn=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],Vu=1234567,Jr=Math.PI/180,ws=180/Math.PI;function Yn(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(rn[s&255]+rn[s>>8&255]+rn[s>>16&255]+rn[s>>24&255]+"-"+rn[e&255]+rn[e>>8&255]+"-"+rn[e>>16&15|64]+rn[e>>24&255]+"-"+rn[t&63|128]+rn[t>>8&255]+"-"+rn[t>>16&255]+rn[t>>24&255]+rn[n&255]+rn[n>>8&255]+rn[n>>16&255]+rn[n>>24&255]).toLowerCase()}function ct(s,e,t){return Math.max(e,Math.min(t,s))}function Ah(s,e){return(s%e+e)%e}function Wp(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function Xp(s,e,t){return s!==e?(t-s)/(e-s):0}function $r(s,e,t){return(1-t)*s+t*e}function qp(s,e,t,n){return $r(s,e,1-Math.exp(-t*n))}function Yp(s,e=1){return e-Math.abs(Ah(s,e*2)-e)}function Zp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function Kp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function jp(s,e){return s+Math.floor(Math.random()*(e-s+1))}function Jp(s,e){return s+Math.random()*(e-s)}function $p(s){return s*(.5-Math.random())}function Qp(s){s!==void 0&&(Vu=s);let e=Vu+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function em(s){return s*Jr}function tm(s){return s*ws}function nm(s){return(s&s-1)===0&&s!==0}function im(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function sm(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function rm(s,e,t,n,i){let r=Math.cos,o=Math.sin,a=r(t/2),l=o(t/2),c=r((e+n)/2),h=o((e+n)/2),u=r((e-n)/2),d=o((e-n)/2),f=r((n-e)/2),g=o((n-e)/2);switch(i){case"XYX":s.set(a*h,l*u,l*d,a*c);break;case"YZY":s.set(l*d,a*h,l*u,a*c);break;case"ZXZ":s.set(l*u,l*d,a*h,a*c);break;case"XZX":s.set(a*h,l*g,l*f,a*c);break;case"YXY":s.set(l*f,a*h,l*g,a*c);break;case"ZYZ":s.set(l*g,l*f,a*h,a*c);break;default:Ze("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function Xn(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function wt(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var _t={DEG2RAD:Jr,RAD2DEG:ws,generateUUID:Yn,clamp:ct,euclideanModulo:Ah,mapLinear:Wp,inverseLerp:Xp,lerp:$r,damp:qp,pingpong:Yp,smoothstep:Zp,smootherstep:Kp,randInt:jp,randFloat:Jp,randFloatSpread:$p,seededRandom:Qp,degToRad:em,radToDeg:tm,isPowerOfTwo:nm,ceilPowerOfTwo:im,floorPowerOfTwo:sm,setQuaternionFromProperEuler:rm,normalize:wt,denormalize:Xn},Re=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=ct(this.x,e.x,t.x),this.y=ct(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=ct(this.x,e,t),this.y=ct(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ct(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(ct(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,o=this.y-e.y;return this.x=r*n-o*i+e.x,this.y=r*i+o*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},Gt=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,o,a){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],d=r[o+0],f=r[o+1],g=r[o+2],v=r[o+3];if(u!==v||l!==d||c!==f||h!==g){let m=l*d+c*f+h*g+u*v;m<0&&(d=-d,f=-f,g=-g,v=-v,m=-m);let p=1-a;if(m<.9995){let y=Math.acos(m),w=Math.sin(y);p=Math.sin(p*y)/w,a=Math.sin(a*y)/w,l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+v*a}else{l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+v*a;let y=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=y,c*=y,h*=y,u*=y}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,o){let a=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[o],d=r[o+1],f=r[o+2],g=r[o+3];return e[t]=a*g+h*u+l*f-c*d,e[t+1]=l*g+h*d+c*u-a*f,e[t+2]=c*g+h*f+a*d-l*u,e[t+3]=h*g-a*u-l*d-c*f,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,o=e._order,a=Math.cos,l=Math.sin,c=a(n/2),h=a(i/2),u=a(r/2),d=l(n/2),f=l(i/2),g=l(r/2);switch(o){case"XYZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"YXZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"ZXY":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"ZYX":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"YZX":this._x=d*h*u+c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u-d*f*g;break;case"XZY":this._x=d*h*u-c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u+d*f*g;break;default:Ze("Quaternion: .setFromEuler() encountered an unknown order: "+o)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],o=t[1],a=t[5],l=t[9],c=t[2],h=t[6],u=t[10],d=n+a+u;if(d>0){let f=.5/Math.sqrt(d+1);this._w=.25/f,this._x=(h-l)*f,this._y=(r-c)*f,this._z=(o-i)*f}else if(n>a&&n>u){let f=2*Math.sqrt(1+n-a-u);this._w=(h-l)/f,this._x=.25*f,this._y=(i+o)/f,this._z=(r+c)/f}else if(a>u){let f=2*Math.sqrt(1+a-n-u);this._w=(r-c)/f,this._x=(i+o)/f,this._y=.25*f,this._z=(l+h)/f}else{let f=2*Math.sqrt(1+u-n-a);this._w=(o-i)/f,this._x=(r+c)/f,this._y=(l+h)/f,this._z=.25*f}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(ct(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+o*a+i*c-r*l,this._y=i*h+o*l+r*a-n*c,this._z=r*h+o*c+n*l-i*a,this._w=o*h-n*a-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=this.dot(e);a<0&&(n=-n,i=-i,r=-r,o=-o,a=-a);let l=1-t;if(a<.9995){let c=Math.acos(a),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},z=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(Gu.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(Gu.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,o=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*o,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*o,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*o,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,o=e.y,a=e.z,l=e.w,c=2*(o*i-a*n),h=2*(a*t-r*i),u=2*(r*n-o*t);return this.x=t+l*c+o*u-a*h,this.y=n+l*h+a*c-r*u,this.z=i+l*u+r*h-o*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=ct(this.x,e.x,t.x),this.y=ct(this.y,e.y,t.y),this.z=ct(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=ct(this.x,e,t),this.y=ct(this.y,e,t),this.z=ct(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ct(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,o=t.x,a=t.y,l=t.z;return this.x=i*l-r*a,this.y=r*o-n*l,this.z=n*a-i*o,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return Rc.copy(this).projectOnVector(e),this.sub(Rc)}reflect(e){return this.sub(Rc.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(ct(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},Rc=new z,Gu=new Gt,ot=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,o,a,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c)}set(e,t,n,i,r,o,a,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=a,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=o,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[3],l=n[6],c=n[1],h=n[4],u=n[7],d=n[2],f=n[5],g=n[8],v=i[0],m=i[3],p=i[6],y=i[1],w=i[4],x=i[7],A=i[2],T=i[5],I=i[8];return r[0]=o*v+a*y+l*A,r[3]=o*m+a*w+l*T,r[6]=o*p+a*x+l*I,r[1]=c*v+h*y+u*A,r[4]=c*m+h*w+u*T,r[7]=c*p+h*x+u*I,r[2]=d*v+f*y+g*A,r[5]=d*m+f*w+g*T,r[8]=d*p+f*x+g*I,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8];return t*o*h-t*a*c-n*r*h+n*a*l+i*r*c-i*o*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=h*o-a*c,d=a*l-h*r,f=c*r-o*l,g=t*u+n*d+i*f;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let v=1/g;return e[0]=u*v,e[1]=(i*c-h*n)*v,e[2]=(a*n-i*o)*v,e[3]=d*v,e[4]=(h*t-i*l)*v,e[5]=(i*r-a*t)*v,e[6]=f*v,e[7]=(n*l-c*t)*v,e[8]=(o*t-n*r)*v,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,o,a){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*o+c*a)+o+e,-i*c,i*l,-i*(-c*o+l*a)+a+t,0,0,1),this}scale(e,t){return vs("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Cc.makeScale(e,t)),this}rotate(e){return vs("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Cc.makeRotation(-e)),this}translate(e,t){return vs("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Cc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Cc=new ot,Wu=new ot().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),Xu=new ot().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function om(){let s={enabled:!0,workingColorSpace:dn,spaces:{},convert:function(i,r,o){return this.enabled===!1||r===o||!r||!o||(this.spaces[r].transfer===yt&&(i.r=Ti(i.r),i.g=Ti(i.g),i.b=Ti(i.b)),this.spaces[r].primaries!==this.spaces[o].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[o].fromXYZ)),this.spaces[o].transfer===yt&&(i.r=rr(i.r),i.g=rr(i.g),i.b=rr(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===Bn?no:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,o){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[o].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return vs("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return vs("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[dn]:{primaries:e,whitePoint:n,transfer:no,toXYZ:Wu,fromXYZ:Xu,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:Ut},outputColorSpaceConfig:{drawingBufferColorSpace:Ut}},[Ut]:{primaries:e,whitePoint:n,transfer:yt,toXYZ:Wu,fromXYZ:Xu,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:Ut}}}),s}var lt=om();function Ti(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function rr(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var Xs,Ga=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{Xs===void 0&&(Xs=lr("canvas")),Xs.width=e.width,Xs.height=e.height;let i=Xs.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=Xs}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=lr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let o=0;o<r.length;o++)r[o]=Ti(r[o]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(Ti(t[n]/255)*255):t[n]=Ti(t[n]);return{data:t,width:e.width,height:e.height}}else return Ze("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},am=0,hr=class{constructor(e=null){this.isSource=!0,Object.defineProperty(this,"id",{value:am++}),this.uuid=Yn(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let o=0,a=i.length;o<a;o++)i[o].isDataTexture?r.push(Pc(i[o].image)):r.push(Pc(i[o]))}else r=Pc(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function Pc(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Ga.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(Ze("Texture: Unable to serialize Texture."),{})}var lm=0,Ic=new z,Xt=class s extends Fn{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=Un,i=Un,r=Ft,o=Jn,a=Pn,l=yn,c=s.DEFAULT_ANISOTROPY,h=Bn){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:lm++}),this.uuid=Yn(),this.name="",this.source=new hr(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=o,this.anisotropy=c,this.format=a,this.internalFormat=null,this.type=l,this.offset=new Re(0,0),this.repeat=new Re(1,1),this.center=new Re(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new ot,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Ic).x}get height(){return this.source.getSize(Ic).y}get depth(){return this.source.getSize(Ic).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){Ze(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Ze(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==_h)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case ri:e.x=e.x-Math.floor(e.x);break;case Un:e.x=e.x<0?0:1;break;case or:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case ri:e.y=e.y-Math.floor(e.y);break;case Un:e.y=e.y<0?0:1;break;case or:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};Xt.DEFAULT_IMAGE=null;Xt.DEFAULT_MAPPING=_h;Xt.DEFAULT_ANISOTROPY=1;var xt=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,o=e.elements;return this.x=o[0]*t+o[4]*n+o[8]*i+o[12]*r,this.y=o[1]*t+o[5]*n+o[9]*i+o[13]*r,this.z=o[2]*t+o[6]*n+o[10]*i+o[14]*r,this.w=o[3]*t+o[7]*n+o[11]*i+o[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],d=l[1],f=l[5],g=l[9],v=l[2],m=l[6],p=l[10];if(Math.abs(h-d)<.01&&Math.abs(u-v)<.01&&Math.abs(g-m)<.01){if(Math.abs(h+d)<.1&&Math.abs(u+v)<.1&&Math.abs(g+m)<.1&&Math.abs(c+f+p-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let w=(c+1)/2,x=(f+1)/2,A=(p+1)/2,T=(h+d)/4,I=(u+v)/4,_=(g+m)/4;return w>x&&w>A?w<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(w),i=T/n,r=I/n):x>A?x<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(x),n=T/i,r=_/i):A<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(A),n=I/r,i=_/r),this.set(n,i,r,t),this}let y=Math.sqrt((m-g)*(m-g)+(u-v)*(u-v)+(d-h)*(d-h));return Math.abs(y)<.001&&(y=1),this.x=(m-g)/y,this.y=(u-v)/y,this.z=(d-h)/y,this.w=Math.acos((c+f+p-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=ct(this.x,e.x,t.x),this.y=ct(this.y,e.y,t.y),this.z=ct(this.z,e.z,t.z),this.w=ct(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=ct(this.x,e,t),this.y=ct(this.y,e,t),this.z=ct(this.z,e,t),this.w=ct(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ct(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},Wa=class extends Fn{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:Ft,depthBuffer:!0,stencilBuffer:!1,resolveDepthBuffer:!0,resolveStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new xt(0,0,e,t),this.scissorTest=!1,this.viewport=new xt(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new Xt(i),o=n.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0,this.textures[a].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:Ft,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&(this._depthTexture.renderTarget=null),e!==null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new hr(i)}return this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,e.depthTexture!==null&&(this.depthTexture=e.depthTexture.clone()),this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},zt=class extends Wa{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},so=class extends Xt{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Wt,this.minFilter=Wt,this.wrapR=Un,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var Xa=class extends Xt{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Wt,this.minFilter=Wt,this.wrapR=Un,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var it=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,o,a,l,c,h,u,d,f,g,v,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,v,m)}set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,v,m){let p=this.elements;return p[0]=e,p[4]=t,p[8]=n,p[12]=i,p[1]=r,p[5]=o,p[9]=a,p[13]=l,p[2]=c,p[6]=h,p[10]=u,p[14]=d,p[3]=f,p[7]=g,p[11]=v,p[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/qs.setFromMatrixColumn(e,0).length(),r=1/qs.setFromMatrixColumn(e,1).length(),o=1/qs.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*o,t[9]=n[9]*o,t[10]=n[10]*o,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,o=Math.cos(n),a=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let d=o*h,f=o*u,g=a*h,v=a*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=f+g*c,t[5]=d-v*c,t[9]=-a*l,t[2]=v-d*c,t[6]=g+f*c,t[10]=o*l}else if(e.order==="YXZ"){let d=l*h,f=l*u,g=c*h,v=c*u;t[0]=d+v*a,t[4]=g*a-f,t[8]=o*c,t[1]=o*u,t[5]=o*h,t[9]=-a,t[2]=f*a-g,t[6]=v+d*a,t[10]=o*l}else if(e.order==="ZXY"){let d=l*h,f=l*u,g=c*h,v=c*u;t[0]=d-v*a,t[4]=-o*u,t[8]=g+f*a,t[1]=f+g*a,t[5]=o*h,t[9]=v-d*a,t[2]=-o*c,t[6]=a,t[10]=o*l}else if(e.order==="ZYX"){let d=o*h,f=o*u,g=a*h,v=a*u;t[0]=l*h,t[4]=g*c-f,t[8]=d*c+v,t[1]=l*u,t[5]=v*c+d,t[9]=f*c-g,t[2]=-c,t[6]=a*l,t[10]=o*l}else if(e.order==="YZX"){let d=o*l,f=o*c,g=a*l,v=a*c;t[0]=l*h,t[4]=v-d*u,t[8]=g*u+f,t[1]=u,t[5]=o*h,t[9]=-a*h,t[2]=-c*h,t[6]=f*u+g,t[10]=d-v*u}else if(e.order==="XZY"){let d=o*l,f=o*c,g=a*l,v=a*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=d*u+v,t[5]=o*h,t[9]=f*u-g,t[2]=g*u-f,t[6]=a*h,t[10]=v*u+d}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(cm,e,hm)}lookAt(e,t,n){let i=this.elements;return wn.subVectors(e,t),wn.lengthSq()===0&&(wn.z=1),wn.normalize(),ki.crossVectors(n,wn),ki.lengthSq()===0&&(Math.abs(n.z)===1?wn.x+=1e-4:wn.z+=1e-4,wn.normalize(),ki.crossVectors(n,wn)),ki.normalize(),oa.crossVectors(wn,ki),i[0]=ki.x,i[4]=oa.x,i[8]=wn.x,i[1]=ki.y,i[5]=oa.y,i[9]=wn.y,i[2]=ki.z,i[6]=oa.z,i[10]=wn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[4],l=n[8],c=n[12],h=n[1],u=n[5],d=n[9],f=n[13],g=n[2],v=n[6],m=n[10],p=n[14],y=n[3],w=n[7],x=n[11],A=n[15],T=i[0],I=i[4],_=i[8],D=i[12],E=i[1],b=i[5],P=i[9],W=i[13],R=i[2],U=i[6],F=i[10],S=i[14],X=i[3],Z=i[7],V=i[11],ie=i[15];return r[0]=o*T+a*E+l*R+c*X,r[4]=o*I+a*b+l*U+c*Z,r[8]=o*_+a*P+l*F+c*V,r[12]=o*D+a*W+l*S+c*ie,r[1]=h*T+u*E+d*R+f*X,r[5]=h*I+u*b+d*U+f*Z,r[9]=h*_+u*P+d*F+f*V,r[13]=h*D+u*W+d*S+f*ie,r[2]=g*T+v*E+m*R+p*X,r[6]=g*I+v*b+m*U+p*Z,r[10]=g*_+v*P+m*F+p*V,r[14]=g*D+v*W+m*S+p*ie,r[3]=y*T+w*E+x*R+A*X,r[7]=y*I+w*b+x*U+A*Z,r[11]=y*_+w*P+x*F+A*V,r[15]=y*D+w*W+x*S+A*ie,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],o=e[1],a=e[5],l=e[9],c=e[13],h=e[2],u=e[6],d=e[10],f=e[14],g=e[3],v=e[7],m=e[11],p=e[15],y=l*f-c*d,w=a*f-c*u,x=a*d-l*u,A=o*f-c*h,T=o*d-l*h,I=o*u-a*h;return t*(v*y-m*w+p*x)-n*(g*y-m*A+p*T)+i*(g*w-v*A+p*I)-r*(g*x-v*T+m*I)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],o=e[5],a=e[9],l=e[2],c=e[6],h=e[10];return t*(o*h-a*c)-n*(r*h-a*l)+i*(r*c-o*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=e[9],d=e[10],f=e[11],g=e[12],v=e[13],m=e[14],p=e[15],y=t*a-n*o,w=t*l-i*o,x=t*c-r*o,A=n*l-i*a,T=n*c-r*a,I=i*c-r*l,_=h*v-u*g,D=h*m-d*g,E=h*p-f*g,b=u*m-d*v,P=u*p-f*v,W=d*p-f*m,R=y*W-w*P+x*b+A*E-T*D+I*_;if(R===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let U=1/R;return e[0]=(a*W-l*P+c*b)*U,e[1]=(i*P-n*W-r*b)*U,e[2]=(v*I-m*T+p*A)*U,e[3]=(d*T-u*I-f*A)*U,e[4]=(l*E-o*W-c*D)*U,e[5]=(t*W-i*E+r*D)*U,e[6]=(m*x-g*I-p*w)*U,e[7]=(h*I-d*x+f*w)*U,e[8]=(o*P-a*E+c*_)*U,e[9]=(n*E-t*P-r*_)*U,e[10]=(g*T-v*x+p*y)*U,e[11]=(u*x-h*T-f*y)*U,e[12]=(a*D-o*b-l*_)*U,e[13]=(t*b-n*D+i*_)*U,e[14]=(v*w-g*A-m*y)*U,e[15]=(h*A-u*w+d*y)*U,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,o=e.x,a=e.y,l=e.z,c=r*o,h=r*a;return this.set(c*o+n,c*a-i*l,c*l+i*a,0,c*a+i*l,h*a+n,h*l-i*o,0,c*l-i*a,h*l+i*o,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,o){return this.set(1,n,r,0,e,1,o,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,o=t._y,a=t._z,l=t._w,c=r+r,h=o+o,u=a+a,d=r*c,f=r*h,g=r*u,v=o*h,m=o*u,p=a*u,y=l*c,w=l*h,x=l*u,A=n.x,T=n.y,I=n.z;return i[0]=(1-(v+p))*A,i[1]=(f+x)*A,i[2]=(g-w)*A,i[3]=0,i[4]=(f-x)*T,i[5]=(1-(d+p))*T,i[6]=(m+y)*T,i[7]=0,i[8]=(g+w)*I,i[9]=(m-y)*I,i[10]=(1-(d+v))*I,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let o=qs.set(i[0],i[1],i[2]).length(),a=qs.set(i[4],i[5],i[6]).length(),l=qs.set(i[8],i[9],i[10]).length();r<0&&(o=-o),Vn.copy(this);let c=1/o,h=1/a,u=1/l;return Vn.elements[0]*=c,Vn.elements[1]*=c,Vn.elements[2]*=c,Vn.elements[4]*=h,Vn.elements[5]*=h,Vn.elements[6]*=h,Vn.elements[8]*=u,Vn.elements[9]*=u,Vn.elements[10]*=u,t.setFromRotationMatrix(Vn),n.x=o,n.y=a,n.z=l,this}makePerspective(e,t,n,i,r,o,a=qn,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),d=(t+e)/(t-e),f=(n+i)/(n-i),g,v;if(l)g=r/(o-r),v=o*r/(o-r);else if(a===qn)g=-(o+r)/(o-r),v=-2*o*r/(o-r);else if(a===ar)g=-o/(o-r),v=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=u,c[9]=f,c[13]=0,c[2]=0,c[6]=0,c[10]=g,c[14]=v,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,o,a=qn,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),d=-(t+e)/(t-e),f=-(n+i)/(n-i),g,v;if(l)g=1/(o-r),v=o/(o-r);else if(a===qn)g=-2/(o-r),v=-(o+r)/(o-r);else if(a===ar)g=-1/(o-r),v=-r/(o-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=0,c[12]=d,c[1]=0,c[5]=u,c[9]=0,c[13]=f,c[2]=0,c[6]=0,c[10]=g,c[14]=v,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},qs=new z,Vn=new it,cm=new z(0,0,0),hm=new z(1,1,1),ki=new z,oa=new z,wn=new z,qu=new it,Yu=new Gt,mn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],o=i[4],a=i[8],l=i[1],c=i[5],h=i[9],u=i[2],d=i[6],f=i[10];switch(t){case"XYZ":this._y=Math.asin(ct(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-h,f),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(d,c),this._z=0);break;case"YXZ":this._x=Math.asin(-ct(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(a,f),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(ct(d,-1,1)),Math.abs(d)<.9999999?(this._y=Math.atan2(-u,f),this._z=Math.atan2(-o,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-ct(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(d,f),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-o,c));break;case"YZX":this._z=Math.asin(ct(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(a,f));break;case"XZY":this._z=Math.asin(-ct(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(d,c),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-h,f),this._y=0);break;default:Ze("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return qu.makeRotationFromQuaternion(e),this.setFromRotationMatrix(qu,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return Yu.setFromEuler(this),this.setFromQuaternion(Yu,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};mn.DEFAULT_ORDER="XYZ";var ur=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},um=0,Zu=new z,Ys=new Gt,yi=new it,aa=new z,Gr=new z,dm=new z,fm=new Gt,Ku=new z(1,0,0),ju=new z(0,1,0),Ju=new z(0,0,1),$u={type:"added"},pm={type:"removed"},Zs={type:"childadded",child:null},Lc={type:"childremoved",child:null},bt=class s extends Fn{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:um++}),this.uuid=Yn(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new z,t=new mn,n=new Gt,i=new z(1,1,1);function r(){n.setFromEuler(t,!1)}function o(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new it},normalMatrix:{value:new ot}}),this.matrix=new it,this.matrixWorld=new it,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new ur,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return Ys.setFromAxisAngle(e,t),this.quaternion.multiply(Ys),this}rotateOnWorldAxis(e,t){return Ys.setFromAxisAngle(e,t),this.quaternion.premultiply(Ys),this}rotateX(e){return this.rotateOnAxis(Ku,e)}rotateY(e){return this.rotateOnAxis(ju,e)}rotateZ(e){return this.rotateOnAxis(Ju,e)}translateOnAxis(e,t){return Zu.copy(e).applyQuaternion(this.quaternion),this.position.add(Zu.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(Ku,e)}translateY(e){return this.translateOnAxis(ju,e)}translateZ(e){return this.translateOnAxis(Ju,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(yi.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?aa.copy(e):aa.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),Gr.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?yi.lookAt(Gr,aa,this.up):yi.lookAt(aa,Gr,this.up),this.quaternion.setFromRotationMatrix(yi),i&&(yi.extractRotation(i.matrixWorld),Ys.setFromRotationMatrix(yi),this.quaternion.premultiply(Ys.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?($e("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent($u),Zs.child=e,this.dispatchEvent(Zs),Zs.child=null):$e("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(pm),Lc.child=e,this.dispatchEvent(Lc),Lc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),yi.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),yi.multiply(e.parent.matrixWorld)),e.applyMatrix4(yi),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent($u),Zs.child=e,this.dispatchEvent(Zs),Zs.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let o=this.children[n].getObjectByProperty(e,t);if(o!==void 0)return o}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,o=i.length;r<o;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Gr,e,dm),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Gr,fm,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let o=0,a=r.length;o<a;o++)r[o].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,this.name!==""&&(i.name=this.name),this.castShadow===!0&&(i.castShadow=!0),this.receiveShadow===!0&&(i.receiveShadow=!0),this.visible===!1&&(i.visible=!1),this.frustumCulled===!1&&(i.frustumCulled=!1),this.renderOrder!==0&&(i.renderOrder=this.renderOrder),this.static!==!1&&(i.static=this.static),Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.matrixAutoUpdate===!1&&(i.matrixAutoUpdate=!1),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(a=>({...a,boundingBox:a.boundingBox?a.boundingBox.toJSON():void 0,boundingSphere:a.boundingSphere?a.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(a=>({...a})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(a,l){return a[l.uuid]===void 0&&(a[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let l=a.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let l=0,c=this.material.length;l<c;l++)a.push(r(e.materials,this.material[l]));i.material=a}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let a=0;a<this.children.length;a++)i.children.push(this.children[a].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let a=0;a<this.animations.length;a++){let l=this.animations[a];i.animations.push(r(e.animations,l))}}if(t){let a=o(e.geometries),l=o(e.materials),c=o(e.textures),h=o(e.images),u=o(e.shapes),d=o(e.skeletons),f=o(e.animations),g=o(e.nodes);a.length>0&&(n.geometries=a),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),d.length>0&&(n.skeletons=d),f.length>0&&(n.animations=f),g.length>0&&(n.nodes=g)}return n.object=i,n;function o(a){let l=[];for(let c in a){let h=a[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}};bt.DEFAULT_UP=new z(0,1,0);bt.DEFAULT_MATRIX_AUTO_UPDATE=!0;bt.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var dt=class extends bt{constructor(){super(),this.isGroup=!0,this.type="Group"}},mm={type:"move"},dr=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new dt,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new dt,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new z,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new z),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new dt,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new z,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new z,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,o=null,a=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){o=!0;for(let v of e.hand.values()){let m=t.getJointPose(v,n),p=this._getHandJoint(c,v);m!==null&&(p.matrix.fromArray(m.transform.matrix),p.matrix.decompose(p.position,p.rotation,p.scale),p.matrixWorldNeedsUpdate=!0,p.jointRadius=m.radius),p.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],d=h.position.distanceTo(u.position),f=.02,g=.005;c.inputState.pinching&&d>f+g?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&d<=f-g&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));a!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(a.matrix.fromArray(i.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,i.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(i.linearVelocity)):a.hasLinearVelocity=!1,i.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(i.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(mm)))}return a!==null&&(a.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=o!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new dt;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},pf={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Hi={h:0,s:0,l:0},la={h:0,s:0,l:0};function Dc(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var be=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=Ut){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,lt.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=lt.workingColorSpace){return this.r=e,this.g=t,this.b=n,lt.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=lt.workingColorSpace){if(e=Ah(e,1),t=ct(t,0,1),n=ct(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,o=2*n-r;this.r=Dc(o,r,e+1/3),this.g=Dc(o,r,e),this.b=Dc(o,r,e-1/3)}return lt.colorSpaceToWorking(this,i),this}setStyle(e,t=Ut){function n(r){r!==void 0&&parseFloat(r)<1&&Ze("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,o=i[1],a=i[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:Ze("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(o===6)return this.setHex(parseInt(r,16),t);Ze("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=Ut){let n=pf[e.toLowerCase()];return n!==void 0?this.setHex(n,t):Ze("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=Ti(e.r),this.g=Ti(e.g),this.b=Ti(e.b),this}copyLinearToSRGB(e){return this.r=rr(e.r),this.g=rr(e.g),this.b=rr(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=Ut){return lt.workingToColorSpace(on.copy(this),e),Math.round(ct(on.r*255,0,255))*65536+Math.round(ct(on.g*255,0,255))*256+Math.round(ct(on.b*255,0,255))}getHexString(e=Ut){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=lt.workingColorSpace){lt.workingToColorSpace(on.copy(this),t);let n=on.r,i=on.g,r=on.b,o=Math.max(n,i,r),a=Math.min(n,i,r),l,c,h=(a+o)/2;if(a===o)l=0,c=0;else{let u=o-a;switch(c=h<=.5?u/(o+a):u/(2-o-a),o){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=lt.workingColorSpace){return lt.workingToColorSpace(on.copy(this),t),e.r=on.r,e.g=on.g,e.b=on.b,e}getStyle(e=Ut){lt.workingToColorSpace(on.copy(this),e);let t=on.r,n=on.g,i=on.b;return e!==Ut?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(Hi),this.setHSL(Hi.h+e,Hi.s+t,Hi.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(Hi),e.getHSL(la);let n=$r(Hi.h,la.h,t),i=$r(Hi.s,la.s,t),r=$r(Hi.l,la.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},on=new be;be.NAMES=pf;var ro=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new be(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var Es=class extends bt{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new mn,this.environmentIntensity=1,this.environmentRotation=new mn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),this.backgroundBlurriness>0&&(t.object.backgroundBlurriness=this.backgroundBlurriness),this.backgroundIntensity!==1&&(t.object.backgroundIntensity=this.backgroundIntensity),t.object.backgroundRotation=this.backgroundRotation.toArray(),this.environmentIntensity!==1&&(t.object.environmentIntensity=this.environmentIntensity),t.object.environmentRotation=this.environmentRotation.toArray(),t}},Gn=new z,Mi=new z,Nc=new z,bi=new z,Ks=new z,js=new z,Qu=new z,Uc=new z,Fc=new z,Oc=new z,Bc=new xt,zc=new xt,kc=new xt,qi=class s{constructor(e=new z,t=new z,n=new z){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),Gn.subVectors(e,t),i.cross(Gn);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){Gn.subVectors(i,t),Mi.subVectors(n,t),Nc.subVectors(e,t);let o=Gn.dot(Gn),a=Gn.dot(Mi),l=Gn.dot(Nc),c=Mi.dot(Mi),h=Mi.dot(Nc),u=o*c-a*a;if(u===0)return r.set(0,0,0),null;let d=1/u,f=(c*l-a*h)*d,g=(o*h-a*l)*d;return r.set(1-f-g,g,f)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,bi)===null?!1:bi.x>=0&&bi.y>=0&&bi.x+bi.y<=1}static getInterpolation(e,t,n,i,r,o,a,l){return this.getBarycoord(e,t,n,i,bi)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,bi.x),l.addScaledVector(o,bi.y),l.addScaledVector(a,bi.z),l)}static getInterpolatedAttribute(e,t,n,i,r,o){return Bc.setScalar(0),zc.setScalar(0),kc.setScalar(0),Bc.fromBufferAttribute(e,t),zc.fromBufferAttribute(e,n),kc.fromBufferAttribute(e,i),o.setScalar(0),o.addScaledVector(Bc,r.x),o.addScaledVector(zc,r.y),o.addScaledVector(kc,r.z),o}static isFrontFacing(e,t,n,i){return Gn.subVectors(n,t),Mi.subVectors(e,t),Gn.cross(Mi).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return Gn.subVectors(this.c,this.b),Mi.subVectors(this.a,this.b),Gn.cross(Mi).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,o,a;Ks.subVectors(i,n),js.subVectors(r,n),Uc.subVectors(e,n);let l=Ks.dot(Uc),c=js.dot(Uc);if(l<=0&&c<=0)return t.copy(n);Fc.subVectors(e,i);let h=Ks.dot(Fc),u=js.dot(Fc);if(h>=0&&u<=h)return t.copy(i);let d=l*u-h*c;if(d<=0&&l>=0&&h<=0)return o=l/(l-h),t.copy(n).addScaledVector(Ks,o);Oc.subVectors(e,r);let f=Ks.dot(Oc),g=js.dot(Oc);if(g>=0&&f<=g)return t.copy(r);let v=f*c-l*g;if(v<=0&&c>=0&&g<=0)return a=c/(c-g),t.copy(n).addScaledVector(js,a);let m=h*g-f*u;if(m<=0&&u-h>=0&&f-g>=0)return Qu.subVectors(r,i),a=(u-h)/(u-h+(f-g)),t.copy(i).addScaledVector(Qu,a);let p=1/(m+v+d);return o=v*p,a=d*p,t.copy(n).addScaledVector(Ks,o).addScaledVector(js,a)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},Kt=class{constructor(e=new z(1/0,1/0,1/0),t=new z(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(Wn.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(Wn.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=Wn.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)e.isMesh===!0?e.getVertexPosition(o,Wn):Wn.fromBufferAttribute(r,o),Wn.applyMatrix4(e.matrixWorld),this.expandByPoint(Wn);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),ca.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),ca.copy(n.boundingBox)),ca.applyMatrix4(e.matrixWorld),this.union(ca)}let i=e.children;for(let r=0,o=i.length;r<o;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,Wn),Wn.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(Wr),ha.subVectors(this.max,Wr),Js.subVectors(e.a,Wr),$s.subVectors(e.b,Wr),Qs.subVectors(e.c,Wr),Vi.subVectors($s,Js),Gi.subVectors(Qs,$s),ds.subVectors(Js,Qs);let t=[0,-Vi.z,Vi.y,0,-Gi.z,Gi.y,0,-ds.z,ds.y,Vi.z,0,-Vi.x,Gi.z,0,-Gi.x,ds.z,0,-ds.x,-Vi.y,Vi.x,0,-Gi.y,Gi.x,0,-ds.y,ds.x,0];return!Hc(t,Js,$s,Qs,ha)||(t=[1,0,0,0,1,0,0,0,1],!Hc(t,Js,$s,Qs,ha))?!1:(ua.crossVectors(Vi,Gi),t=[ua.x,ua.y,ua.z],Hc(t,Js,$s,Qs,ha))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,Wn).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(Wn).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Si[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Si[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Si[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Si[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Si[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Si[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Si[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Si[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Si),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Si=[new z,new z,new z,new z,new z,new z,new z,new z],Wn=new z,ca=new Kt,Js=new z,$s=new z,Qs=new z,Vi=new z,Gi=new z,ds=new z,Wr=new z,ha=new z,ua=new z,fs=new z;function Hc(s,e,t,n,i){for(let r=0,o=s.length-3;r<=o;r+=3){fs.fromArray(s,r);let a=i.x*Math.abs(fs.x)+i.y*Math.abs(fs.y)+i.z*Math.abs(fs.z),l=e.dot(fs),c=t.dot(fs),h=n.dot(fs);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>a)return!1}return!0}var Yt=new z,da=new Re,gm=0,gt=class extends Fn{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:gm++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Va,this.updateRanges=[],this.gpuType=Cn,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)da.fromBufferAttribute(this,t),da.applyMatrix3(e),this.setXY(t,da.x,da.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)Yt.fromBufferAttribute(this,t),Yt.applyMatrix3(e),this.setXYZ(t,Yt.x,Yt.y,Yt.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)Yt.fromBufferAttribute(this,t),Yt.applyMatrix4(e),this.setXYZ(t,Yt.x,Yt.y,Yt.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Yt.fromBufferAttribute(this,t),Yt.applyNormalMatrix(e),this.setXYZ(t,Yt.x,Yt.y,Yt.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Yt.fromBufferAttribute(this,t),Yt.transformDirection(e),this.setXYZ(t,Yt.x,Yt.y,Yt.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=Xn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=wt(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=Xn(t,this.array)),t}setX(e,t){return this.normalized&&(t=wt(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=Xn(t,this.array)),t}setY(e,t){return this.normalized&&(t=wt(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=Xn(t,this.array)),t}setZ(e,t){return this.normalized&&(t=wt(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=Xn(t,this.array)),t}setW(e,t){return this.normalized&&(t=wt(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=wt(t,this.array),n=wt(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=wt(t,this.array),n=wt(n,this.array),i=wt(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=wt(t,this.array),n=wt(n,this.array),i=wt(i,this.array),r=wt(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return this.name!==""&&(e.name=this.name),this.usage!==Va&&(e.usage=this.usage),e}dispose(){this.dispatchEvent({type:"dispose"})}};var oo=class extends gt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var ao=class extends gt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var et=class extends gt{constructor(e,t,n){super(new Float32Array(e),t,n)}},xm=new Kt,Xr=new z,Vc=new z,an=class{constructor(e=new z,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):xm.setFromPoints(e).getCenter(n);let i=0;for(let r=0,o=e.length;r<o;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;Xr.subVectors(e,this.center);let t=Xr.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(Xr,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(Vc.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(Xr.copy(e.center).add(Vc)),this.expandByPoint(Xr.copy(e.center).sub(Vc))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},_m=0,Dn=new it,Gc=new bt,er=new z,En=new Kt,qr=new Kt,tn=new z,ht=class s extends Fn{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:_m++}),this.uuid=Yn(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Vp(e)?ao:oo)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new ot().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return Dn.makeRotationFromQuaternion(e),this.applyMatrix4(Dn),this}rotateX(e){return Dn.makeRotationX(e),this.applyMatrix4(Dn),this}rotateY(e){return Dn.makeRotationY(e),this.applyMatrix4(Dn),this}rotateZ(e){return Dn.makeRotationZ(e),this.applyMatrix4(Dn),this}translate(e,t,n){return Dn.makeTranslation(e,t,n),this.applyMatrix4(Dn),this}scale(e,t,n){return Dn.makeScale(e,t,n),this.applyMatrix4(Dn),this}lookAt(e){return Gc.lookAt(e),Gc.updateMatrix(),this.applyMatrix4(Gc.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(er).negate(),this.translate(er.x,er.y,er.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let o=e[i];n.push(o.x,o.y,o.z||0)}this.setAttribute("position",new et(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&Ze("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new Kt);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){$e("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new z(-1/0,-1/0,-1/0),new z(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];En.setFromBufferAttribute(r),this.morphTargetsRelative?(tn.addVectors(this.boundingBox.min,En.min),this.boundingBox.expandByPoint(tn),tn.addVectors(this.boundingBox.max,En.max),this.boundingBox.expandByPoint(tn)):(this.boundingBox.expandByPoint(En.min),this.boundingBox.expandByPoint(En.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&$e('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new an);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){$e("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new z,1/0);return}if(e){let n=this.boundingSphere.center;if(En.setFromBufferAttribute(e),t)for(let r=0,o=t.length;r<o;r++){let a=t[r];qr.setFromBufferAttribute(a),this.morphTargetsRelative?(tn.addVectors(En.min,qr.min),En.expandByPoint(tn),tn.addVectors(En.max,qr.max),En.expandByPoint(tn)):(En.expandByPoint(qr.min),En.expandByPoint(qr.max))}En.getCenter(n);let i=0;for(let r=0,o=e.count;r<o;r++)tn.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(tn));if(t)for(let r=0,o=t.length;r<o;r++){let a=t[r],l=this.morphTargetsRelative;for(let c=0,h=a.count;c<h;c++)tn.fromBufferAttribute(a,c),l&&(er.fromBufferAttribute(e,c),tn.add(er)),i=Math.max(i,n.distanceToSquared(tn))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&$e('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){$e("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,o=this.getAttribute("tangent");(o===void 0||o.count!==n.count)&&(o=new gt(new Float32Array(4*n.count),4),this.setAttribute("tangent",o));let a=[],l=[];for(let _=0;_<n.count;_++)a[_]=new z,l[_]=new z;let c=new z,h=new z,u=new z,d=new Re,f=new Re,g=new Re,v=new z,m=new z;function p(_,D,E){c.fromBufferAttribute(n,_),h.fromBufferAttribute(n,D),u.fromBufferAttribute(n,E),d.fromBufferAttribute(r,_),f.fromBufferAttribute(r,D),g.fromBufferAttribute(r,E),h.sub(c),u.sub(c),f.sub(d),g.sub(d);let b=1/(f.x*g.y-g.x*f.y);isFinite(b)&&(v.copy(h).multiplyScalar(g.y).addScaledVector(u,-f.y).multiplyScalar(b),m.copy(u).multiplyScalar(f.x).addScaledVector(h,-g.x).multiplyScalar(b),a[_].add(v),a[D].add(v),a[E].add(v),l[_].add(m),l[D].add(m),l[E].add(m))}let y=this.groups;y.length===0&&(y=[{start:0,count:e.count}]);for(let _=0,D=y.length;_<D;++_){let E=y[_],b=E.start,P=E.count;for(let W=b,R=b+P;W<R;W+=3)p(e.getX(W+0),e.getX(W+1),e.getX(W+2))}let w=new z,x=new z,A=new z,T=new z;function I(_){A.fromBufferAttribute(i,_),T.copy(A);let D=a[_];w.copy(D),w.sub(A.multiplyScalar(A.dot(D))).normalize(),x.crossVectors(T,D);let b=x.dot(l[_])<0?-1:1;o.setXYZW(_,w.x,w.y,w.z,b)}for(let _=0,D=y.length;_<D;++_){let E=y[_],b=E.start,P=E.count;for(let W=b,R=b+P;W<R;W+=3)I(e.getX(W+0)),I(e.getX(W+1)),I(e.getX(W+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new gt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let d=0,f=n.count;d<f;d++)n.setXYZ(d,0,0,0);let i=new z,r=new z,o=new z,a=new z,l=new z,c=new z,h=new z,u=new z;if(e)for(let d=0,f=e.count;d<f;d+=3){let g=e.getX(d+0),v=e.getX(d+1),m=e.getX(d+2);i.fromBufferAttribute(t,g),r.fromBufferAttribute(t,v),o.fromBufferAttribute(t,m),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),a.fromBufferAttribute(n,g),l.fromBufferAttribute(n,v),c.fromBufferAttribute(n,m),a.add(h),l.add(h),c.add(h),n.setXYZ(g,a.x,a.y,a.z),n.setXYZ(v,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let d=0,f=t.count;d<f;d+=3)i.fromBufferAttribute(t,d+0),r.fromBufferAttribute(t,d+1),o.fromBufferAttribute(t,d+2),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),n.setXYZ(d+0,h.x,h.y,h.z),n.setXYZ(d+1,h.x,h.y,h.z),n.setXYZ(d+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)tn.fromBufferAttribute(e,t),tn.normalize(),e.setXYZ(t,tn.x,tn.y,tn.z)}toNonIndexed(){function e(a,l){let c=a.array,h=a.itemSize,u=a.normalized,d=new c.constructor(l.length*h),f=0,g=0;for(let v=0,m=l.length;v<m;v++){a.isInterleavedBufferAttribute?f=l[v]*a.data.stride+a.offset:f=l[v]*h;for(let p=0;p<h;p++)d[g++]=c[f++]}return new gt(d,h,u)}if(this.index===null)return Ze("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let a in i){let l=i[a],c=e(l,n);t.setAttribute(a,c)}let r=this.morphAttributes;for(let a in r){let l=[],c=r[a];for(let h=0,u=c.length;h<u;h++){let d=c[h],f=e(d,n);l.push(f)}t.morphAttributes[a]=l}t.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,l=o.length;a<l;a++){let c=o[a];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,this.name!==""&&(e.name=this.name),Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,d=c.length;u<d;u++){let f=c[u];h.push(f.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(e.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(e.data.boundingSphere=a.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let d=0,f=u.length;d<f;d++)h.push(u[d].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let o=e.groups;for(let c=0,h=o.length;c<h;c++){let u=o[c];this.addGroup(u.start,u.count,u.materialIndex)}let a=e.boundingBox;a!==null&&(this.boundingBox=a.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},fr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Va,this.updateRanges=[],this.version=0,this.uuid=Yn()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=Yn()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){return e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=Yn()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer))),{uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride}}},un=new z,pr=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)un.fromBufferAttribute(this,t),un.applyMatrix4(e),this.setXYZ(t,un.x,un.y,un.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)un.fromBufferAttribute(this,t),un.applyNormalMatrix(e),this.setXYZ(t,un.x,un.y,un.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)un.fromBufferAttribute(this,t),un.transformDirection(e),this.setXYZ(t,un.x,un.y,un.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=Xn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=wt(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=wt(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=wt(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=wt(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=wt(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=Xn(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=Xn(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=Xn(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=Xn(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=wt(t,this.array),n=wt(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=wt(t,this.array),n=wt(n,this.array),i=wt(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=wt(t,this.array),n=wt(n,this.array),i=wt(i,this.array),r=wt(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){io("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new gt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){io("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},vm=0,fn=class extends Fn{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:vm++}),this.uuid=Yn(),this.name="",this.type="Material",this.blending=ys,this.side=Tn,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=Da,this.blendDst=Na,this.blendEquation=Yi,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new be(0,0,0),this.blendAlpha=0,this.depthFunc=Ms,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=rh,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=gs,this.stencilZFail=gs,this.stencilZPass=gs,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){Ze(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Ze(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,this.name!==""&&(n.name=this.name),this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&this.emissiveIntensity!==1&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&this.attenuationDistance!==1/0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.shadowSide!==null&&(n.shadowSide=this.shadowSide),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),this.blending!==ys&&(n.blending=this.blending),this.side!==Tn&&(n.side=this.side),this.vertexColors===!0&&(n.vertexColors=!0),this.opacity<1&&(n.opacity=this.opacity),this.transparent===!0&&(n.transparent=!0),this.blendSrc!==Da&&(n.blendSrc=this.blendSrc),this.blendDst!==Na&&(n.blendDst=this.blendDst),this.blendEquation!==Yi&&(n.blendEquation=this.blendEquation),this.blendSrcAlpha!==null&&(n.blendSrcAlpha=this.blendSrcAlpha),this.blendDstAlpha!==null&&(n.blendDstAlpha=this.blendDstAlpha),this.blendEquationAlpha!==null&&(n.blendEquationAlpha=this.blendEquationAlpha),this.blendColor&&this.blendColor.isColor&&(n.blendColor=this.blendColor.getHex()),this.blendAlpha!==0&&(n.blendAlpha=this.blendAlpha),this.depthFunc!==Ms&&(n.depthFunc=this.depthFunc),this.depthTest===!1&&(n.depthTest=this.depthTest),this.depthWrite===!1&&(n.depthWrite=this.depthWrite),this.colorWrite===!1&&(n.colorWrite=this.colorWrite),this.stencilWriteMask!==255&&(n.stencilWriteMask=this.stencilWriteMask),this.stencilFunc!==rh&&(n.stencilFunc=this.stencilFunc),this.stencilRef!==0&&(n.stencilRef=this.stencilRef),this.stencilFuncMask!==255&&(n.stencilFuncMask=this.stencilFuncMask),this.stencilFail!==gs&&(n.stencilFail=this.stencilFail),this.stencilZFail!==gs&&(n.stencilZFail=this.stencilZFail),this.stencilZPass!==gs&&(n.stencilZPass=this.stencilZPass),this.stencilWrite===!0&&(n.stencilWrite=this.stencilWrite),this.rotation!==void 0&&this.rotation!==0&&(n.rotation=this.rotation),this.polygonOffset===!0&&(n.polygonOffset=!0),this.polygonOffsetFactor!==0&&(n.polygonOffsetFactor=this.polygonOffsetFactor),this.polygonOffsetUnits!==0&&(n.polygonOffsetUnits=this.polygonOffsetUnits),this.linewidth!==void 0&&this.linewidth!==1&&(n.linewidth=this.linewidth),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.dithering===!0&&(n.dithering=!0),this.alphaTest>0&&(n.alphaTest=this.alphaTest),this.alphaHash===!0&&(n.alphaHash=!0),this.alphaToCoverage===!0&&(n.alphaToCoverage=!0),this.premultipliedAlpha===!0&&(n.premultipliedAlpha=!0),this.forceSinglePass===!0&&(n.forceSinglePass=!0),this.allowOverride===!1&&(n.allowOverride=!1),this.wireframe===!0&&(n.wireframe=!0),this.wireframeLinewidth>1&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!=="round"&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!=="round"&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading===!0&&(n.flatShading=!0),this.visible===!1&&(n.visible=!1),this.toneMapped===!1&&(n.toneMapped=!1),this.fog===!1&&(n.fog=!1),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let o=[];for(let a in r){let l=r[a];delete l.metadata,o.push(l)}return o}if(t){let r=i(e.textures),o=i(e.images);r.length>0&&(n.textures=r),o.length>0&&(n.images=o)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new be().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new Re().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new Re().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var wi=new z,Wc=new z,fa=new z,Wi=new z,Xc=new z,pa=new z,qc=new z,ai=class{constructor(e=new z,t=new z(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,wi)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=wi.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(wi.copy(this.origin).addScaledVector(this.direction,t),wi.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){Wc.copy(e).add(t).multiplyScalar(.5),fa.copy(t).sub(e).normalize(),Wi.copy(this.origin).sub(Wc);let r=e.distanceTo(t)*.5,o=-this.direction.dot(fa),a=Wi.dot(this.direction),l=-Wi.dot(fa),c=Wi.lengthSq(),h=Math.abs(1-o*o),u,d,f,g;if(h>0)if(u=o*l-a,d=o*a-l,g=r*h,u>=0)if(d>=-g)if(d<=g){let v=1/h;u*=v,d*=v,f=u*(u+o*d+2*a)+d*(o*u+d+2*l)+c}else d=r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d=-r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d<=-g?(u=Math.max(0,-(-o*r+a)),d=u>0?-r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c):d<=g?(u=0,d=Math.min(Math.max(-r,-l),r),f=d*(d+2*l)+c):(u=Math.max(0,-(o*r+a)),d=u>0?r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c);else d=o>0?-r:r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(Wc).addScaledVector(fa,d),f}intersectSphere(e,t){wi.subVectors(e.center,this.origin);let n=wi.dot(this.direction),i=wi.dot(wi)-n*n,r=e.radius*e.radius;if(i>r)return null;let o=Math.sqrt(r-i),a=n-o,l=n+o;return l<0?null:a<0?this.at(l,t):this.at(a,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,o,a,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,d=this.origin;return c>=0?(n=(e.min.x-d.x)*c,i=(e.max.x-d.x)*c):(n=(e.max.x-d.x)*c,i=(e.min.x-d.x)*c),h>=0?(r=(e.min.y-d.y)*h,o=(e.max.y-d.y)*h):(r=(e.max.y-d.y)*h,o=(e.min.y-d.y)*h),n>o||r>i||((r>n||isNaN(n))&&(n=r),(o<i||isNaN(i))&&(i=o),u>=0?(a=(e.min.z-d.z)*u,l=(e.max.z-d.z)*u):(a=(e.max.z-d.z)*u,l=(e.min.z-d.z)*u),n>l||a>i)||((a>n||n!==n)&&(n=a),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,wi)!==null}intersectTriangle(e,t,n,i,r){Xc.subVectors(t,e),pa.subVectors(n,e),qc.crossVectors(Xc,pa);let o=this.direction.dot(qc),a;if(o>0){if(i)return null;a=1}else if(o<0)a=-1,o=-o;else return null;Wi.subVectors(this.origin,e);let l=a*this.direction.dot(pa.crossVectors(Wi,pa));if(l<0)return null;let c=a*this.direction.dot(Xc.cross(Wi));if(c<0||l+c>o)return null;let h=-a*Wi.dot(qc);return h<0?null:this.at(h/o,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},Mt=class extends fn{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new be(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new mn,this.combine=ml,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},ed=new it,ps=new ai,ma=new an,td=new z,ga=new z,xa=new z,_a=new z,Yc=new z,va=new z,nd=new z,ya=new z,Ke=class extends bt{constructor(e=new ht,t=new Mt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,o=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let a=this.morphTargetInfluences;if(r&&a){va.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=a[l],u=r[l];h!==0&&(Yc.fromBufferAttribute(u,e),o?va.addScaledVector(Yc,h):va.addScaledVector(Yc.sub(t),h))}t.add(va)}return t}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),ma.copy(n.boundingSphere),ma.applyMatrix4(r),ps.copy(e.ray).recast(e.near),!(ma.containsPoint(ps.origin)===!1&&(ps.intersectSphere(ma,td)===null||ps.origin.distanceToSquared(td)>(e.far-e.near)**2))&&(ed.copy(r).invert(),ps.copy(e.ray).applyMatrix4(ed),!(n.boundingBox!==null&&ps.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,ps)))}_computeIntersections(e,t,n){let i,r=this.geometry,o=this.material,a=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,d=r.groups,f=r.drawRange;if(a!==null)if(Array.isArray(o))for(let g=0,v=d.length;g<v;g++){let m=d[g],p=o[m.materialIndex],y=Math.max(m.start,f.start),w=Math.min(a.count,Math.min(m.start+m.count,f.start+f.count));for(let x=y,A=w;x<A;x+=3){let T=a.getX(x),I=a.getX(x+1),_=a.getX(x+2);i=Ma(this,p,e,n,c,h,u,T,I,_),i&&(i.faceIndex=Math.floor(x/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),v=Math.min(a.count,f.start+f.count);for(let m=g,p=v;m<p;m+=3){let y=a.getX(m),w=a.getX(m+1),x=a.getX(m+2);i=Ma(this,o,e,n,c,h,u,y,w,x),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(o))for(let g=0,v=d.length;g<v;g++){let m=d[g],p=o[m.materialIndex],y=Math.max(m.start,f.start),w=Math.min(l.count,Math.min(m.start+m.count,f.start+f.count));for(let x=y,A=w;x<A;x+=3){let T=x,I=x+1,_=x+2;i=Ma(this,p,e,n,c,h,u,T,I,_),i&&(i.faceIndex=Math.floor(x/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),v=Math.min(l.count,f.start+f.count);for(let m=g,p=v;m<p;m+=3){let y=m,w=m+1,x=m+2;i=Ma(this,o,e,n,c,h,u,y,w,x),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function ym(s,e,t,n,i,r,o,a){let l;if(e.side===jt?l=n.intersectTriangle(o,r,i,!0,a):l=n.intersectTriangle(i,r,o,e.side===Tn,a),l===null)return null;ya.copy(a),ya.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(ya);return c<t.near||c>t.far?null:{distance:c,point:ya.clone(),object:s}}function Ma(s,e,t,n,i,r,o,a,l,c){s.getVertexPosition(a,ga),s.getVertexPosition(l,xa),s.getVertexPosition(c,_a);let h=ym(s,e,t,n,ga,xa,_a,nd);if(h){let u=new z;qi.getBarycoord(nd,ga,xa,_a,u),i&&(h.uv=qi.getInterpolatedAttribute(i,a,l,c,u,new Re)),r&&(h.uv1=qi.getInterpolatedAttribute(r,a,l,c,u,new Re)),o&&(h.normal=qi.getInterpolatedAttribute(o,a,l,c,u,new z),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let d={a,b:l,c,normal:new z,materialIndex:0};qi.getNormal(ga,xa,_a,d.normal),h.face=d,h.barycoord=u}return h}var Yr=new xt,id=new xt,sd=new xt,Mm=new xt,rd=new it,ba=new z,Zc=new an,od=new it,Kc=new ai,lo=class extends Ke{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=ih,this.bindMatrix=new it,this.bindMatrixInverse=new it,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new Kt),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ba),this.boundingBox.expandByPoint(ba)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new an),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ba),this.boundingSphere.expandByPoint(ba)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Zc.copy(this.boundingSphere),Zc.applyMatrix4(i),e.ray.intersectsSphere(Zc)!==!1&&(od.copy(i).invert(),Kc.copy(e.ray).applyMatrix4(od),!(this.boundingBox!==null&&Kc.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,Kc)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new xt,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===ih?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===Jd?this.bindMatrixInverse.copy(this.bindMatrix).invert():Ze("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;id.fromBufferAttribute(i.attributes.skinIndex,e),sd.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(Yr.copy(t),t.set(0,0,0,0)):(Yr.set(...t,1),t.set(0,0,0)),Yr.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let o=sd.getComponent(r);if(o!==0){let a=id.getComponent(r);rd.multiplyMatrices(n.bones[a].matrixWorld,n.boneInverses[a]),t.addScaledVector(Mm.copy(Yr).applyMatrix4(rd),o)}}return t.isVector4&&(t.w=Yr.w),t.applyMatrix4(this.bindMatrixInverse)}},mr=class extends bt{constructor(){super(),this.isBone=!0,this.type="Bone"}},Zi=class extends Xt{constructor(e=null,t=1,n=1,i,r,o,a,l,c=Wt,h=Wt,u,d){super(null,o,a,l,c,h,i,r,u,d),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},ad=new it,bm=new it,co=class s{constructor(e=[],t=[]){this.uuid=Yn(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){Ze("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new it)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new it;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,o=e.length;r<o;r++){let a=e[r]?e[r].matrixWorld:bm;ad.multiplyMatrices(a,t[r]),ad.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new Zi(t,e,e,Pn,Cn);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],o=t[r];o===void 0&&(Ze("Skeleton: No bone found with UUID:",r),o=new mr),this.bones.push(o),this.boneInverses.push(new it().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let o=t[i];e.bones.push(o.uuid);let a=n[i];e.boneInverses.push(a.toArray())}return e}},Ki=class extends gt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},tr=new it,ld=new it,Sa=[],cd=new Kt,Sm=new it,Zr=new Ke,Kr=new an,gn=class extends Ke{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new Ki(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,Sm)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new Kt),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,tr),cd.copy(e.boundingBox).applyMatrix4(tr),this.boundingBox.union(cd)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new an),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,tr),Kr.copy(e.boundingSphere).applyMatrix4(tr),this.boundingSphere.union(Kr)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,o=e*r+1;for(let a=0;a<n.length;a++)n[a]=i[o+a]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(Zr.geometry=this.geometry,Zr.material=this.material,Zr.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Kr.copy(this.boundingSphere),Kr.applyMatrix4(n),e.ray.intersectsSphere(Kr)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,tr),ld.multiplyMatrices(n,tr),Zr.matrixWorld=ld,Zr.raycast(e,Sa);for(let o=0,a=Sa.length;o<a;o++){let l=Sa[o];l.instanceId=r,l.object=this,t.push(l)}Sa.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new Ki(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new Zi(new Float32Array(i*this.count),i,this.count,bl,Cn));let r=this.morphTexture.source.data.data,o=0;for(let c=0;c<n.length;c++)o+=n[c];let a=this.geometry.morphTargetsRelative?1:1-o,l=i*e;return r[l]=a,r.set(n,l+1),this}updateMorphTargets(){}dispose(){this.dispatchEvent({type:"dispose"}),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},jc=new z,wm=new z,Em=new ot,Nn=class{constructor(e=new z(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=jc.subVectors(n,t).cross(wm.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(jc),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let o=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(o<0||o>1)?null:t.copy(e.start).addScaledVector(i,o)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||Em.getNormalMatrix(e),i=this.coplanarPoint(jc).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}},ms=new an,Tm=new Re(.5,.5),wa=new z,gr=class{constructor(e=new Nn,t=new Nn,n=new Nn,i=new Nn,r=new Nn,o=new Nn){this.planes=[e,t,n,i,r,o]}set(e,t,n,i,r,o){let a=this.planes;return a[0].copy(e),a[1].copy(t),a[2].copy(n),a[3].copy(i),a[4].copy(r),a[5].copy(o),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=qn,n=!1){let i=this.planes,r=e.elements,o=r[0],a=r[1],l=r[2],c=r[3],h=r[4],u=r[5],d=r[6],f=r[7],g=r[8],v=r[9],m=r[10],p=r[11],y=r[12],w=r[13],x=r[14],A=r[15];if(i[0].setComponents(c-o,f-h,p-g,A-y).normalize(),i[1].setComponents(c+o,f+h,p+g,A+y).normalize(),i[2].setComponents(c+a,f+u,p+v,A+w).normalize(),i[3].setComponents(c-a,f-u,p-v,A-w).normalize(),n)i[4].setComponents(l,d,m,x).normalize(),i[5].setComponents(c-l,f-d,p-m,A-x).normalize();else if(i[4].setComponents(c-l,f-d,p-m,A-x).normalize(),t===qn)i[5].setComponents(c+l,f+d,p+m,A+x).normalize();else if(t===ar)i[5].setComponents(l,d,m,x).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),ms.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),ms.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(ms)}intersectsSprite(e){ms.center.set(0,0,0);let t=Tm.distanceTo(e.center);return ms.radius=.7071067811865476+t,ms.applyMatrix4(e.matrixWorld),this.intersectsSphere(ms)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(wa.x=i.normal.x>0?e.max.x:e.min.x,wa.y=i.normal.y>0?e.max.y:e.min.y,wa.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(wa)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var li=class extends fn{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new be(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},qa=new z,Ya=new z,hd=new it,jr=new ai,Ea=new an,Jc=new z,ud=new z,Ai=class extends bt{constructor(e=new ht,t=new li){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)qa.fromBufferAttribute(t,i-1),Ya.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=qa.distanceTo(Ya);e.setAttribute("lineDistance",new et(n,1))}else Ze("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Ea.copy(n.boundingSphere),Ea.applyMatrix4(i),Ea.radius+=r,e.ray.intersectsSphere(Ea)===!1)return;hd.copy(i).invert(),jr.copy(e.ray).applyMatrix4(hd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=this.isLineSegments?2:1,h=n.index,d=n.attributes.position;if(h!==null){let f=Math.max(0,o.start),g=Math.min(h.count,o.start+o.count);for(let v=f,m=g-1;v<m;v+=c){let p=h.getX(v),y=h.getX(v+1),w=Ta(this,e,jr,l,p,y,v);w&&t.push(w)}if(this.isLineLoop){let v=h.getX(g-1),m=h.getX(f),p=Ta(this,e,jr,l,v,m,g-1);p&&t.push(p)}}else{let f=Math.max(0,o.start),g=Math.min(d.count,o.start+o.count);for(let v=f,m=g-1;v<m;v+=c){let p=Ta(this,e,jr,l,v,v+1,v);p&&t.push(p)}if(this.isLineLoop){let v=Ta(this,e,jr,l,g-1,f,g-1);v&&t.push(v)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function Ta(s,e,t,n,i,r,o){let a=s.geometry.attributes.position;if(qa.fromBufferAttribute(a,i),Ya.fromBufferAttribute(a,r),t.distanceSqToSegment(qa,Ya,Jc,ud)>n)return;Jc.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(Jc);if(!(c<e.near||c>e.far))return{distance:c,point:ud.clone().applyMatrix4(s.matrixWorld),index:o,face:null,faceIndex:null,barycoord:null,object:s}}var dd=new z,fd=new z,Ts=class extends Ai{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)dd.fromBufferAttribute(t,i),fd.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+dd.distanceTo(fd);e.setAttribute("lineDistance",new et(n,1))}else Ze("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},ho=class extends Ai{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},xr=class extends fn{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new be(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},pd=new it,oh=new ai,Aa=new an,Ra=new z,ln=class extends bt{constructor(e=new ht,t=new xr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),Aa.copy(n.boundingSphere),Aa.applyMatrix4(i),Aa.radius+=r,e.ray.intersectsSphere(Aa)===!1)return;pd.copy(i).invert(),oh.copy(e.ray).applyMatrix4(pd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=n.index,u=n.attributes.position;if(c!==null){let d=Math.max(0,o.start),f=Math.min(c.count,o.start+o.count);for(let g=d,v=f;g<v;g++){let m=c.getX(g);Ra.fromBufferAttribute(u,m),md(Ra,m,l,i,e,t,this)}}else{let d=Math.max(0,o.start),f=Math.min(u.count,o.start+o.count);for(let g=d,v=f;g<v;g++)Ra.fromBufferAttribute(u,g),md(Ra,g,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function md(s,e,t,n,i,r,o){let a=oh.distanceSqToPoint(s);if(a<t){let l=new z;oh.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(a),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:o})}}var uo=class extends Xt{constructor(e=[],t=ts,n,i,r,o,a,l,c,h){super(e,t,n,i,r,o,a,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},As=class extends Xt{constructor(e,t,n,i,r,o,a,l,c){super(e,t,n,i,r,o,a,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var Ri=class extends Xt{constructor(e,t,n=$n,i,r,o,a=Wt,l=Wt,c,h=oi,u=1){if(h!==oi&&h!==ns)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let d={width:e,height:t,depth:u};super(d,i,r,o,a,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new hr(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return this.compareFunction!==null&&(t.compareFunction=this.compareFunction),t}},Za=class extends Ri{constructor(e,t=$n,n=ts,i,r,o=Wt,a=Wt,l,c=oi){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,o,a,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},fo=class extends Xt{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},ci=class s extends ht{constructor(e=1,t=1,n=1,i=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:o};let a=this;i=Math.floor(i),r=Math.floor(r),o=Math.floor(o);let l=[],c=[],h=[],u=[],d=0,f=0;g("z","y","x",-1,-1,n,t,e,o,r,0),g("z","y","x",1,-1,n,t,-e,o,r,1),g("x","z","y",1,1,e,n,t,i,o,2),g("x","z","y",1,-1,e,n,-t,i,o,3),g("x","y","z",1,-1,e,t,n,i,r,4),g("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new et(c,3)),this.setAttribute("normal",new et(h,3)),this.setAttribute("uv",new et(u,2));function g(v,m,p,y,w,x,A,T,I,_,D){let E=x/I,b=A/_,P=x/2,W=A/2,R=T/2,U=I+1,F=_+1,S=0,X=0,Z=new z;for(let V=0;V<F;V++){let ie=V*b-W;for(let fe=0;fe<U;fe++){let Ce=fe*E-P;Z[v]=Ce*y,Z[m]=ie*w,Z[p]=R,c.push(Z.x,Z.y,Z.z),Z[v]=0,Z[m]=0,Z[p]=T>0?1:-1,h.push(Z.x,Z.y,Z.z),u.push(fe/I),u.push(1-V/_),S+=1}}for(let V=0;V<_;V++)for(let ie=0;ie<I;ie++){let fe=d+ie+U*V,Ce=d+ie+U*(V+1),ze=d+(ie+1)+U*(V+1),He=d+(ie+1)+U*V;l.push(fe,Ce,He),l.push(Ce,ze,He),X+=6}a.addGroup(f,X,D),f+=X,d+=S}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var _r=class s extends ht{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],o=[],a=[],l=[],c=new z,h=new Re;o.push(0,0,0),a.push(0,0,1),l.push(.5,.5);for(let u=0,d=3;u<=t;u++,d+=3){let f=n+u/t*i;c.x=e*Math.cos(f),c.y=e*Math.sin(f),o.push(c.x,c.y,c.z),a.push(0,0,1),h.x=(o[d]/e+1)/2,h.y=(o[d+1]/e+1)/2,l.push(h.x,h.y)}for(let u=1;u<=t;u++)r.push(u,u+1,0);this.setIndex(r),this.setAttribute("position",new et(o,3)),this.setAttribute("normal",new et(a,3)),this.setAttribute("uv",new et(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},ji=class s extends ht{constructor(e=1,t=1,n=1,i=32,r=1,o=!1,a=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],d=[],f=[],g=0,v=[],m=n/2,p=0;y(),o===!1&&(e>0&&w(!0),t>0&&w(!1)),this.setIndex(h),this.setAttribute("position",new et(u,3)),this.setAttribute("normal",new et(d,3)),this.setAttribute("uv",new et(f,2));function y(){let x=new z,A=new z,T=0,I=(t-e)/n;for(let _=0;_<=r;_++){let D=[],E=_/r,b=E*(t-e)+e;for(let P=0;P<=i;P++){let W=P/i,R=W*l+a,U=Math.sin(R),F=Math.cos(R);A.x=b*U,A.y=-E*n+m,A.z=b*F,u.push(A.x,A.y,A.z),x.set(U,I,F).normalize(),d.push(x.x,x.y,x.z),f.push(W,1-E),D.push(g++)}v.push(D)}for(let _=0;_<i;_++)for(let D=0;D<r;D++){let E=v[D][_],b=v[D+1][_],P=v[D+1][_+1],W=v[D][_+1];(e>0||D!==0)&&(h.push(E,b,W),T+=3),(t>0||D!==r-1)&&(h.push(b,P,W),T+=3)}c.addGroup(p,T,0),p+=T}function w(x){let A=g,T=new Re,I=new z,_=0,D=x===!0?e:t,E=x===!0?1:-1;for(let P=1;P<=i;P++)u.push(0,m*E,0),d.push(0,E,0),f.push(.5,.5),g++;let b=g;for(let P=0;P<=i;P++){let R=P/i*l+a,U=Math.cos(R),F=Math.sin(R);I.x=D*F,I.y=m*E,I.z=D*U,u.push(I.x,I.y,I.z),d.push(0,E,0),T.x=U*.5+.5,T.y=F*.5*E+.5,f.push(T.x,T.y),g++}for(let P=0;P<i;P++){let W=A+P,R=b+P;x===!0?h.push(R,R+1,W):h.push(R+1,R,W),_+=3}c.addGroup(p,_,x===!0?1:2),p+=_}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Rs=class s extends ji{constructor(e=1,t=1,n=32,i=1,r=!1,o=0,a=Math.PI*2){super(0,e,t,n,i,r,o,a),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:o,thetaLength:a}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Ka=class s extends ht{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],o=[];a(i),c(n),h(),this.setAttribute("position",new et(r,3)),this.setAttribute("normal",new et(r.slice(),3)),this.setAttribute("uv",new et(o,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function a(y){let w=new z,x=new z,A=new z;for(let T=0;T<t.length;T+=3)f(t[T+0],w),f(t[T+1],x),f(t[T+2],A),l(w,x,A,y)}function l(y,w,x,A){let T=A+1,I=[];for(let _=0;_<=T;_++){I[_]=[];let D=y.clone().lerp(x,_/T),E=w.clone().lerp(x,_/T),b=T-_;for(let P=0;P<=b;P++)P===0&&_===T?I[_][P]=D:I[_][P]=D.clone().lerp(E,P/b)}for(let _=0;_<T;_++)for(let D=0;D<2*(T-_)-1;D++){let E=Math.floor(D/2);D%2===0?(d(I[_][E+1]),d(I[_+1][E]),d(I[_][E])):(d(I[_][E+1]),d(I[_+1][E+1]),d(I[_+1][E]))}}function c(y){let w=new z;for(let x=0;x<r.length;x+=3)w.x=r[x+0],w.y=r[x+1],w.z=r[x+2],w.normalize().multiplyScalar(y),r[x+0]=w.x,r[x+1]=w.y,r[x+2]=w.z}function h(){let y=new z;for(let w=0;w<r.length;w+=3){y.x=r[w+0],y.y=r[w+1],y.z=r[w+2];let x=m(y)/2/Math.PI+.5,A=p(y)/Math.PI+.5;o.push(x,1-A)}g(),u()}function u(){for(let y=0;y<o.length;y+=6){let w=o[y+0],x=o[y+2],A=o[y+4],T=Math.max(w,x,A),I=Math.min(w,x,A);T>.9&&I<.1&&(w<.2&&(o[y+0]+=1),x<.2&&(o[y+2]+=1),A<.2&&(o[y+4]+=1))}}function d(y){r.push(y.x,y.y,y.z)}function f(y,w){let x=y*3;w.x=e[x+0],w.y=e[x+1],w.z=e[x+2]}function g(){let y=new z,w=new z,x=new z,A=new z,T=new Re,I=new Re,_=new Re;for(let D=0,E=0;D<r.length;D+=9,E+=6){y.set(r[D+0],r[D+1],r[D+2]),w.set(r[D+3],r[D+4],r[D+5]),x.set(r[D+6],r[D+7],r[D+8]),T.set(o[E+0],o[E+1]),I.set(o[E+2],o[E+3]),_.set(o[E+4],o[E+5]),A.copy(y).add(w).add(x).divideScalar(3);let b=m(A);v(T,E+0,y,b),v(I,E+2,w,b),v(_,E+4,x,b)}}function v(y,w,x,A){A<0&&y.x===1&&(o[w]=y.x-1),x.x===0&&x.z===0&&(o[w]=A/2/Math.PI+.5)}function m(y){return Math.atan2(y.z,-y.x)}function p(y){return Math.atan2(-y.y,Math.sqrt(y.x*y.x+y.z*y.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var An=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){Ze("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let o=1;o<=e;o++)n=this.getPoint(o/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,o;t?o=t:o=e*n[r-1];let a=0,l=r-1,c;for(;a<=l;)if(i=Math.floor(a+(l-a)/2),c=n[i]-o,c<0)a=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===o)return i/(r-1);let h=n[i],d=n[i+1]-h,f=(o-h)/d;return(i+f)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let o=this.getPoint(i),a=this.getPoint(r),l=t||(o.isVector2?new Re:new z);return l.copy(a).sub(o).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new z,i=[],r=[],o=[],a=new z,l=new it;for(let f=0;f<=e;f++){let g=f/e;i[f]=this.getTangentAt(g,new z)}r[0]=new z,o[0]=new z;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),d=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),d<=c&&n.set(0,0,1),a.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],a),o[0].crossVectors(i[0],r[0]);for(let f=1;f<=e;f++){if(r[f]=r[f-1].clone(),o[f]=o[f-1].clone(),a.crossVectors(i[f-1],i[f]),a.length()>Number.EPSILON){a.normalize();let g=Math.acos(ct(i[f-1].dot(i[f]),-1,1));r[f].applyMatrix4(l.makeRotationAxis(a,g))}o[f].crossVectors(i[f],r[f])}if(t===!0){let f=Math.acos(ct(r[0].dot(r[e]),-1,1));f/=e,i[0].dot(a.crossVectors(r[0],r[e]))>0&&(f=-f);for(let g=1;g<=e;g++)r[g].applyMatrix4(l.makeRotationAxis(i[g],f*g)),o[g].crossVectors(i[g],r[g])}return{tangents:i,normals:r,binormals:o}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},po=class extends An{constructor(e=0,t=0,n=1,i=1,r=0,o=Math.PI*2,a=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=o,this.aClockwise=a,this.aRotation=l}getPoint(e,t=new Re){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,o=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(o?r=0:r=i),this.aClockwise===!0&&!o&&(r===i?r=-i:r=r-i);let a=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(a),c=this.aY+this.yRadius*Math.sin(a);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),d=l-this.aX,f=c-this.aY;l=d*h-f*u+this.aX,c=d*u+f*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},ja=class extends po{constructor(e,t,n,i,r,o){super(e,t,n,n,i,r,o),this.isArcCurve=!0,this.type="ArcCurve"}};function Rh(){let s=0,e=0,t=0,n=0;function i(r,o,a,l){s=r,e=a,t=-3*r+3*o-2*a-l,n=2*r-2*o+a+l}return{initCatmullRom:function(r,o,a,l,c){i(o,a,c*(a-r),c*(l-o))},initNonuniformCatmullRom:function(r,o,a,l,c,h,u){let d=(o-r)/c-(a-r)/(c+h)+(a-o)/h,f=(a-o)/h-(l-o)/(h+u)+(l-a)/u;d*=h,f*=h,i(o,a,d,f)},calc:function(r){let o=r*r,a=o*r;return s+e*r+t*o+n*a}}}var gd=new z,xd=new z,$c=new Rh,Qc=new Rh,eh=new Rh,vr=class extends An{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new z){let n=t,i=this.points,r=i.length,o=(r-(this.closed?0:1))*e,a=Math.floor(o),l=o-a;this.closed?a+=a>0?0:(Math.floor(Math.abs(a)/r)+1)*r:l===0&&a===r-1&&(a=r-2,l=1);let c,h;this.closed||a>0?c=i[(a-1)%r]:(xd.subVectors(i[0],i[1]).add(i[0]),c=xd);let u=i[a%r],d=i[(a+1)%r];if(this.closed||a+2<r?h=i[(a+2)%r]:(gd.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=gd),this.curveType==="centripetal"||this.curveType==="chordal"){let f=this.curveType==="chordal"?.5:.25,g=Math.pow(c.distanceToSquared(u),f),v=Math.pow(u.distanceToSquared(d),f),m=Math.pow(d.distanceToSquared(h),f);v<1e-4&&(v=1),g<1e-4&&(g=v),m<1e-4&&(m=v),$c.initNonuniformCatmullRom(c.x,u.x,d.x,h.x,g,v,m),Qc.initNonuniformCatmullRom(c.y,u.y,d.y,h.y,g,v,m),eh.initNonuniformCatmullRom(c.z,u.z,d.z,h.z,g,v,m)}else this.curveType==="catmullrom"&&($c.initCatmullRom(c.x,u.x,d.x,h.x,this.tension),Qc.initCatmullRom(c.y,u.y,d.y,h.y,this.tension),eh.initCatmullRom(c.z,u.z,d.z,h.z,this.tension));return n.set($c.calc(l),Qc.calc(l),eh.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new z().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function _d(s,e,t,n,i){let r=(n-e)*.5,o=(i-t)*.5,a=s*s,l=s*a;return(2*t-2*n+r+o)*l+(-3*t+3*n-2*r-o)*a+r*s+t}function Am(s,e){let t=1-s;return t*t*e}function Rm(s,e){return 2*(1-s)*s*e}function Cm(s,e){return s*s*e}function Qr(s,e,t,n){return Am(s,e)+Rm(s,t)+Cm(s,n)}function Pm(s,e){let t=1-s;return t*t*t*e}function Im(s,e){let t=1-s;return 3*t*t*s*e}function Lm(s,e){return 3*(1-s)*s*s*e}function Dm(s,e){return s*s*s*e}function eo(s,e,t,n,i){return Pm(s,e)+Im(s,t)+Lm(s,n)+Dm(s,i)}var Ja=class extends An{constructor(e=new Re,t=new Re,n=new Re,i=new Re){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new Re){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(eo(e,i.x,r.x,o.x,a.x),eo(e,i.y,r.y,o.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},$a=class extends An{constructor(e=new z,t=new z,n=new z,i=new z){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new z){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(eo(e,i.x,r.x,o.x,a.x),eo(e,i.y,r.y,o.y,a.y),eo(e,i.z,r.z,o.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Qa=class extends An{constructor(e=new Re,t=new Re){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new Re){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new Re){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},yr=class extends An{constructor(e=new z,t=new z){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new z){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new z){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},el=class extends An{constructor(e=new Re,t=new Re,n=new Re){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new Re){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(Qr(e,i.x,r.x,o.x),Qr(e,i.y,r.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Zn=class extends An{constructor(e=new z,t=new z,n=new z){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new z){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(Qr(e,i.x,r.x,o.x),Qr(e,i.y,r.y,o.y),Qr(e,i.z,r.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},tl=class extends An{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new Re){let n=t,i=this.points,r=(i.length-1)*e,o=Math.floor(r),a=r-o,l=i[o===0?o:o-1],c=i[o],h=i[o>i.length-2?i.length-1:o+1],u=i[o>i.length-3?i.length-1:o+2];return n.set(_d(a,l.x,c.x,h.x,u.x),_d(a,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new Re().fromArray(i))}return this}},ah=Object.freeze({__proto__:null,ArcCurve:ja,CatmullRomCurve3:vr,CubicBezierCurve:Ja,CubicBezierCurve3:$a,EllipseCurve:po,LineCurve:Qa,LineCurve3:yr,QuadraticBezierCurve:el,QuadraticBezierCurve3:Zn,SplineCurve:tl}),mo=class extends An{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new ah[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let o=i[r]-n,a=this.curves[r],l=a.getLength(),c=l===0?0:1-o/l;return a.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let o=r[i],a=o.isEllipseCurve?e*2:o.isLineCurve||o.isLineCurve3?1:o.isSplineCurve?e*o.points.length:e,l=o.getPoints(a);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new ah[i.type]().fromJSON(i))}return this}};var Mr=class s extends Ka{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var nn=class s extends ht{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,o=t/2,a=Math.floor(n),l=Math.floor(i),c=a+1,h=l+1,u=e/a,d=t/l,f=[],g=[],v=[],m=[];for(let p=0;p<h;p++){let y=p*d-o;for(let w=0;w<c;w++){let x=w*u-r;g.push(x,-y,0),v.push(0,0,1),m.push(w/a),m.push(1-p/l)}}for(let p=0;p<l;p++)for(let y=0;y<a;y++){let w=y+c*p,x=y+c*(p+1),A=y+1+c*(p+1),T=y+1+c*p;f.push(w,x,T),f.push(x,A,T)}this.setIndex(f),this.setAttribute("position",new et(g,3)),this.setAttribute("normal",new et(v,3)),this.setAttribute("uv",new et(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},Cs=class s extends ht{constructor(e=.5,t=1,n=32,i=1,r=0,o=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:o},n=Math.max(3,n),i=Math.max(1,i);let a=[],l=[],c=[],h=[],u=e,d=(t-e)/i,f=new z,g=new Re;for(let v=0;v<=i;v++){for(let m=0;m<=n;m++){let p=r+m/n*o;f.x=u*Math.cos(p),f.y=u*Math.sin(p),l.push(f.x,f.y,f.z),c.push(0,0,1),g.x=(f.x/t+1)/2,g.y=(f.y/t+1)/2,h.push(g.x,g.y)}u+=d}for(let v=0;v<i;v++){let m=v*(n+1);for(let p=0;p<n;p++){let y=p+m,w=y,x=y+n+1,A=y+n+2,T=y+1;a.push(w,x,T),a.push(x,A,T)}}this.setIndex(a),this.setAttribute("position",new et(l,3)),this.setAttribute("normal",new et(c,3)),this.setAttribute("uv",new et(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var hi=class s extends ht{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,o=0,a=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:o,thetaLength:a},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(o+a,Math.PI),c=0,h=[],u=new z,d=new z,f=[],g=[],v=[],m=[];for(let p=0;p<=n;p++){let y=[],w=p/n,x=o+w*a,A=e*Math.cos(x),T=Math.sqrt(e*e-A*A),I=0;p===0&&o===0?I=.5/t:p===n&&l===Math.PI&&(I=-.5/t);for(let _=0;_<=t;_++){let D=_/t,E=i+D*r;u.x=-T*Math.cos(E),u.y=A,u.z=T*Math.sin(E),g.push(u.x,u.y,u.z),d.copy(u).normalize(),v.push(d.x,d.y,d.z),m.push(D+I,1-w),y.push(c++)}h.push(y)}for(let p=0;p<n;p++)for(let y=0;y<t;y++){let w=h[p][y+1],x=h[p][y],A=h[p+1][y],T=h[p+1][y+1];(p!==0||o>0)&&f.push(w,x,T),(p!==n-1||l<Math.PI)&&f.push(x,A,T)}this.setIndex(f),this.setAttribute("position",new et(g,3)),this.setAttribute("normal",new et(v,3)),this.setAttribute("uv",new et(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Rn=class s extends ht{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,o=0,a=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:o,thetaLength:a},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],d=new z,f=new z,g=new z;for(let v=0;v<=n;v++){let m=o+v/n*a;for(let p=0;p<=i;p++){let y=p/i*r;f.x=(e+t*Math.cos(m))*Math.cos(y),f.y=(e+t*Math.cos(m))*Math.sin(y),f.z=t*Math.sin(m),c.push(f.x,f.y,f.z),d.x=e*Math.cos(y),d.y=e*Math.sin(y),g.subVectors(f,d).normalize(),h.push(g.x,g.y,g.z),u.push(p/i),u.push(v/n)}}for(let v=1;v<=n;v++)for(let m=1;m<=i;m++){let p=(i+1)*v+m-1,y=(i+1)*(v-1)+m-1,w=(i+1)*(v-1)+m,x=(i+1)*v+m;l.push(p,y,x),l.push(y,w,x)}this.setIndex(l),this.setAttribute("position",new et(c,3)),this.setAttribute("normal",new et(h,3)),this.setAttribute("uv",new et(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc)}};var go=class s extends ht{constructor(e=new Zn(new z(-1,-1,0),new z(-1,1,0),new z(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let o=e.computeFrenetFrames(t,r);this.tangents=o.tangents,this.normals=o.normals,this.binormals=o.binormals;let a=new z,l=new z,c=new Re,h=new z,u=[],d=[],f=[],g=[];v(),this.setIndex(g),this.setAttribute("position",new et(u,3)),this.setAttribute("normal",new et(d,3)),this.setAttribute("uv",new et(f,2));function v(){for(let w=0;w<t;w++)m(w);m(r===!1?t:0),y(),p()}function m(w){h=e.getPointAt(w/t,h);let x=o.normals[w],A=o.binormals[w];for(let T=0;T<=i;T++){let I=T/i*Math.PI*2,_=Math.sin(I),D=-Math.cos(I);l.x=D*x.x+_*A.x,l.y=D*x.y+_*A.y,l.z=D*x.z+_*A.z,l.normalize(),d.push(l.x,l.y,l.z),a.x=h.x+n*l.x,a.y=h.y+n*l.y,a.z=h.z+n*l.z,u.push(a.x,a.y,a.z)}}function p(){for(let w=1;w<=t;w++)for(let x=1;x<=i;x++){let A=(i+1)*(w-1)+(x-1),T=(i+1)*w+(x-1),I=(i+1)*w+x,_=(i+1)*(w-1)+x;g.push(A,T,_),g.push(T,I,_)}}function y(){for(let w=0;w<=t;w++)for(let x=0;x<=i;x++)c.x=w/t,c.y=x/i,f.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new ah[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},xo=class extends ht{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new z,r=new z;if(e.index!==null){let o=e.attributes.position,a=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:a.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],d=u.start,f=u.count;for(let g=d,v=d+f;g<v;g+=3)for(let m=0;m<3;m++){let p=a.getX(g+m),y=a.getX(g+(m+1)%3);i.fromBufferAttribute(o,p),r.fromBufferAttribute(o,y),vd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let o=e.attributes.position;for(let a=0,l=o.count/3;a<l;a++)for(let c=0;c<3;c++){let h=3*a+c,u=3*a+(c+1)%3;i.fromBufferAttribute(o,h),r.fromBufferAttribute(o,u),vd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new et(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function vd(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function Us(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(yd(i))i.isRenderTargetTexture?(Ze("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(yd(i[0])){let r=[];for(let o=0,a=i.length;o<a;o++)r[o]=i[o].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function cn(s){let e={};for(let t=0;t<s.length;t++){let n=Us(s[t]);for(let i in n)e[i]=n[i]}return e}function yd(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function Nm(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Ch(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:lt.workingColorSpace}var Qn={clone:Us,merge:cn},Um=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Fm=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,pt=class extends fn{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Um,this.fragmentShader=Fm,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=Us(e.uniforms),this.uniformsGroups=Nm(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let o=this.uniforms[i].value;o&&o.isTexture?t.uniforms[i]={type:"t",value:o.toJSON(e).uuid}:o&&o.isColor?t.uniforms[i]={type:"c",value:o.getHex()}:o&&o.isVector2?t.uniforms[i]={type:"v2",value:o.toArray()}:o&&o.isVector3?t.uniforms[i]={type:"v3",value:o.toArray()}:o&&o.isVector4?t.uniforms[i]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?t.uniforms[i]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?t.uniforms[i]={type:"m4",value:o.toArray()}:t.uniforms[i]={value:o}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new be().setHex(i.value);break;case"v2":this.uniforms[n].value=new Re().fromArray(i.value);break;case"v3":this.uniforms[n].value=new z().fromArray(i.value);break;case"v4":this.uniforms[n].value=new xt().fromArray(i.value);break;case"m3":this.uniforms[n].value=new ot().fromArray(i.value);break;case"m4":this.uniforms[n].value=new it().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},br=class extends pt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},xn=class extends fn{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new be(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new be(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=qo,this.normalScale=new Re(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new mn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},_n=class extends xn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new Re(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return ct(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new be(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new be(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new be(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var _o=class extends fn{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new be(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new be(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=qo,this.normalScale=new Re(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new mn,this.combine=ml,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},nl=class extends fn{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=tf,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},il=class extends fn{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function Ca(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function Om(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function Md(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,o=0;o!==n;++r){let a=t[r]*e;for(let l=0;l!==e;++l)i[o++]=s[a+l]}return i}function Bm(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let o=r[n];if(o!==void 0)if(Array.isArray(o))do o=r[n],o!==void 0&&(e.push(r.time),t.push(...o)),r=s[i++];while(r!==void 0);else if(o.toArray!==void 0)do o=r[n],o!==void 0&&(e.push(r.time),o.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do o=r[n],o!==void 0&&(e.push(r.time),t.push(o)),r=s[i++];while(r!==void 0)}var ui=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let o;n:{i:if(!(e<i)){for(let a=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===a)break;if(r=i,i=t[++n],e<i)break t}o=t.length;break n}if(!(e>=r)){let a=t[1];e<a&&(n=2,r=a);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}o=n,n=0;break n}break e}for(;n<o;){let a=n+o>>>1;e<t[a]?o=a:n=a+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let o=0;o!==i;++o)t[o]=n[r+o];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},sl=class extends ui{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:xs,endingEnd:xs}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,o=e+1,a=i[r],l=i[o];if(a===void 0)switch(this.getSettings_().endingStart){case _s:r=e,a=2*t-n;break;case to:r=i.length-2,a=t+i[r]-i[r+1];break;default:r=e,a=n}if(l===void 0)switch(this.getSettings_().endingEnd){case _s:o=e,l=2*n-t;break;case to:o=1,l=n+i[1]-i[0];break;default:o=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-a),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=o*h}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this._offsetPrev,u=this._offsetNext,d=this._weightPrev,f=this._weightNext,g=(n-t)/(i-t),v=g*g,m=v*g,p=-d*m+2*d*v-d*g,y=(1+d)*m+(-1.5-2*d)*v+(-.5+d)*g+1,w=(-1-f)*m+(1.5+f)*v+.5*g,x=f*m-f*v;for(let A=0;A!==a;++A)r[A]=p*o[h+A]+y*o[c+A]+w*o[l+A]+x*o[u+A];return r}},vo=class extends ui{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=(n-t)/(i-t),u=1-h;for(let d=0;d!==a;++d)r[d]=o[c+d]*u+o[l+d]*h;return r}},rl=class extends ui{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},ol=class extends ui{interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this.inTangents,u=this.outTangents;if(!h||!u){let g=(n-t)/(i-t),v=1-g;for(let m=0;m!==a;++m)r[m]=o[c+m]*v+o[l+m]*g;return r}let d=a*2,f=e-1;for(let g=0;g!==a;++g){let v=o[c+g],m=o[l+g],p=f*d+g*2,y=u[p],w=u[p+1],x=e*d+g*2,A=h[x],T=h[x+1],I=(n-t)/(i-t),_,D,E,b,P;for(let W=0;W<8;W++){_=I*I,D=_*I,E=1-I,b=E*E,P=b*E;let U=P*t+3*b*I*y+3*E*_*A+D*i-n;if(Math.abs(U)<1e-10)break;let F=3*b*(y-t)+6*E*I*(A-y)+3*_*(i-A);if(Math.abs(F)<1e-10)break;I=I-U/F,I=Math.max(0,Math.min(1,I))}r[g]=P*v+3*b*I*w+3*E*_*T+D*m}return r}},vn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=Ca(t,this.TimeBufferType),this.values=Ca(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:Ca(e.times,Array),values:Ca(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i)}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new rl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new vo(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new sl(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new ol(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case bs:t=this.InterpolantFactoryMethodDiscrete;break;case Ss:t=this.InterpolantFactoryMethodLinear;break;case La:t=this.InterpolantFactoryMethodSmooth;break;case sh:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return Ze("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return bs;case this.InterpolantFactoryMethodLinear:return Ss;case this.InterpolantFactoryMethodSmooth:return La;case this.InterpolantFactoryMethodBezier:return sh}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e}return this}trim(e,t){let n=this.times,i=n.length,r=0,o=i-1;for(;r!==i&&n[r]<e;)++r;for(;o!==-1&&n[o]>t;)--o;if(++o,r!==0||o!==i){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=n.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&($e("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&($e("KeyframeTrack: Track is empty.",this),e=!1);let o=null;for(let a=0;a!==r;a++){let l=n[a];if(typeof l=="number"&&isNaN(l)){$e("KeyframeTrack: Time is not a valid number.",this,a,l),e=!1;break}if(o!==null&&o>l){$e("KeyframeTrack: Out of order keys.",this,a,l,o),e=!1;break}o=l}if(i!==void 0&&Gp(i))for(let a=0,l=i.length;a!==l;++a){let c=i[a];if(isNaN(c)){$e("KeyframeTrack: Value is not a valid number.",this,a,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===La,r=e.length-1,o=1;for(let a=1;a<r;++a){let l=!1,c=e[a],h=e[a+1];if(c!==h&&(a!==1||c!==e[0]))if(i)l=!0;else{let u=a*n,d=u-n,f=u+n;for(let g=0;g!==n;++g){let v=t[u+g];if(v!==t[d+g]||v!==t[f+g]){l=!0;break}}}if(l){if(a!==o){e[o]=e[a];let u=a*n,d=o*n;for(let f=0;f!==n;++f)t[d+f]=t[u+f]}++o}}if(r>0){e[o]=e[r];for(let a=r*n,l=o*n,c=0;c!==n;++c)t[l+c]=t[a+c];++o}return o!==e.length?(this.times=e.slice(0,o),this.values=t.slice(0,o*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,i}};vn.prototype.ValueTypeName="";vn.prototype.TimeBufferType=Float32Array;vn.prototype.ValueBufferType=Float32Array;vn.prototype.DefaultInterpolation=Ss;var Ci=class extends vn{constructor(e,t,n){super(e,t,n)}};Ci.prototype.ValueTypeName="bool";Ci.prototype.ValueBufferType=Array;Ci.prototype.DefaultInterpolation=bs;Ci.prototype.InterpolantFactoryMethodLinear=void 0;Ci.prototype.InterpolantFactoryMethodSmooth=void 0;var yo=class extends vn{constructor(e,t,n,i){super(e,t,n,i)}};yo.prototype.ValueTypeName="color";var Pi=class extends vn{constructor(e,t,n,i){super(e,t,n,i)}};Pi.prototype.ValueTypeName="number";var al=class extends ui{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=(n-t)/(i-t),c=e*a;for(let h=c+a;c!==h;c+=4)Gt.slerpFlat(r,0,o,c-a,o,c,l);return r}},Ii=class extends vn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new al(this.times,this.values,this.getValueSize(),e)}};Ii.prototype.ValueTypeName="quaternion";Ii.prototype.InterpolantFactoryMethodSmooth=void 0;var Li=class extends vn{constructor(e,t,n){super(e,t,n)}};Li.prototype.ValueTypeName="string";Li.prototype.ValueBufferType=Array;Li.prototype.DefaultInterpolation=bs;Li.prototype.InterpolantFactoryMethodLinear=void 0;Li.prototype.InterpolantFactoryMethodSmooth=void 0;var Ji=class extends vn{constructor(e,t,n,i){super(e,t,n,i)}};Ji.prototype.ValueTypeName="vector";var Ps=class{constructor(e="",t=-1,n=[],i=ic){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=Yn(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let o=0,a=n.length;o!==a;++o)t.push(km(n[o]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,o=n.length;r!==o;++r)t.push(vn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,o=[];for(let a=0;a<r;a++){let l=[],c=[];l.push((a+r-1)%r,a,(a+1)%r),c.push(0,1,0);let h=Om(l);l=Md(l,1,h),c=Md(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),o.push(new Pi(".morphTargetInfluences["+t[a].name+"]",l,c).scale(1/n))}return new this(e,-1,o)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let a=0,l=e.length;a<l;a++){let c=e[a],h=c.name.match(r);if(h&&h.length>1){let u=h[1],d=i[u];d||(i[u]=d=[]),d.push(c)}}let o=[];for(let a in i)o.push(this.CreateFromMorphTargetSequence(a,i[a],t,n));return o}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function zm(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Pi;case"vector":case"vector2":case"vector3":case"vector4":return Ji;case"color":return yo;case"quaternion":return Ii;case"bool":case"boolean":return Ci;case"string":return Li}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function km(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=zm(s.type);if(s.times===void 0){let t=[],n=[];Bm(s.keys,t,n,"value"),s.times=t,s.values=n}return e.parse!==void 0?e.parse(s):new e(s.name,s.times,s.values,s.interpolation)}var si={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(bd(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!bd(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function bd(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var ll=class{constructor(e,t,n){let i=this,r=!1,o=0,a=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){a++,r===!1&&i.onStart!==void 0&&i.onStart(h,o,a),r=!0},this.itemEnd=function(h){o++,i.onProgress!==void 0&&i.onProgress(h,o,a),o===a&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,d=c.length;u<d;u+=2){let f=c[u],g=c[u+1];if(f.global&&(f.lastIndex=0),f.test(h))return g}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},mf=new ll,di=class{constructor(e){this.manager=e!==void 0?e:mf,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};di.DEFAULT_MATERIAL_NAME="__DEFAULT";var Ei={},lh=class extends Error{constructor(e,t){super(e),this.response=t}},Sr=class extends di{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=si.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Ei[e]!==void 0){Ei[e].push({onLoad:t,onProgress:n,onError:i});return}Ei[e]=[],Ei[e].push({onLoad:t,onProgress:n,onError:i});let o=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),a=this.mimeType,l=this.responseType;fetch(o).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&Ze("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Ei[e],u=c.body.getReader(),d=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),f=d?parseInt(d):0,g=f!==0,v=0,m=new ReadableStream({start(p){y();function y(){u.read().then(({done:w,value:x})=>{if(w)p.close();else{v+=x.byteLength;let A=new ProgressEvent("progress",{lengthComputable:g,loaded:v,total:f});for(let T=0,I=h.length;T<I;T++){let _=h[T];_.onProgress&&_.onProgress(A)}p.enqueue(x),y()}},w=>{p.error(w)})}}});return new Response(m)}else throw new lh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,a));case"json":return c.json();default:if(a==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(a),d=u&&u[1]?u[1].toLowerCase():void 0,f=new TextDecoder(d);return c.arrayBuffer().then(g=>f.decode(g))}}}).then(c=>{si.add(`file:${e}`,c);let h=Ei[e];delete Ei[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onLoad&&f.onLoad(c)}}).catch(c=>{let h=Ei[e];if(h===void 0)throw this.manager.itemError(e),c;delete Ei[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onError&&f.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var nr=new WeakMap,cl=class extends di{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=si.get(`image:${e}`);if(o!==void 0){if(o.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);else{let u=nr.get(o);u===void 0&&(u=[],nr.set(o,u)),u.push({onLoad:t,onError:i})}return o}let a=lr("img");function l(){h(),t&&t(this);let u=nr.get(this)||[];for(let d=0;d<u.length;d++){let f=u[d];f.onLoad&&f.onLoad(this)}nr.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),si.remove(`image:${e}`);let d=nr.get(this)||[];for(let f=0;f<d.length;f++){let g=d[f];g.onError&&g.onError(u)}nr.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){a.removeEventListener("load",l,!1),a.removeEventListener("error",c,!1)}return a.addEventListener("load",l,!1),a.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(a.crossOrigin=this.crossOrigin),si.add(`image:${e}`,a),r.manager.itemStart(e),a.src=e,a}};var Mo=class extends di{constructor(e){super(e)}load(e,t,n,i){let r=new Xt,o=new cl(this.manager);return o.setCrossOrigin(this.crossOrigin),o.setPath(this.path),o.load(e,function(a){r.image=a,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Is=class extends bt{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new be(e),this.intensity=t}dispose(){this.dispatchEvent({type:"dispose"})}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},bo=class extends Is{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(bt.DEFAULT_UP),this.updateMatrix(),this.groundColor=new be(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},th=new it,Sd=new z,wd=new z,So=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new Re(512,512),this.mapType=yn,this.map=null,this.mapPass=null,this.matrix=new it,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new gr,this._frameExtents=new Re(1,1),this._viewportCount=1,this._viewports=[new xt(0,0,1,1)]}getViewportCount(){return this._viewportCount}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera,n=this.matrix;Sd.setFromMatrixPosition(e.matrixWorld),t.position.copy(Sd),wd.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(wd),t.updateMatrixWorld(),th.multiplyMatrices(t.projectionMatrix,t.matrixWorldInverse),this._frustum.setFromProjectionMatrix(th,t.coordinateSystem,t.reversedDepth),t.coordinateSystem===ar||t.reversedDepth?n.set(.5,0,0,.5,0,.5,0,.5,0,0,1,0,0,0,0,1):n.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),n.multiply(th)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return this.intensity!==1&&(e.intensity=this.intensity),this.bias!==0&&(e.bias=this.bias),this.normalBias!==0&&(e.normalBias=this.normalBias),this.radius!==1&&(e.radius=this.radius),(this.mapSize.x!==512||this.mapSize.y!==512)&&(e.mapSize=this.mapSize.toArray()),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Pa=new z,Ia=new Gt,ii=new z,wo=class extends bt{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new it,this.projectionMatrix=new it,this.projectionMatrixInverse=new it,this.coordinateSystem=qn,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Pa,Ia,ii),ii.x===1&&ii.y===1&&ii.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Pa,Ia,ii.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Pa,Ia,ii),ii.x===1&&ii.y===1&&ii.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Pa,Ia,ii.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},Xi=new z,Ed=new Re,Td=new Re,Zt=class extends wo{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=ws*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(Jr*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return ws*2*Math.atan(Math.tan(Jr*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){Xi.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(Xi.x,Xi.y).multiplyScalar(-e/Xi.z),Xi.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(Xi.x,Xi.y).multiplyScalar(-e/Xi.z)}getViewSize(e,t){return this.getViewBounds(e,Ed,Td),t.subVectors(Td,Ed)}setViewOffset(e,t,n,i,r,o){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(Jr*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,o=this.view;if(this.view!==null&&this.view.enabled){let l=o.fullWidth,c=o.fullHeight;r+=o.offsetX*i/l,t-=o.offsetY*n/c,i*=o.width/l,n*=o.height/c}let a=this.filmOffset;a!==0&&(r+=e*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},ch=class extends So{constructor(){super(new Zt(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=ws*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this}},Eo=class extends Is{constructor(e,t,n=0,i=Math.PI/3,r=0,o=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(bt.DEFAULT_UP),this.updateMatrix(),this.target=new bt,this.distance=n,this.angle=i,this.penumbra=r,this.decay=o,this.map=null,this.shadow=new ch}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},hh=class extends So{constructor(){super(new Zt(90,1,.5,500)),this.isPointLightShadow=!0}},Kn=class extends Is{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new hh}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},fi=class extends wo{constructor(e=-1,t=1,n=1,i=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,o=n+e,a=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,o=r+c*this.view.width,a-=h*this.view.offsetY,l=a-h*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},uh=class extends So{constructor(){super(new fi(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},$i=class extends Is{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(bt.DEFAULT_UP),this.updateMatrix(),this.target=new bt,this.shadow=new uh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var Di=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var nh=new WeakMap,To=class extends di{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&Ze("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&Ze("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=si.get(`image-bitmap:${e}`);if(o!==void 0){if(r.manager.itemStart(e),o.then){o.then(c=>{nh.has(o)===!0?(i&&i(nh.get(o)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);return}let a={};a.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",a.headers=this.requestHeader,a.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,a).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign(r.options,{colorSpaceConversion:"none"}))}).then(function(c){si.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e)}).catch(function(c){i&&i(c),nh.set(l,c),si.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});si.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var ir=-90,sr=1,hl=class extends bt{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new Zt(ir,sr,e,t);i.layers=this.layers,this.add(i);let r=new Zt(ir,sr,e,t);r.layers=this.layers,this.add(r);let o=new Zt(ir,sr,e,t);o.layers=this.layers,this.add(o);let a=new Zt(ir,sr,e,t);a.layers=this.layers,this.add(a);let l=new Zt(ir,sr,e,t);l.layers=this.layers,this.add(l);let c=new Zt(ir,sr,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,o,a,l]=t;for(let c of t)this.remove(c);if(e===qn)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===ar)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,l,c,h]=this.children,u=e.getRenderTarget(),d=e.getActiveCubeFace(),f=e.getActiveMipmapLevel(),g=e.xr.enabled;e.xr.enabled=!1;let v=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=v,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,d,f),e.xr.enabled=g,n.texture.needsPMREMUpdate=!0}},ul=class extends Zt{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},Ao=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=Hm.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function Hm(){this._document.hidden===!1&&this.reset()}var dl=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,o;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,o=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,o=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,o=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=o,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,o=this.cumulativeWeight;if(o===0){for(let a=0;a!==i;++a)n[r+a]=n[a];o=t}else{o+=t;let a=t/o;this._mixBufferRegion(n,r,0,a,i)}this.cumulativeWeight=o}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,o=this.cumulativeWeightAdditive,a=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}o>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){a.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,o=i;r!==o;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let o=0;o!==r;++o)e[t+o]=e[n+o]}_slerp(e,t,n,i){Gt.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let o=this._workIndex*r;Gt.multiplyQuaternionsFlat(e,o,e,t,e,n),Gt.slerpFlat(e,t,e,t,e,o,i)}_lerp(e,t,n,i,r){let o=1-i;for(let a=0;a!==r;++a){let l=t+a;e[l]=e[l]*o+e[n+a]*i}}_lerpAdditive(e,t,n,i,r){for(let o=0;o!==r;++o){let a=t+o;e[a]=e[a]+e[n+o]*i}}},Ph="\\[\\]\\.:\\/",Vm=new RegExp("["+Ph+"]","g"),Ih="[^"+Ph+"]",Gm="[^"+Ph.replace("\\.","")+"]",Wm=/((?:WC+[\/:])*)/.source.replace("WC",Ih),Xm=/(WCOD+)?/.source.replace("WCOD",Gm),qm=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Ih),Ym=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Ih),Zm=new RegExp("^"+Wm+Xm+qm+Ym+"$"),Km=["material","materials","bones","map"],dh=class{constructor(e,t,n){let i=n||Tt.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},Tt=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(Vm,"")}static parseTrackName(e){let t=Zm.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);Km.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===t||a.uuid===t)return a;let l=n(a.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){Ze("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){$e("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){$e("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){$e("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){$e("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){$e("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){$e("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){$e("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let o=e[i];if(o===void 0){let c=t.nodeName;$e("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let a=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?a=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){$e("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){$e("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(l=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};Tt.Composite=dh;Tt.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};Tt.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};Tt.prototype.GetterByBindingType=[Tt.prototype._getValue_direct,Tt.prototype._getValue_array,Tt.prototype._getValue_arrayElement,Tt.prototype._getValue_toArray];Tt.prototype.SetterByBindingTypeAndVersioning=[[Tt.prototype._setValue_direct,Tt.prototype._setValue_direct_setNeedsUpdate,Tt.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[Tt.prototype._setValue_array,Tt.prototype._setValue_array_setNeedsUpdate,Tt.prototype._setValue_array_setMatrixWorldNeedsUpdate],[Tt.prototype._setValue_arrayElement,Tt.prototype._setValue_arrayElement_setNeedsUpdate,Tt.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[Tt.prototype._setValue_fromArray,Tt.prototype._setValue_fromArray_setNeedsUpdate,Tt.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var fl=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,o=r.length,a=new Array(o),l={endingStart:xs,endingEnd:xs};for(let c=0;c!==o;++c){let h=r[c].createInterpolant(null);a[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=a,this._propertyBindings=new Array(o),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=$d,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,o=r/i,a=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,o,t),this.warp(a,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,o=this.timeScale,a=this._timeScaleInterpolant;a===null&&(a=i._lendControlInterpolant(),this._timeScaleInterpolant=a);let l=a.parameterPositions,c=a.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/o,c[1]=t/o,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let o=this._updateTime(t),a=this._updateWeight(e);if(a>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case ef:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulateAdditive(a);break;case ic:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulate(i,a)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,o=n===Qd;if(e===0)return r===-1?i:o&&(r&1)===1?t-i:i;if(n===nc){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,o)):this._setEndings(this.repetitions===0,!0,o)),i>=t||i<0){let a=Math.floor(i/t);i-=t*a,r+=Math.abs(a);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,o)}else this._setEndings(!1,!1,o);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:a})}}else this._loopCount=r,this.time=i;if(o&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=_s,i.endingEnd=_s):(e?i.endingStart=this.zeroSlopeAtStart?_s:xs:i.endingStart=to,t?i.endingEnd=this.zeroSlopeAtEnd?_s:xs:i.endingEnd=to)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,o=this._weightInterpolant;o===null&&(o=i._lendControlInterpolant(),this._weightInterpolant=o);let a=o.parameterPositions,l=o.sampleValues;return a[0]=r,l[0]=t,a[1]=r+e,l[1]=n,this}},jm=new Float32Array(1),Ro=class extends Fn{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,o=e._propertyBindings,a=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let d=i[u],f=d.name,g=h[f];if(g!==void 0)++g.referenceCount,o[u]=g;else{if(g=o[u],g!==void 0){g._cacheIndex===null&&(++g.referenceCount,this._addInactiveBinding(g,l,f));continue}let v=t&&t._propertyBindings[u].binding.parsedPath;g=new dl(Tt.create(n,f,v),d.ValueTypeName,d.getValueSize()),++g.referenceCount,this._addInactiveBinding(g,l,f),o[u]=g}a[u].resultBuffer=g.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,o=r[t];if(o===void 0)o={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=o;else{let a=o.knownActions;e._byClipCacheIndex=a.length,a.push(e)}e._cacheIndex=i.length,i.push(e),o.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,o=this._actionsByClip,a=o[r],l=a.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=a.actionByRoot,d=(e._localRoot||this._root).uuid;delete u[d],l.length===0&&delete o[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,o=i[t];o===void 0&&(o={},i[t]=o),o[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,o=this._bindingsByRootAndName,a=o[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete a[r],Object.keys(a).length===0&&delete o[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new vo(new Float32Array(2),new Float32Array(2),1,jm),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,o=typeof e=="string"?Ps.findByName(i,e):e,a=o!==null?o.uuid:e,l=this._actionsByClip[a],c=null;if(n===void 0&&(o!==null?n=o.blendMode:n=ic),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],o===null&&(o=c._clip)}if(o===null)return null;let h=new fl(this,o,t,n);return this._bindAction(h,c),this._addInactiveAction(h,a,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Ps.findByName(n,e):e,o=r?r.uuid:e,a=this._actionsByClip[o];return a!==void 0&&a.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),o=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,o);let a=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)a[c].apply(o);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let o=r.knownActions;for(let a=0,l=o.length;a!==l;++a){let c=o[a];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let o in n){let a=n[o].actionByRoot,l=a[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let o in r){let a=r[o];a.restoreOriginalState(),this._removeInactiveBinding(a)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var Ad=new it,Co=class{constructor(e,t,n=0,i=1/0){this.ray=new ai(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new ur,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):$e("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return Ad.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(Ad),this}intersectObject(e,t=!0,n=[]){return fh(e,this,n,t),n.sort(Rd),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)fh(e[i],this,n,t);return n.sort(Rd),n}};function Rd(s,e){return s.distance-e.distance}function fh(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let o=0,a=r.length;o<a;o++)fh(r[o],e,t,!0)}}var wr=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=ct(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(ct(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var ph=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var Po=class extends Fn{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){if(e===void 0){Ze("Controls: connect() now requires an element.");return}this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function Lh(s,e,t,n){let i=Jm(n);switch(t){case Sh:return s*e;case bl:return s*e/i.components*i.byteLength;case Sl:return s*e/i.components*i.byteLength;case is:return s*e*2/i.components*i.byteLength;case wl:return s*e*2/i.components*i.byteLength;case wh:return s*e*3/i.components*i.byteLength;case Pn:return s*e*4/i.components*i.byteLength;case El:return s*e*4/i.components*i.byteLength;case zo:case ko:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Ho:case Vo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Al:case Cl:return Math.max(s,16)*Math.max(e,8)/4;case Tl:case Rl:return Math.max(s,8)*Math.max(e,8)/2;case Pl:case Il:case Dl:case Nl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Ll:case Go:case Ul:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Fl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Ol:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case Bl:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case zl:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case kl:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case Hl:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case Vl:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case Gl:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case Wl:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case Xl:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case ql:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case Yl:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case Zl:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Kl:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case jl:case Jl:case $l:return Math.ceil(s/4)*Math.ceil(e/4)*16;case Ql:case ec:return Math.ceil(s/4)*Math.ceil(e/4)*8;case Wo:case tc:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function Jm(s){switch(s){case yn:case vh:return{byteLength:1,components:1};case Ar:case yh:case Jt:return{byteLength:2,components:1};case yl:case Ml:return{byteLength:2,components:4};case $n:case vl:case Cn:return{byteLength:4,components:1};case Mh:case bh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"185"}}));typeof window<"u"&&(window.__THREE__?Ze("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="185");function Bf(){let s=null,e=!1,t=null,n=null;function i(r,o){t(r,o),n=s.requestAnimationFrame(i)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function Qm(s){let e=new WeakMap;function t(a,l){let c=a.array,h=a.usage,u=c.byteLength,d=s.createBuffer();s.bindBuffer(l,d),s.bufferData(l,c,h),a.onUploadCallback();let f;if(c instanceof Float32Array)f=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)f=s.HALF_FLOAT;else if(c instanceof Uint16Array)a.isFloat16BufferAttribute?f=s.HALF_FLOAT:f=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)f=s.SHORT;else if(c instanceof Uint32Array)f=s.UNSIGNED_INT;else if(c instanceof Int32Array)f=s.INT;else if(c instanceof Int8Array)f=s.BYTE;else if(c instanceof Uint8Array)f=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)f=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:d,type:f,bytesPerElement:c.BYTES_PER_ELEMENT,version:a.version,size:u}}function n(a,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,a),u.length===0)s.bufferSubData(c,0,h);else{u.sort((f,g)=>f.start-g.start);let d=0;for(let f=1;f<u.length;f++){let g=u[d],v=u[f];v.start<=g.start+g.count+1?g.count=Math.max(g.count,v.start+v.count-g.start):(++d,u[d]=v)}u.length=d+1;for(let f=0,g=u.length;f<g;f++){let v=u[f];s.bufferSubData(c,v.start*h.BYTES_PER_ELEMENT,h,v.start,v.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(a){return a.isInterleavedBufferAttribute&&(a=a.data),e.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let l=e.get(a);l&&(s.deleteBuffer(l.buffer),e.delete(a))}function o(a,l){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let h=e.get(a);(!h||h.version<a.version)&&e.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let c=e.get(a);if(c===void 0)e.set(a,t(a,l));else if(c.version<a.version){if(c.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,a,l),c.version=a.version}}return{get:i,remove:r,update:o}}var eg=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,tg=`#ifdef USE_ALPHAHASH
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
#endif`,ng=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,ig=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,sg=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,rg=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,og=`#ifdef USE_AOMAP
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
#endif`,ag=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,lg=`#ifdef USE_BATCHING
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
#endif`,cg=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,hg=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,ug=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,dg=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,fg=`#ifdef USE_IRIDESCENCE
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
#endif`,pg=`#ifdef USE_BUMPMAP
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
#endif`,mg=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,gg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,xg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,_g=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,vg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,yg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,Mg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,bg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,Sg=`#define PI 3.141592653589793
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
} // validated`,wg=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,Eg=`vec3 transformedNormal = objectNormal;
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
#endif`,Tg=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Ag=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,Rg=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,Cg=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,Pg="gl_FragColor = linearToOutputTexel( gl_FragColor );",Ig=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,Lg=`#ifdef USE_ENVMAP
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
#endif`,Dg=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,Ng=`#ifdef USE_ENVMAP
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
#endif`,Ug=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Fg=`#ifdef USE_ENVMAP
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
#endif`,Og=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,Bg=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,zg=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,kg=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Hg=`#ifdef USE_GRADIENTMAP
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
}`,Vg=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Gg=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Wg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,Xg=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,qg=`#ifdef USE_ENVMAP
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
#endif`,Yg=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,Zg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,Kg=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,jg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,Jg=`PhysicalMaterial material;
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
#endif`,$g=`uniform sampler2D dfgLUT;
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
}`,Qg=`
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
#endif`,e0=`#if defined( RE_IndirectDiffuse )
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
#endif`,t0=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,n0=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,i0=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,s0=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,r0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,o0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,a0=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,l0=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,c0=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,h0=`#if defined( USE_POINTS_UV )
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
#endif`,u0=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,d0=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,f0=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,p0=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,m0=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,g0=`#ifdef USE_MORPHTARGETS
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
#endif`,x0=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,_0=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,v0=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,y0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,M0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,b0=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,S0=`#ifdef USE_NORMALMAP
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
#endif`,w0=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,E0=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,T0=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,A0=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,R0=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,C0=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,P0=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,I0=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,L0=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,D0=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,N0=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,U0=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,F0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,O0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,B0=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,z0=`float getShadowMask() {
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
}`,k0=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,H0=`#ifdef USE_SKINNING
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
#endif`,V0=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,G0=`#ifdef USE_SKINNING
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
#endif`,W0=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,X0=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,q0=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,Y0=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,Z0=`#ifdef USE_TRANSMISSION
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
#endif`,K0=`#ifdef USE_TRANSMISSION
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
#endif`,j0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,J0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,$0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,Q0=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,ex=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,tx=`uniform sampler2D t2D;
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
}`,nx=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,ix=`#ifdef ENVMAP_TYPE_CUBE
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
}`,sx=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,rx=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,ox=`#include <common>
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
}`,ax=`#if DEPTH_PACKING == 3200
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
}`,lx=`#define DISTANCE
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
}`,cx=`#define DISTANCE
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
}`,hx=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,ux=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,dx=`uniform float scale;
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
}`,fx=`uniform vec3 diffuse;
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
}`,px=`#include <common>
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
}`,mx=`uniform vec3 diffuse;
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
}`,gx=`#define LAMBERT
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
}`,xx=`#define LAMBERT
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
}`,_x=`#define MATCAP
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
}`,vx=`#define MATCAP
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
}`,yx=`#define NORMAL
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
}`,Mx=`#define NORMAL
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
}`,bx=`#define PHONG
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
}`,Sx=`#define PHONG
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
}`,wx=`#define STANDARD
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
}`,Ex=`#define STANDARD
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
}`,Tx=`#define TOON
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
}`,Ax=`#define TOON
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
}`,Rx=`uniform float size;
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
}`,Cx=`uniform vec3 diffuse;
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
}`,Px=`#include <common>
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
}`,Ix=`uniform vec3 color;
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
}`,Lx=`uniform float rotation;
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
}`,Dx=`uniform vec3 diffuse;
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
}`,ft={alphahash_fragment:eg,alphahash_pars_fragment:tg,alphamap_fragment:ng,alphamap_pars_fragment:ig,alphatest_fragment:sg,alphatest_pars_fragment:rg,aomap_fragment:og,aomap_pars_fragment:ag,batching_pars_vertex:lg,batching_vertex:cg,begin_vertex:hg,beginnormal_vertex:ug,bsdfs:dg,iridescence_fragment:fg,bumpmap_pars_fragment:pg,clipping_planes_fragment:mg,clipping_planes_pars_fragment:gg,clipping_planes_pars_vertex:xg,clipping_planes_vertex:_g,color_fragment:vg,color_pars_fragment:yg,color_pars_vertex:Mg,color_vertex:bg,common:Sg,cube_uv_reflection_fragment:wg,defaultnormal_vertex:Eg,displacementmap_pars_vertex:Tg,displacementmap_vertex:Ag,emissivemap_fragment:Rg,emissivemap_pars_fragment:Cg,colorspace_fragment:Pg,colorspace_pars_fragment:Ig,envmap_fragment:Lg,envmap_common_pars_fragment:Dg,envmap_pars_fragment:Ng,envmap_pars_vertex:Ug,envmap_physical_pars_fragment:qg,envmap_vertex:Fg,fog_vertex:Og,fog_pars_vertex:Bg,fog_fragment:zg,fog_pars_fragment:kg,gradientmap_pars_fragment:Hg,lightmap_pars_fragment:Vg,lights_lambert_fragment:Gg,lights_lambert_pars_fragment:Wg,lights_pars_begin:Xg,lights_toon_fragment:Yg,lights_toon_pars_fragment:Zg,lights_phong_fragment:Kg,lights_phong_pars_fragment:jg,lights_physical_fragment:Jg,lights_physical_pars_fragment:$g,lights_fragment_begin:Qg,lights_fragment_maps:e0,lights_fragment_end:t0,lightprobes_pars_fragment:n0,logdepthbuf_fragment:i0,logdepthbuf_pars_fragment:s0,logdepthbuf_pars_vertex:r0,logdepthbuf_vertex:o0,map_fragment:a0,map_pars_fragment:l0,map_particle_fragment:c0,map_particle_pars_fragment:h0,metalnessmap_fragment:u0,metalnessmap_pars_fragment:d0,morphinstance_vertex:f0,morphcolor_vertex:p0,morphnormal_vertex:m0,morphtarget_pars_vertex:g0,morphtarget_vertex:x0,normal_fragment_begin:_0,normal_fragment_maps:v0,normal_pars_fragment:y0,normal_pars_vertex:M0,normal_vertex:b0,normalmap_pars_fragment:S0,clearcoat_normal_fragment_begin:w0,clearcoat_normal_fragment_maps:E0,clearcoat_pars_fragment:T0,iridescence_pars_fragment:A0,opaque_fragment:R0,packing:C0,premultiplied_alpha_fragment:P0,project_vertex:I0,dithering_fragment:L0,dithering_pars_fragment:D0,roughnessmap_fragment:N0,roughnessmap_pars_fragment:U0,shadowmap_pars_fragment:F0,shadowmap_pars_vertex:O0,shadowmap_vertex:B0,shadowmask_pars_fragment:z0,skinbase_vertex:k0,skinning_pars_vertex:H0,skinning_vertex:V0,skinnormal_vertex:G0,specularmap_fragment:W0,specularmap_pars_fragment:X0,tonemapping_fragment:q0,tonemapping_pars_fragment:Y0,transmission_fragment:Z0,transmission_pars_fragment:K0,uv_pars_fragment:j0,uv_pars_vertex:J0,uv_vertex:$0,worldpos_vertex:Q0,background_vert:ex,background_frag:tx,backgroundCube_vert:nx,backgroundCube_frag:ix,cube_vert:sx,cube_frag:rx,depth_vert:ox,depth_frag:ax,distance_vert:lx,distance_frag:cx,equirect_vert:hx,equirect_frag:ux,linedashed_vert:dx,linedashed_frag:fx,meshbasic_vert:px,meshbasic_frag:mx,meshlambert_vert:gx,meshlambert_frag:xx,meshmatcap_vert:_x,meshmatcap_frag:vx,meshnormal_vert:yx,meshnormal_frag:Mx,meshphong_vert:bx,meshphong_frag:Sx,meshphysical_vert:wx,meshphysical_frag:Ex,meshtoon_vert:Tx,meshtoon_frag:Ax,points_vert:Rx,points_frag:Cx,shadow_vert:Px,shadow_frag:Ix,sprite_vert:Lx,sprite_frag:Dx},ke={common:{diffuse:{value:new be(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new ot},alphaMap:{value:null},alphaMapTransform:{value:new ot},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new ot}},envmap:{envMap:{value:null},envMapRotation:{value:new ot},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new ot}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new ot}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new ot},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new ot},normalScale:{value:new Re(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new ot},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new ot}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new ot}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new ot}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new be(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new z},probesMax:{value:new z},probesResolution:{value:new z}},points:{diffuse:{value:new be(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new ot},alphaTest:{value:0},uvTransform:{value:new ot}},sprite:{diffuse:{value:new be(16777215)},opacity:{value:1},center:{value:new Re(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new ot},alphaMap:{value:null},alphaMapTransform:{value:new ot},alphaTest:{value:0}}},mi={basic:{uniforms:cn([ke.common,ke.specularmap,ke.envmap,ke.aomap,ke.lightmap,ke.fog]),vertexShader:ft.meshbasic_vert,fragmentShader:ft.meshbasic_frag},lambert:{uniforms:cn([ke.common,ke.specularmap,ke.envmap,ke.aomap,ke.lightmap,ke.emissivemap,ke.bumpmap,ke.normalmap,ke.displacementmap,ke.fog,ke.lights,{emissive:{value:new be(0)},envMapIntensity:{value:1}}]),vertexShader:ft.meshlambert_vert,fragmentShader:ft.meshlambert_frag},phong:{uniforms:cn([ke.common,ke.specularmap,ke.envmap,ke.aomap,ke.lightmap,ke.emissivemap,ke.bumpmap,ke.normalmap,ke.displacementmap,ke.fog,ke.lights,{emissive:{value:new be(0)},specular:{value:new be(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:ft.meshphong_vert,fragmentShader:ft.meshphong_frag},standard:{uniforms:cn([ke.common,ke.envmap,ke.aomap,ke.lightmap,ke.emissivemap,ke.bumpmap,ke.normalmap,ke.displacementmap,ke.roughnessmap,ke.metalnessmap,ke.fog,ke.lights,{emissive:{value:new be(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:ft.meshphysical_vert,fragmentShader:ft.meshphysical_frag},toon:{uniforms:cn([ke.common,ke.aomap,ke.lightmap,ke.emissivemap,ke.bumpmap,ke.normalmap,ke.displacementmap,ke.gradientmap,ke.fog,ke.lights,{emissive:{value:new be(0)}}]),vertexShader:ft.meshtoon_vert,fragmentShader:ft.meshtoon_frag},matcap:{uniforms:cn([ke.common,ke.bumpmap,ke.normalmap,ke.displacementmap,ke.fog,{matcap:{value:null}}]),vertexShader:ft.meshmatcap_vert,fragmentShader:ft.meshmatcap_frag},points:{uniforms:cn([ke.points,ke.fog]),vertexShader:ft.points_vert,fragmentShader:ft.points_frag},dashed:{uniforms:cn([ke.common,ke.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:ft.linedashed_vert,fragmentShader:ft.linedashed_frag},depth:{uniforms:cn([ke.common,ke.displacementmap]),vertexShader:ft.depth_vert,fragmentShader:ft.depth_frag},normal:{uniforms:cn([ke.common,ke.bumpmap,ke.normalmap,ke.displacementmap,{opacity:{value:1}}]),vertexShader:ft.meshnormal_vert,fragmentShader:ft.meshnormal_frag},sprite:{uniforms:cn([ke.sprite,ke.fog]),vertexShader:ft.sprite_vert,fragmentShader:ft.sprite_frag},background:{uniforms:{uvTransform:{value:new ot},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:ft.background_vert,fragmentShader:ft.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new ot}},vertexShader:ft.backgroundCube_vert,fragmentShader:ft.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:ft.cube_vert,fragmentShader:ft.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:ft.equirect_vert,fragmentShader:ft.equirect_frag},distance:{uniforms:cn([ke.common,ke.displacementmap,{referencePosition:{value:new z},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:ft.distance_vert,fragmentShader:ft.distance_frag},shadow:{uniforms:cn([ke.lights,ke.fog,{color:{value:new be(0)},opacity:{value:1}}]),vertexShader:ft.shadow_vert,fragmentShader:ft.shadow_frag}};mi.physical={uniforms:cn([mi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new ot},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new ot},clearcoatNormalScale:{value:new Re(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new ot},dispersion:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new ot},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new ot},sheen:{value:0},sheenColor:{value:new be(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new ot},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new ot},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new ot},transmissionSamplerSize:{value:new Re},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new ot},attenuationDistance:{value:0},attenuationColor:{value:new be(0)},specularColor:{value:new be(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new ot},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new ot},anisotropyVector:{value:new Re},anisotropyMap:{value:null},anisotropyMapTransform:{value:new ot}}]),vertexShader:ft.meshphysical_vert,fragmentShader:ft.meshphysical_frag};var oc={r:0,b:0,g:0},Nx=new it,zf=new ot;zf.set(-1,0,0,0,1,0,0,0,1);function Ux(s,e,t,n,i,r){let o=new be(0),a=i===!0?0:1,l,c,h=null,u=0,d=null;function f(y){let w=y.isScene===!0?y.background:null;if(w&&w.isTexture){let x=y.backgroundBlurriness>0;w=e.get(w,x)}return w}function g(y){let w=!1,x=f(y);x===null?m(o,a):x&&x.isColor&&(m(x,1),w=!0);let A=s.xr.getEnvironmentBlendMode();A==="additive"?t.buffers.color.setClear(0,0,0,1,r):A==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||w)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function v(y,w){let x=f(w);x&&(x.isCubeTexture||x.mapping===Bo)?(c===void 0&&(c=new Ke(new ci(1,1,1),new pt({name:"BackgroundCubeMaterial",uniforms:Us(mi.backgroundCube.uniforms),vertexShader:mi.backgroundCube.vertexShader,fragmentShader:mi.backgroundCube.fragmentShader,side:jt,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(A,T,I){this.matrixWorld.copyPosition(I.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=x,c.material.uniforms.backgroundBlurriness.value=w.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=w.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Nx.makeRotationFromEuler(w.backgroundRotation)).transpose(),x.isCubeTexture&&x.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(zf),c.material.toneMapped=lt.getTransfer(x.colorSpace)!==yt,(h!==x||u!==x.version||d!==s.toneMapping)&&(c.material.needsUpdate=!0,h=x,u=x.version,d=s.toneMapping),c.layers.enableAll(),y.unshift(c,c.geometry,c.material,0,0,null)):x&&x.isTexture&&(l===void 0&&(l=new Ke(new nn(2,2),new pt({name:"BackgroundMaterial",uniforms:Us(mi.background.uniforms),vertexShader:mi.background.vertexShader,fragmentShader:mi.background.fragmentShader,side:Tn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=x,l.material.uniforms.backgroundIntensity.value=w.backgroundIntensity,l.material.toneMapped=lt.getTransfer(x.colorSpace)!==yt,x.matrixAutoUpdate===!0&&x.updateMatrix(),l.material.uniforms.uvTransform.value.copy(x.matrix),(h!==x||u!==x.version||d!==s.toneMapping)&&(l.material.needsUpdate=!0,h=x,u=x.version,d=s.toneMapping),l.layers.enableAll(),y.unshift(l,l.geometry,l.material,0,0,null))}function m(y,w){y.getRGB(oc,Ch(s)),t.buffers.color.setClear(oc.r,oc.g,oc.b,w,r)}function p(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return o},setClearColor:function(y,w=1){o.set(y),a=w,m(o,a)},getClearAlpha:function(){return a},setClearAlpha:function(y){a=y,m(o,a)},render:g,addToRenderList:v,dispose:p}}function Fx(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=d(null),r=i,o=!1;function a(b,P,W,R,U){let F=!1,S=u(b,R,W,P);r!==S&&(r=S,c(r.object)),F=f(b,R,W,U),F&&g(b,R,W,U),U!==null&&e.update(U,s.ELEMENT_ARRAY_BUFFER),(F||o)&&(o=!1,x(b,P,W,R),U!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(U).buffer))}function l(){return s.createVertexArray()}function c(b){return s.bindVertexArray(b)}function h(b){return s.deleteVertexArray(b)}function u(b,P,W,R){let U=R.wireframe===!0,F=n[P.id];F===void 0&&(F={},n[P.id]=F);let S=b.isInstancedMesh===!0?b.id:0,X=F[S];X===void 0&&(X={},F[S]=X);let Z=X[W.id];Z===void 0&&(Z={},X[W.id]=Z);let V=Z[U];return V===void 0&&(V=d(l()),Z[U]=V),V}function d(b){let P=[],W=[],R=[];for(let U=0;U<t;U++)P[U]=0,W[U]=0,R[U]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:P,enabledAttributes:W,attributeDivisors:R,object:b,attributes:{},index:null}}function f(b,P,W,R){let U=r.attributes,F=P.attributes,S=0,X=W.getAttributes();for(let Z in X)if(X[Z].location>=0){let ie=U[Z],fe=F[Z];if(fe===void 0&&(Z==="instanceMatrix"&&b.instanceMatrix&&(fe=b.instanceMatrix),Z==="instanceColor"&&b.instanceColor&&(fe=b.instanceColor)),ie===void 0||ie.attribute!==fe||fe&&ie.data!==fe.data)return!0;S++}return r.attributesNum!==S||r.index!==R}function g(b,P,W,R){let U={},F=P.attributes,S=0,X=W.getAttributes();for(let Z in X)if(X[Z].location>=0){let ie=F[Z];ie===void 0&&(Z==="instanceMatrix"&&b.instanceMatrix&&(ie=b.instanceMatrix),Z==="instanceColor"&&b.instanceColor&&(ie=b.instanceColor));let fe={};fe.attribute=ie,ie&&ie.data&&(fe.data=ie.data),U[Z]=fe,S++}r.attributes=U,r.attributesNum=S,r.index=R}function v(){let b=r.newAttributes;for(let P=0,W=b.length;P<W;P++)b[P]=0}function m(b){p(b,0)}function p(b,P){let W=r.newAttributes,R=r.enabledAttributes,U=r.attributeDivisors;W[b]=1,R[b]===0&&(s.enableVertexAttribArray(b),R[b]=1),U[b]!==P&&(s.vertexAttribDivisor(b,P),U[b]=P)}function y(){let b=r.newAttributes,P=r.enabledAttributes;for(let W=0,R=P.length;W<R;W++)P[W]!==b[W]&&(s.disableVertexAttribArray(W),P[W]=0)}function w(b,P,W,R,U,F,S){S===!0?s.vertexAttribIPointer(b,P,W,U,F):s.vertexAttribPointer(b,P,W,R,U,F)}function x(b,P,W,R){v();let U=R.attributes,F=W.getAttributes(),S=P.defaultAttributeValues;for(let X in F){let Z=F[X];if(Z.location>=0){let V=U[X];if(V===void 0&&(X==="instanceMatrix"&&b.instanceMatrix&&(V=b.instanceMatrix),X==="instanceColor"&&b.instanceColor&&(V=b.instanceColor)),V!==void 0){let ie=V.normalized,fe=V.itemSize,Ce=e.get(V);if(Ce===void 0)continue;let ze=Ce.buffer,He=Ce.type,ce=Ce.bytesPerElement,Me=He===s.INT||He===s.UNSIGNED_INT||V.gpuType===vl;if(V.isInterleavedBufferAttribute){let me=V.data,Oe=me.stride,ue=V.offset;if(me.isInstancedInterleavedBuffer){for(let xe=0;xe<Z.locationSize;xe++)p(Z.location+xe,me.meshPerAttribute);b.isInstancedMesh!==!0&&R._maxInstanceCount===void 0&&(R._maxInstanceCount=me.meshPerAttribute*me.count)}else for(let xe=0;xe<Z.locationSize;xe++)m(Z.location+xe);s.bindBuffer(s.ARRAY_BUFFER,ze);for(let xe=0;xe<Z.locationSize;xe++)w(Z.location+xe,fe/Z.locationSize,He,ie,Oe*ce,(ue+fe/Z.locationSize*xe)*ce,Me)}else{if(V.isInstancedBufferAttribute){for(let me=0;me<Z.locationSize;me++)p(Z.location+me,V.meshPerAttribute);b.isInstancedMesh!==!0&&R._maxInstanceCount===void 0&&(R._maxInstanceCount=V.meshPerAttribute*V.count)}else for(let me=0;me<Z.locationSize;me++)m(Z.location+me);s.bindBuffer(s.ARRAY_BUFFER,ze);for(let me=0;me<Z.locationSize;me++)w(Z.location+me,fe/Z.locationSize,He,ie,fe*ce,fe/Z.locationSize*me*ce,Me)}}else if(S!==void 0){let ie=S[X];if(ie!==void 0)switch(ie.length){case 2:s.vertexAttrib2fv(Z.location,ie);break;case 3:s.vertexAttrib3fv(Z.location,ie);break;case 4:s.vertexAttrib4fv(Z.location,ie);break;default:s.vertexAttrib1fv(Z.location,ie)}}}}y()}function A(){D();for(let b in n){let P=n[b];for(let W in P){let R=P[W];for(let U in R){let F=R[U];for(let S in F)h(F[S].object),delete F[S];delete R[U]}}delete n[b]}}function T(b){if(n[b.id]===void 0)return;let P=n[b.id];for(let W in P){let R=P[W];for(let U in R){let F=R[U];for(let S in F)h(F[S].object),delete F[S];delete R[U]}}delete n[b.id]}function I(b){for(let P in n){let W=n[P];for(let R in W){let U=W[R];if(U[b.id]===void 0)continue;let F=U[b.id];for(let S in F)h(F[S].object),delete F[S];delete U[b.id]}}}function _(b){for(let P in n){let W=n[P],R=b.isInstancedMesh===!0?b.id:0,U=W[R];if(U!==void 0){for(let F in U){let S=U[F];for(let X in S)h(S[X].object),delete S[X];delete U[F]}delete W[R],Object.keys(W).length===0&&delete n[P]}}}function D(){E(),o=!0,r!==i&&(r=i,c(r.object))}function E(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:a,reset:D,resetDefaultState:E,dispose:A,releaseStatesOfGeometry:T,releaseStatesOfObject:_,releaseStatesOfProgram:I,initAttributes:v,enableAttribute:m,disableUnusedAttributes:y}}function Ox(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function o(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function a(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let d=0;for(let f=0;f<h;f++)d+=c[f];t.update(d,n,1)}this.setMode=i,this.render=r,this.renderInstances=o,this.renderMultiDraw=a}function Bx(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let I=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(I.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function o(I){return!(I!==Pn&&n.convert(I)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(I){let _=I===Jt&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(I!==yn&&n.convert(I)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE)&&I!==Cn&&!_)}function l(I){if(I==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";I="mediump"}return I==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(Ze("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,d=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&d===!1&&Ze("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let f=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),g=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),v=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),p=s.getParameter(s.MAX_VERTEX_ATTRIBS),y=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),w=s.getParameter(s.MAX_VARYING_VECTORS),x=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),A=s.getParameter(s.MAX_SAMPLES),T=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:o,textureTypeReadable:a,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:d,maxTextures:f,maxVertexTextures:g,maxTextureSize:v,maxCubemapSize:m,maxAttributes:p,maxVertexUniforms:y,maxVaryings:w,maxFragmentUniforms:x,maxSamples:A,samples:T}}function zx(s){let e=this,t=null,n=0,i=!1,r=!1,o=new Nn,a=new ot,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,d){let f=u.length!==0||d||n!==0||i;return i=d,n=u.length,f},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,d){t=h(u,d,0)},this.setState=function(u,d,f){let g=u.clippingPlanes,v=u.clipIntersection,m=u.clipShadows,p=s.get(u);if(!i||g===null||g.length===0||r&&!m)r?h(null):c();else{let y=r?0:n,w=y*4,x=p.clippingState||null;l.value=x,x=h(g,d,w,f);for(let A=0;A!==w;++A)x[A]=t[A];p.clippingState=x,this.numIntersection=v?this.numPlanes:0,this.numPlanes+=y}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,d,f,g){let v=u!==null?u.length:0,m=null;if(v!==0){if(m=l.value,g!==!0||m===null){let p=f+v*4,y=d.matrixWorldInverse;a.getNormalMatrix(y),(m===null||m.length<p)&&(m=new Float32Array(p));for(let w=0,x=f;w!==v;++w,x+=4)o.copy(u[w]).applyMatrix4(y,a),o.normal.toArray(m,x),m[x+3]=o.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=v,e.numIntersection=0,m}}var ss=4,gf=[.125,.215,.35,.446,.526,.582],Fs=20,kx=256,Yo=new fi,xf=new be,Dh=null,Nh=0,Uh=0,Fh=!1,Hx=new z,Lr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._sigmas=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:o=256,position:a=Hx}=r;Dh=this._renderer.getRenderTarget(),Nh=this._renderer.getActiveCubeFace(),Uh=this._renderer.getActiveMipmapLevel(),Fh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(o);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,a),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=yf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=vf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Dh,Nh,Uh),this._renderer.xr.enabled=Fh,e.scissorTest=!1,Pr(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===ts||e.mapping===Ds?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Dh=this._renderer.getRenderTarget(),Nh=this._renderer.getActiveCubeFace(),Uh=this._renderer.getActiveMipmapLevel(),Fh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:Ft,minFilter:Ft,generateMipmaps:!1,type:Jt,format:Pn,colorSpace:dn,depthBuffer:!1},i=_f(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=_f(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods,sigmas:this._sigmas}=Vx(r)),this._blurMaterial=Wx(r,e,t),this._ggxMaterial=Gx(r,e,t)}return i}_compileMaterial(e){let t=new Ke(new ht,e);this._renderer.compile(t,Yo)}_sceneToCubeUV(e,t,n,i,r){let l=new Zt(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,d=u.autoClear,f=u.toneMapping;u.getClearColor(xf),u.toneMapping=jn,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new Ke(new ci,new Mt({name:"PMREM.Background",side:jt,depthWrite:!1,depthTest:!1})));let v=this._backgroundBox,m=v.material,p=!1,y=e.background;y?y.isColor&&(m.color.copy(y),e.background=null,p=!0):(m.color.copy(xf),p=!0);for(let w=0;w<6;w++){let x=w%3;x===0?(l.up.set(0,c[w],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[w],r.y,r.z)):x===1?(l.up.set(0,0,c[w]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[w],r.z)):(l.up.set(0,c[w],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[w]));let A=this._cubeSize;Pr(i,x*A,w>2?A:0,A,A),u.setRenderTarget(i),p&&u.render(v,l),u.render(e,l)}u.toneMapping=f,u.autoClear=d,e.background=y}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===ts||e.mapping===Ds;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=yf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=vf());let r=i?this._cubemapMaterial:this._equirectMaterial,o=this._lodMeshes[0];o.material=r;let a=r.uniforms;a.envMap.value=e;let l=this._cubeSize;Pr(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(o,Yo)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,o=this._ggxMaterial,a=this._lodMeshes[n];a.material=o;let l=o.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),d=0+c*1.25,f=u*d,{_lodMax:g}=this,v=this._sizeLods[n],m=3*v*(n>g-ss?n-g+ss:0),p=4*(this._cubeSize-v);l.envMap.value=e.texture,l.roughness.value=f,l.mipInt.value=g-t,Pr(r,m,p,3*v,2*v),i.setRenderTarget(r),i.render(a,Yo),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=g-n,Pr(e,m,p,3*v,2*v),i.setRenderTarget(e),i.render(a,Yo)}_blur(e,t,n,i,r){let o=this._pingPongRenderTarget;this._halfBlur(e,o,t,n,i,"latitudinal",r),this._halfBlur(o,e,n,n,i,"longitudinal",r)}_halfBlur(e,t,n,i,r,o,a){let l=this._renderer,c=this._blurMaterial;o!=="latitudinal"&&o!=="longitudinal"&&$e("blur direction must be either latitudinal or longitudinal!");let h=3,u=this._lodMeshes[i];u.material=c;let d=c.uniforms,f=this._sizeLods[n]-1,g=isFinite(r)?Math.PI/(2*f):2*Math.PI/(2*Fs-1),v=r/g,m=isFinite(r)?1+Math.floor(h*v):Fs;m>Fs&&Ze(`sigmaRadians, ${r}, is too large and will clip, as it requested ${m} samples when the maximum is set to ${Fs}`);let p=[],y=0;for(let I=0;I<Fs;++I){let _=I/v,D=Math.exp(-_*_/2);p.push(D),I===0?y+=D:I<m&&(y+=2*D)}for(let I=0;I<p.length;I++)p[I]=p[I]/y;d.envMap.value=e.texture,d.samples.value=m,d.weights.value=p,d.latitudinal.value=o==="latitudinal",a&&(d.poleAxis.value=a);let{_lodMax:w}=this;d.dTheta.value=g,d.mipInt.value=w-n;let x=this._sizeLods[i],A=3*x*(i>w-ss?i-w+ss:0),T=4*(this._cubeSize-x);Pr(t,A,T,3*x,2*x),l.setRenderTarget(t),l.render(u,Yo)}};function Vx(s){let e=[],t=[],n=[],i=s,r=s-ss+1+gf.length;for(let o=0;o<r;o++){let a=Math.pow(2,i);e.push(a);let l=1/a;o>s-ss?l=gf[o-s+ss-1]:o===0&&(l=0),t.push(l);let c=1/(a-2),h=-c,u=1+c,d=[h,h,u,h,u,u,h,h,u,u,h,u],f=6,g=6,v=3,m=2,p=1,y=new Float32Array(v*g*f),w=new Float32Array(m*g*f),x=new Float32Array(p*g*f);for(let T=0;T<f;T++){let I=T%3*2/3-1,_=T>2?0:-1,D=[I,_,0,I+2/3,_,0,I+2/3,_+1,0,I,_,0,I+2/3,_+1,0,I,_+1,0];y.set(D,v*g*T),w.set(d,m*g*T);let E=[T,T,T,T,T,T];x.set(E,p*g*T)}let A=new ht;A.setAttribute("position",new gt(y,v)),A.setAttribute("uv",new gt(w,m)),A.setAttribute("faceIndex",new gt(x,p)),n.push(new Ke(A,null)),i>ss&&i--}return{lodMeshes:n,sizeLods:e,sigmas:t}}function _f(s,e,t){let n=new zt(s,e,t);return n.texture.mapping=Bo,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function Pr(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function Gx(s,e,t){return new pt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:kx,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:hc(),fragmentShader:`

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
		`,blending:On,depthTest:!1,depthWrite:!1})}function Wx(s,e,t){let n=new Float32Array(Fs),i=new z(0,1,0);return new pt({name:"SphericalGaussianBlur",defines:{n:Fs,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},samples:{value:1},weights:{value:n},latitudinal:{value:!1},dTheta:{value:0},mipInt:{value:0},poleAxis:{value:i}},vertexShader:hc(),fragmentShader:`

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
		`,blending:On,depthTest:!1,depthWrite:!1})}function vf(){return new pt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:hc(),fragmentShader:`

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
		`,blending:On,depthTest:!1,depthWrite:!1})}function yf(){return new pt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:hc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:On,depthTest:!1,depthWrite:!1})}function hc(){return`

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
	`}var lc=class extends zt{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new uo(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new ci(5,5,5),r=new pt({name:"CubemapFromEquirect",uniforms:Us(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:jt,blending:On});r.uniforms.tEquirect.value=t;let o=new Ke(i,r),a=t.minFilter;return t.minFilter===Jn&&(t.minFilter=Ft),new hl(1,10,this).update(e,o),t.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let o=0;o<6;o++)e.setRenderTarget(this,o),e.clear(t,n,i);e.setRenderTarget(r)}};function Xx(s){let e=new WeakMap,t=new WeakMap,n=null;function i(d,f=!1){return d==null?null:f?o(d):r(d)}function r(d){if(d&&d.isTexture){let f=d.mapping;if(f===gl||f===xl)if(e.has(d)){let g=e.get(d).texture;return a(g,d.mapping)}else{let g=d.image;if(g&&g.height>0){let v=new lc(g.height);return v.fromEquirectangularTexture(s,d),e.set(d,v),d.addEventListener("dispose",c),a(v.texture,d.mapping)}else return null}}return d}function o(d){if(d&&d.isTexture){let f=d.mapping,g=f===gl||f===xl,v=f===ts||f===Ds;if(g||v){let m=t.get(d),p=m!==void 0?m.texture.pmremVersion:0;if(d.isRenderTargetTexture&&d.pmremVersion!==p)return n===null&&(n=new Lr(s)),m=g?n.fromEquirectangular(d,m):n.fromCubemap(d,m),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),m.texture;if(m!==void 0)return m.texture;{let y=d.image;return g&&y&&y.height>0||v&&y&&l(y)?(n===null&&(n=new Lr(s)),m=g?n.fromEquirectangular(d):n.fromCubemap(d),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),d.addEventListener("dispose",h),m.texture):null}}}return d}function a(d,f){return f===gl?d.mapping=ts:f===xl&&(d.mapping=Ds),d}function l(d){let f=0,g=6;for(let v=0;v<g;v++)d[v]!==void 0&&f++;return f===g}function c(d){let f=d.target;f.removeEventListener("dispose",c);let g=e.get(f);g!==void 0&&(e.delete(f),g.dispose())}function h(d){let f=d.target;f.removeEventListener("dispose",h);let g=t.get(f);g!==void 0&&(t.delete(f),g.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function qx(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&vs("WebGLRenderer: "+n+" extension not supported."),i}}}function Yx(s,e,t,n){let i={},r=new WeakMap;function o(u){let d=u.target;d.index!==null&&e.remove(d.index);for(let g in d.attributes)e.remove(d.attributes[g]);d.removeEventListener("dispose",o),delete i[d.id];let f=r.get(d);f&&(e.remove(f),r.delete(d)),n.releaseStatesOfGeometry(d),d.isInstancedBufferGeometry===!0&&delete d._maxInstanceCount,t.memory.geometries--}function a(u,d){return i[d.id]===!0||(d.addEventListener("dispose",o),i[d.id]=!0,t.memory.geometries++),d}function l(u){let d=u.attributes;for(let f in d)e.update(d[f],s.ARRAY_BUFFER)}function c(u){let d=[],f=u.index,g=u.attributes.position,v=0;if(g===void 0)return;if(f!==null){let y=f.array;v=f.version;for(let w=0,x=y.length;w<x;w+=3){let A=y[w+0],T=y[w+1],I=y[w+2];d.push(A,T,T,I,I,A)}}else{let y=g.array;v=g.version;for(let w=0,x=y.length/3-1;w<x;w+=3){let A=w+0,T=w+1,I=w+2;d.push(A,T,T,I,I,A)}}let m=new(g.count>=65535?ao:oo)(d,1);m.version=v;let p=r.get(u);p&&e.remove(p),r.set(u,m)}function h(u){let d=r.get(u);if(d){let f=u.index;f!==null&&d.version<f.version&&c(u)}else c(u);return r.get(u)}return{get:a,update:l,getWireframeAttribute:h}}function Zx(s,e,t){let n;function i(u){n=u}let r,o;function a(u){r=u.type,o=u.bytesPerElement}function l(u,d){s.drawElements(n,d,r,u*o),t.update(d,n,1)}function c(u,d,f){f!==0&&(s.drawElementsInstanced(n,d,r,u*o,f),t.update(d,n,f))}function h(u,d,f){if(f===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,d,0,r,u,0,f);let v=0;for(let m=0;m<f;m++)v+=d[m];t.update(v,n,1)}this.setMode=i,this.setIndex=a,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function Kx(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,o,a){switch(t.calls++,o){case s.TRIANGLES:t.triangles+=a*(r/3);break;case s.LINES:t.lines+=a*(r/2);break;case s.LINE_STRIP:t.lines+=a*(r-1);break;case s.LINE_LOOP:t.lines+=a*r;break;case s.POINTS:t.points+=a*r;break;default:$e("WebGLInfo: Unknown draw mode:",o);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function jx(s,e,t){let n=new WeakMap,i=new xt;function r(o,a,l){let c=o.morphTargetInfluences,h=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,u=h!==void 0?h.length:0,d=n.get(a);if(d===void 0||d.count!==u){let D=function(){I.dispose(),n.delete(a),a.removeEventListener("dispose",D)};d!==void 0&&d.texture.dispose();let f=a.morphAttributes.position!==void 0,g=a.morphAttributes.normal!==void 0,v=a.morphAttributes.color!==void 0,m=a.morphAttributes.position||[],p=a.morphAttributes.normal||[],y=a.morphAttributes.color||[],w=0;f===!0&&(w=1),g===!0&&(w=2),v===!0&&(w=3);let x=a.attributes.position.count*w,A=1;x>e.maxTextureSize&&(A=Math.ceil(x/e.maxTextureSize),x=e.maxTextureSize);let T=new Float32Array(x*A*4*u),I=new so(T,x,A,u);I.type=Cn,I.needsUpdate=!0;let _=w*4;for(let E=0;E<u;E++){let b=m[E],P=p[E],W=y[E],R=x*A*4*E;for(let U=0;U<b.count;U++){let F=U*_;f===!0&&(i.fromBufferAttribute(b,U),T[R+F+0]=i.x,T[R+F+1]=i.y,T[R+F+2]=i.z,T[R+F+3]=0),g===!0&&(i.fromBufferAttribute(P,U),T[R+F+4]=i.x,T[R+F+5]=i.y,T[R+F+6]=i.z,T[R+F+7]=0),v===!0&&(i.fromBufferAttribute(W,U),T[R+F+8]=i.x,T[R+F+9]=i.y,T[R+F+10]=i.z,T[R+F+11]=W.itemSize===4?i.w:1)}}d={count:u,texture:I,size:new Re(x,A)},n.set(a,d),a.addEventListener("dispose",D)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",o.morphTexture,t);else{let f=0;for(let v=0;v<c.length;v++)f+=c[v];let g=a.morphTargetsRelative?1:1-f;l.getUniforms().setValue(s,"morphTargetBaseInfluence",g),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",d.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",d.size)}return{update:r}}function Jx(s,e,t,n,i){let r=new WeakMap;function o(c){let h=i.render.frame,u=c.geometry,d=e.get(c,u);if(r.get(d)!==h&&(e.update(d),r.set(d,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let f=c.skeleton;r.get(f)!==h&&(f.update(),r.set(f,h))}return d}function a(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:o,dispose:a}}var $x={[Lo]:"LINEAR_TONE_MAPPING",[Do]:"REINHARD_TONE_MAPPING",[No]:"CINEON_TONE_MAPPING",[Ls]:"ACES_FILMIC_TONE_MAPPING",[Fo]:"AGX_TONE_MAPPING",[Oo]:"NEUTRAL_TONE_MAPPING",[Uo]:"CUSTOM_TONE_MAPPING"};function Qx(s,e,t,n,i,r){let o=new zt(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,depthTexture:i?new Ri(e,t):void 0}),a=new zt(e,t,{type:Jt,depthBuffer:!1,stencilBuffer:!1}),l=new ht;l.setAttribute("position",new et([-1,3,0,-1,-1,0,3,-1,0],3)),l.setAttribute("uv",new et([0,2,0,0,2,0],2));let c=new br({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),h=new Ke(l,c),u=new fi(-1,1,1,-1,0,1),d=null,f=null,g=!1,v,m=null,p=[],y=!1;this.setSize=function(w,x){o.setSize(w,x),a.setSize(w,x);for(let A=0;A<p.length;A++){let T=p[A];T.setSize&&T.setSize(w,x)}},this.setEffects=function(w){p=w,y=p.length>0&&p[0].isRenderPass===!0;let x=o.width,A=o.height;for(let T=0;T<p.length;T++){let I=p[T];I.setSize&&I.setSize(x,A)}},this.begin=function(w,x){if(g||w.toneMapping===jn&&p.length===0)return!1;if(m=x,x!==null){let A=x.width,T=x.height;(o.width!==A||o.height!==T)&&this.setSize(A,T)}return y===!1&&w.setRenderTarget(o),v=w.toneMapping,w.toneMapping=jn,!0},this.hasRenderPass=function(){return y},this.end=function(w,x){w.toneMapping=v,g=!0;let A=o,T=a;for(let I=0;I<p.length;I++){let _=p[I];if(_.enabled!==!1&&(_.render(w,T,A,x),_.needsSwap!==!1)){let D=A;A=T,T=D}}if(d!==w.outputColorSpace||f!==w.toneMapping){d=w.outputColorSpace,f=w.toneMapping,c.defines={},lt.getTransfer(d)===yt&&(c.defines.SRGB_TRANSFER="");let I=$x[f];I&&(c.defines[I]=""),c.needsUpdate=!0}c.uniforms.tDiffuse.value=A.texture,w.setRenderTarget(m),w.render(h,u),m=null,g=!1},this.isCompositing=function(){return g},this.dispose=function(){o.depthTexture&&o.depthTexture.dispose(),o.dispose(),a.dispose(),l.dispose(),c.dispose()}}var kf=new Xt,zh=new Ri(1,1),Hf=new so,Vf=new Xa,Gf=new uo,Mf=[],bf=[],Sf=new Float32Array(16),wf=new Float32Array(9),Ef=new Float32Array(4);function Dr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=Mf[i];if(r===void 0&&(r=new Float32Array(i),Mf[i]=r),e!==0){n.toArray(r,0);for(let o=1,a=0;o!==e;++o)a+=t,s[o].toArray(r,a)}return r}function $t(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function Qt(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function uc(s,e){let t=bf[e];t===void 0&&(t=new Int32Array(e),bf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function e_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function t_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if($t(t,e))return;s.uniform2fv(this.addr,e),Qt(t,e)}}function n_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if($t(t,e))return;s.uniform3fv(this.addr,e),Qt(t,e)}}function i_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if($t(t,e))return;s.uniform4fv(this.addr,e),Qt(t,e)}}function s_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if($t(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),Qt(t,e)}else{if($t(t,n))return;Ef.set(n),s.uniformMatrix2fv(this.addr,!1,Ef),Qt(t,n)}}function r_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if($t(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),Qt(t,e)}else{if($t(t,n))return;wf.set(n),s.uniformMatrix3fv(this.addr,!1,wf),Qt(t,n)}}function o_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if($t(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),Qt(t,e)}else{if($t(t,n))return;Sf.set(n),s.uniformMatrix4fv(this.addr,!1,Sf),Qt(t,n)}}function a_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function l_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if($t(t,e))return;s.uniform2iv(this.addr,e),Qt(t,e)}}function c_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if($t(t,e))return;s.uniform3iv(this.addr,e),Qt(t,e)}}function h_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if($t(t,e))return;s.uniform4iv(this.addr,e),Qt(t,e)}}function u_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function d_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if($t(t,e))return;s.uniform2uiv(this.addr,e),Qt(t,e)}}function f_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if($t(t,e))return;s.uniform3uiv(this.addr,e),Qt(t,e)}}function p_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if($t(t,e))return;s.uniform4uiv(this.addr,e),Qt(t,e)}}function m_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(zh.compareFunction=t.isReversedDepthBuffer()?rc:sc,r=zh):r=kf,t.setTexture2D(e||r,i)}function g_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||Vf,i)}function x_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||Gf,i)}function __(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||Hf,i)}function v_(s){switch(s){case 5126:return e_;case 35664:return t_;case 35665:return n_;case 35666:return i_;case 35674:return s_;case 35675:return r_;case 35676:return o_;case 5124:case 35670:return a_;case 35667:case 35671:return l_;case 35668:case 35672:return c_;case 35669:case 35673:return h_;case 5125:return u_;case 36294:return d_;case 36295:return f_;case 36296:return p_;case 35678:case 36198:case 36298:case 36306:case 35682:return m_;case 35679:case 36299:case 36307:return g_;case 35680:case 36300:case 36308:case 36293:return x_;case 36289:case 36303:case 36311:case 36292:return __}}function y_(s,e){s.uniform1fv(this.addr,e)}function M_(s,e){let t=Dr(e,this.size,2);s.uniform2fv(this.addr,t)}function b_(s,e){let t=Dr(e,this.size,3);s.uniform3fv(this.addr,t)}function S_(s,e){let t=Dr(e,this.size,4);s.uniform4fv(this.addr,t)}function w_(s,e){let t=Dr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function E_(s,e){let t=Dr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function T_(s,e){let t=Dr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function A_(s,e){s.uniform1iv(this.addr,e)}function R_(s,e){s.uniform2iv(this.addr,e)}function C_(s,e){s.uniform3iv(this.addr,e)}function P_(s,e){s.uniform4iv(this.addr,e)}function I_(s,e){s.uniform1uiv(this.addr,e)}function L_(s,e){s.uniform2uiv(this.addr,e)}function D_(s,e){s.uniform3uiv(this.addr,e)}function N_(s,e){s.uniform4uiv(this.addr,e)}function U_(s,e,t){let n=this.cache,i=e.length,r=uc(t,i);$t(n,r)||(s.uniform1iv(this.addr,r),Qt(n,r));let o;this.type===s.SAMPLER_2D_SHADOW?o=zh:o=kf;for(let a=0;a!==i;++a)t.setTexture2D(e[a]||o,r[a])}function F_(s,e,t){let n=this.cache,i=e.length,r=uc(t,i);$t(n,r)||(s.uniform1iv(this.addr,r),Qt(n,r));for(let o=0;o!==i;++o)t.setTexture3D(e[o]||Vf,r[o])}function O_(s,e,t){let n=this.cache,i=e.length,r=uc(t,i);$t(n,r)||(s.uniform1iv(this.addr,r),Qt(n,r));for(let o=0;o!==i;++o)t.setTextureCube(e[o]||Gf,r[o])}function B_(s,e,t){let n=this.cache,i=e.length,r=uc(t,i);$t(n,r)||(s.uniform1iv(this.addr,r),Qt(n,r));for(let o=0;o!==i;++o)t.setTexture2DArray(e[o]||Hf,r[o])}function z_(s){switch(s){case 5126:return y_;case 35664:return M_;case 35665:return b_;case 35666:return S_;case 35674:return w_;case 35675:return E_;case 35676:return T_;case 5124:case 35670:return A_;case 35667:case 35671:return R_;case 35668:case 35672:return C_;case 35669:case 35673:return P_;case 5125:return I_;case 36294:return L_;case 36295:return D_;case 36296:return N_;case 35678:case 36198:case 36298:case 36306:case 35682:return U_;case 35679:case 36299:case 36307:return F_;case 35680:case 36300:case 36308:case 36293:return O_;case 36289:case 36303:case 36311:case 36292:return B_}}var kh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=v_(t.type)}},Hh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=z_(t.type)}},Vh=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,o=i.length;r!==o;++r){let a=i[r];a.setValue(e,t[a.id],n)}}},Oh=/(\w+)(\])?(\[|\.)?/g;function Tf(s,e){s.seq.push(e),s.map[e.id]=e}function k_(s,e,t){let n=s.name,i=n.length;for(Oh.lastIndex=0;;){let r=Oh.exec(n),o=Oh.lastIndex,a=r[1],l=r[2]==="]",c=r[3];if(l&&(a=a|0),c===void 0||c==="["&&o+2===i){Tf(t,c===void 0?new kh(a,s,e):new Hh(a,s,e));break}else{let u=t.map[a];u===void 0&&(u=new Vh(a),Tf(t,u)),t=u}}}var Ir=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let o=0;o<n;++o){let a=e.getActiveUniform(t,o),l=e.getUniformLocation(t,a.name);k_(a,l,this)}let i=[],r=[];for(let o of this.seq)o.type===e.SAMPLER_2D_SHADOW||o.type===e.SAMPLER_CUBE_SHADOW||o.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(o):r.push(o);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,o=t.length;r!==o;++r){let a=t[r],l=n[a.id];l.needsUpdate!==!1&&a.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let o=e[i];o.id in t&&n.push(o)}return n}};function Af(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var H_=37297,V_=0;function G_(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let o=i;o<r;o++){let a=o+1;n.push(`${a===e?">":" "} ${a}: ${t[o]}`)}return n.join(`
`)}var Rf=new ot;function W_(s){lt._getMatrix(Rf,lt.workingColorSpace,s);let e=`mat3( ${Rf.elements.map(t=>t.toFixed(4))} )`;switch(lt.getTransfer(s)){case no:return[e,"LinearTransferOETF"];case yt:return[e,"sRGBTransferOETF"];default:return Ze("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function Cf(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let o=/ERROR: 0:(\d+)/.exec(r);if(o){let a=parseInt(o[1]);return t.toUpperCase()+`

`+r+`

`+G_(s.getShaderSource(e),a)}else return r}function X_(s,e){let t=W_(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var q_={[Lo]:"Linear",[Do]:"Reinhard",[No]:"Cineon",[Ls]:"ACESFilmic",[Fo]:"AgX",[Oo]:"Neutral",[Uo]:"Custom"};function Y_(s,e){let t=q_[e];return t===void 0?(Ze("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var ac=new z;function Z_(){lt.getLuminanceCoefficients(ac);let s=ac.x.toFixed(4),e=ac.y.toFixed(4),t=ac.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function K_(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(Ko).join(`
`)}function j_(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function J_(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),o=r.name,a=1;r.type===s.FLOAT_MAT2&&(a=2),r.type===s.FLOAT_MAT3&&(a=3),r.type===s.FLOAT_MAT4&&(a=4),t[o]={type:r.type,location:s.getAttribLocation(e,o),locationSize:a}}return t}function Ko(s){return s!==""}function Pf(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function If(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var $_=/^[ \t]*#include +<([\w\d./]+)>/gm;function Gh(s){return s.replace($_,ev)}var Q_=new Map;function ev(s,e){let t=ft[e];if(t===void 0){let n=Q_.get(e);if(n!==void 0)t=ft[n],Ze('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return Gh(t)}var tv=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function Lf(s){return s.replace(tv,nv)}function nv(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function Df(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var iv={[Io]:"SHADOWMAP_TYPE_PCF",[Er]:"SHADOWMAP_TYPE_VSM"};function sv(s){return iv[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var rv={[ts]:"ENVMAP_TYPE_CUBE",[Ds]:"ENVMAP_TYPE_CUBE",[Bo]:"ENVMAP_TYPE_CUBE_UV"};function ov(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":rv[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var av={[Ds]:"ENVMAP_MODE_REFRACTION"};function lv(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":av[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var cv={[ml]:"ENVMAP_BLENDING_MULTIPLY",[Kd]:"ENVMAP_BLENDING_MIX",[jd]:"ENVMAP_BLENDING_ADD"};function hv(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":cv[s.combine]||"ENVMAP_BLENDING_NONE"}function uv(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function dv(s,e,t,n){let i=s.getContext(),r=t.defines,o=t.vertexShader,a=t.fragmentShader,l=sv(t),c=ov(t),h=lv(t),u=hv(t),d=uv(t),f=K_(t),g=j_(r),v=i.createProgram(),m,p,y=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Ko).join(`
`),m.length>0&&(m+=`
`),p=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Ko).join(`
`),p.length>0&&(p+=`
`)):(m=[Df(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(Ko).join(`
`),p=[Df(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",d?"#define CUBEUV_TEXEL_WIDTH "+d.texelWidth:"",d?"#define CUBEUV_TEXEL_HEIGHT "+d.texelHeight:"",d?"#define CUBEUV_MAX_MIP "+d.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==jn?"#define TONE_MAPPING":"",t.toneMapping!==jn?ft.tonemapping_pars_fragment:"",t.toneMapping!==jn?Y_("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",ft.colorspace_pars_fragment,X_("linearToOutputTexel",t.outputColorSpace),Z_(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(Ko).join(`
`)),o=Gh(o),o=Pf(o,t),o=If(o,t),a=Gh(a),a=Pf(a,t),a=If(a,t),o=Lf(o),a=Lf(a),t.isRawShaderMaterial!==!0&&(y=`#version 300 es
`,m=[f,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,p=["#define varying in",t.glslVersion===Th?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===Th?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+p);let w=y+m+o,x=y+p+a,A=Af(i,i.VERTEX_SHADER,w),T=Af(i,i.FRAGMENT_SHADER,x);i.attachShader(v,A),i.attachShader(v,T),t.index0AttributeName!==void 0?i.bindAttribLocation(v,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(v,0,"position"),i.linkProgram(v);function I(b){if(s.debug.checkShaderErrors){let P=i.getProgramInfoLog(v)||"",W=i.getShaderInfoLog(A)||"",R=i.getShaderInfoLog(T)||"",U=P.trim(),F=W.trim(),S=R.trim(),X=!0,Z=!0;if(i.getProgramParameter(v,i.LINK_STATUS)===!1)if(X=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,v,A,T);else{let V=Cf(i,A,"vertex"),ie=Cf(i,T,"fragment");$e("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(v,i.VALIDATE_STATUS)+`

Material Name: `+b.name+`
Material Type: `+b.type+`

Program Info Log: `+U+`
`+V+`
`+ie)}else U!==""?Ze("WebGLProgram: Program Info Log:",U):(F===""||S==="")&&(Z=!1);Z&&(b.diagnostics={runnable:X,programLog:U,vertexShader:{log:F,prefix:m},fragmentShader:{log:S,prefix:p}})}i.deleteShader(A),i.deleteShader(T),_=new Ir(i,v),D=J_(i,v)}let _;this.getUniforms=function(){return _===void 0&&I(this),_};let D;this.getAttributes=function(){return D===void 0&&I(this),D};let E=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return E===!1&&(E=i.getProgramParameter(v,H_)),E},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(v),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=V_++,this.cacheKey=e,this.usedTimes=1,this.program=v,this.vertexShader=A,this.fragmentShader=T,this}var fv=0,Wh=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new Xh(e),t.set(e,n)),n}},Xh=class{constructor(e){this.id=fv++,this.code=e,this.usedTimes=0}};function pv(s){return s===is||s===Go||s===Wo}function mv(s,e,t,n,i,r){let o=new ur,a=new Wh,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,d=n.precision,f={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function g(_){return l.add(_),_===0?"uv":`uv${_}`}function v(_,D,E,b,P,W){let R=b.fog,U=P.geometry,F=_.isMeshStandardMaterial||_.isMeshLambertMaterial||_.isMeshPhongMaterial?b.environment:null,S=_.isMeshStandardMaterial||_.isMeshLambertMaterial&&!_.envMap||_.isMeshPhongMaterial&&!_.envMap,X=e.get(_.envMap||F,S),Z=X&&X.mapping===Bo?X.image.height:null,V=f[_.type];_.precision!==null&&(d=n.getMaxPrecision(_.precision),d!==_.precision&&Ze("WebGLProgram.getParameters:",_.precision,"not supported, using",d,"instead."));let ie=U.morphAttributes.position||U.morphAttributes.normal||U.morphAttributes.color,fe=ie!==void 0?ie.length:0,Ce=0;U.morphAttributes.position!==void 0&&(Ce=1),U.morphAttributes.normal!==void 0&&(Ce=2),U.morphAttributes.color!==void 0&&(Ce=3);let ze,He,ce,Me;if(V){let Ae=mi[V];ze=Ae.vertexShader,He=Ae.fragmentShader}else{ze=_.vertexShader,He=_.fragmentShader;let Ae=a.getVertexShaderStage(_),at=a.getFragmentShaderStage(_);a.update(_,Ae,at),ce=Ae.id,Me=at.id}let me=s.getRenderTarget(),Oe=s.state.buffers.depth.getReversed(),ue=P.isInstancedMesh===!0,xe=P.isBatchedMesh===!0,H=!!_.map,K=!!_.matcap,Q=!!X,ee=!!_.aoMap,ae=!!_.lightMap,ve=!!_.bumpMap&&_.wireframe===!1,Ue=!!_.normalMap,Se=!!_.displacementMap,je=!!_.emissiveMap,Ge=!!_.metalnessMap,st=!!_.roughnessMap,j=_.anisotropy>0,tt=_.clearcoat>0,Je=_.dispersion>0,B=_.iridescence>0,M=_.sheen>0,te=_.transmission>0,he=j&&!!_.anisotropyMap,de=tt&&!!_.clearcoatMap,Te=tt&&!!_.clearcoatNormalMap,Le=tt&&!!_.clearcoatRoughnessMap,pe=B&&!!_.iridescenceMap,_e=B&&!!_.iridescenceThicknessMap,De=M&&!!_.sheenColorMap,We=M&&!!_.sheenRoughnessMap,Pe=!!_.specularMap,Fe=!!_.specularColorMap,O=!!_.specularIntensityMap,q=te&&!!_.transmissionMap,Y=te&&!!_.thicknessMap,N=!!_.gradientMap,L=!!_.alphaMap,k=_.alphaTest>0,G=!!_.alphaHash,le=!!_.extensions,se=jn;_.toneMapped&&(me===null||me.isXRRenderTarget===!0)&&(se=s.toneMapping);let ge={shaderID:V,shaderType:_.type,shaderName:_.name,vertexShader:ze,fragmentShader:He,defines:_.defines,customVertexShaderID:ce,customFragmentShaderID:Me,isRawShaderMaterial:_.isRawShaderMaterial===!0,glslVersion:_.glslVersion,precision:d,batching:xe,batchingColor:xe&&P._colorsTexture!==null,instancing:ue,instancingColor:ue&&P.instanceColor!==null,instancingMorph:ue&&P.morphTexture!==null,outputColorSpace:me===null?s.outputColorSpace:me.isXRRenderTarget===!0?me.texture.colorSpace:lt.workingColorSpace,alphaToCoverage:!!_.alphaToCoverage,map:H,matcap:K,envMap:Q,envMapMode:Q&&X.mapping,envMapCubeUVHeight:Z,aoMap:ee,lightMap:ae,bumpMap:ve,normalMap:Ue,displacementMap:Se,emissiveMap:je,normalMapObjectSpace:Ue&&_.normalMapType===nf,normalMapTangentSpace:Ue&&_.normalMapType===qo,packedNormalMap:Ue&&_.normalMapType===qo&&pv(_.normalMap.format),metalnessMap:Ge,roughnessMap:st,anisotropy:j,anisotropyMap:he,clearcoat:tt,clearcoatMap:de,clearcoatNormalMap:Te,clearcoatRoughnessMap:Le,dispersion:Je,iridescence:B,iridescenceMap:pe,iridescenceThicknessMap:_e,sheen:M,sheenColorMap:De,sheenRoughnessMap:We,specularMap:Pe,specularColorMap:Fe,specularIntensityMap:O,transmission:te,transmissionMap:q,thicknessMap:Y,gradientMap:N,opaque:_.transparent===!1&&_.blending===ys&&_.alphaToCoverage===!1,alphaMap:L,alphaTest:k,alphaHash:G,combine:_.combine,mapUv:H&&g(_.map.channel),aoMapUv:ee&&g(_.aoMap.channel),lightMapUv:ae&&g(_.lightMap.channel),bumpMapUv:ve&&g(_.bumpMap.channel),normalMapUv:Ue&&g(_.normalMap.channel),displacementMapUv:Se&&g(_.displacementMap.channel),emissiveMapUv:je&&g(_.emissiveMap.channel),metalnessMapUv:Ge&&g(_.metalnessMap.channel),roughnessMapUv:st&&g(_.roughnessMap.channel),anisotropyMapUv:he&&g(_.anisotropyMap.channel),clearcoatMapUv:de&&g(_.clearcoatMap.channel),clearcoatNormalMapUv:Te&&g(_.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:Le&&g(_.clearcoatRoughnessMap.channel),iridescenceMapUv:pe&&g(_.iridescenceMap.channel),iridescenceThicknessMapUv:_e&&g(_.iridescenceThicknessMap.channel),sheenColorMapUv:De&&g(_.sheenColorMap.channel),sheenRoughnessMapUv:We&&g(_.sheenRoughnessMap.channel),specularMapUv:Pe&&g(_.specularMap.channel),specularColorMapUv:Fe&&g(_.specularColorMap.channel),specularIntensityMapUv:O&&g(_.specularIntensityMap.channel),transmissionMapUv:q&&g(_.transmissionMap.channel),thicknessMapUv:Y&&g(_.thicknessMap.channel),alphaMapUv:L&&g(_.alphaMap.channel),vertexTangents:!!U.attributes.tangent&&(Ue||j),vertexNormals:!!U.attributes.normal,vertexColors:_.vertexColors,vertexAlphas:_.vertexColors===!0&&!!U.attributes.color&&U.attributes.color.itemSize===4,pointsUvs:P.isPoints===!0&&!!U.attributes.uv&&(H||L),fog:!!R,useFog:_.fog===!0,fogExp2:!!R&&R.isFogExp2,flatShading:_.wireframe===!1&&(_.flatShading===!0||U.attributes.normal===void 0&&Ue===!1&&(_.isMeshLambertMaterial||_.isMeshPhongMaterial||_.isMeshStandardMaterial||_.isMeshPhysicalMaterial)),sizeAttenuation:_.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:Oe,skinning:P.isSkinnedMesh===!0,hasPositionAttribute:U.attributes.position!==void 0,morphTargets:U.morphAttributes.position!==void 0,morphNormals:U.morphAttributes.normal!==void 0,morphColors:U.morphAttributes.color!==void 0,morphTargetsCount:fe,morphTextureStride:Ce,numDirLights:D.directional.length,numPointLights:D.point.length,numSpotLights:D.spot.length,numSpotLightMaps:D.spotLightMap.length,numRectAreaLights:D.rectArea.length,numHemiLights:D.hemi.length,numDirLightShadows:D.directionalShadowMap.length,numPointLightShadows:D.pointShadowMap.length,numSpotLightShadows:D.spotShadowMap.length,numSpotLightShadowsWithMaps:D.numSpotLightShadowsWithMaps,numLightProbes:D.numLightProbes,numLightProbeGrids:W.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:_.dithering,shadowMapEnabled:s.shadowMap.enabled&&E.length>0,shadowMapType:s.shadowMap.type,toneMapping:se,decodeVideoTexture:H&&_.map.isVideoTexture===!0&&lt.getTransfer(_.map.colorSpace)===yt,decodeVideoTextureEmissive:je&&_.emissiveMap.isVideoTexture===!0&&lt.getTransfer(_.emissiveMap.colorSpace)===yt,premultipliedAlpha:_.premultipliedAlpha,doubleSided:_.side===At,flipSided:_.side===jt,useDepthPacking:_.depthPacking>=0,depthPacking:_.depthPacking||0,index0AttributeName:_.index0AttributeName,extensionClipCullDistance:le&&_.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(le&&_.extensions.multiDraw===!0||xe)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:_.customProgramCacheKey()};return ge.vertexUv1s=l.has(1),ge.vertexUv2s=l.has(2),ge.vertexUv3s=l.has(3),l.clear(),ge}function m(_){let D=[];if(_.shaderID?D.push(_.shaderID):(D.push(_.customVertexShaderID),D.push(_.customFragmentShaderID)),_.defines!==void 0)for(let E in _.defines)D.push(E),D.push(_.defines[E]);return _.isRawShaderMaterial===!1&&(p(D,_),y(D,_),D.push(s.outputColorSpace)),D.push(_.customProgramCacheKey),D.join()}function p(_,D){_.push(D.precision),_.push(D.outputColorSpace),_.push(D.envMapMode),_.push(D.envMapCubeUVHeight),_.push(D.mapUv),_.push(D.alphaMapUv),_.push(D.lightMapUv),_.push(D.aoMapUv),_.push(D.bumpMapUv),_.push(D.normalMapUv),_.push(D.displacementMapUv),_.push(D.emissiveMapUv),_.push(D.metalnessMapUv),_.push(D.roughnessMapUv),_.push(D.anisotropyMapUv),_.push(D.clearcoatMapUv),_.push(D.clearcoatNormalMapUv),_.push(D.clearcoatRoughnessMapUv),_.push(D.iridescenceMapUv),_.push(D.iridescenceThicknessMapUv),_.push(D.sheenColorMapUv),_.push(D.sheenRoughnessMapUv),_.push(D.specularMapUv),_.push(D.specularColorMapUv),_.push(D.specularIntensityMapUv),_.push(D.transmissionMapUv),_.push(D.thicknessMapUv),_.push(D.combine),_.push(D.fogExp2),_.push(D.sizeAttenuation),_.push(D.morphTargetsCount),_.push(D.morphAttributeCount),_.push(D.numDirLights),_.push(D.numPointLights),_.push(D.numSpotLights),_.push(D.numSpotLightMaps),_.push(D.numHemiLights),_.push(D.numRectAreaLights),_.push(D.numDirLightShadows),_.push(D.numPointLightShadows),_.push(D.numSpotLightShadows),_.push(D.numSpotLightShadowsWithMaps),_.push(D.numLightProbes),_.push(D.shadowMapType),_.push(D.toneMapping),_.push(D.numClippingPlanes),_.push(D.numClipIntersection),_.push(D.depthPacking)}function y(_,D){o.disableAll(),D.instancing&&o.enable(0),D.instancingColor&&o.enable(1),D.instancingMorph&&o.enable(2),D.matcap&&o.enable(3),D.envMap&&o.enable(4),D.normalMapObjectSpace&&o.enable(5),D.normalMapTangentSpace&&o.enable(6),D.clearcoat&&o.enable(7),D.iridescence&&o.enable(8),D.alphaTest&&o.enable(9),D.vertexColors&&o.enable(10),D.vertexAlphas&&o.enable(11),D.vertexUv1s&&o.enable(12),D.vertexUv2s&&o.enable(13),D.vertexUv3s&&o.enable(14),D.vertexTangents&&o.enable(15),D.anisotropy&&o.enable(16),D.alphaHash&&o.enable(17),D.batching&&o.enable(18),D.dispersion&&o.enable(19),D.batchingColor&&o.enable(20),D.gradientMap&&o.enable(21),D.packedNormalMap&&o.enable(22),D.vertexNormals&&o.enable(23),_.push(o.mask),o.disableAll(),D.fog&&o.enable(0),D.useFog&&o.enable(1),D.flatShading&&o.enable(2),D.logarithmicDepthBuffer&&o.enable(3),D.reversedDepthBuffer&&o.enable(4),D.skinning&&o.enable(5),D.morphTargets&&o.enable(6),D.morphNormals&&o.enable(7),D.morphColors&&o.enable(8),D.premultipliedAlpha&&o.enable(9),D.shadowMapEnabled&&o.enable(10),D.doubleSided&&o.enable(11),D.flipSided&&o.enable(12),D.useDepthPacking&&o.enable(13),D.dithering&&o.enable(14),D.transmission&&o.enable(15),D.sheen&&o.enable(16),D.opaque&&o.enable(17),D.pointsUvs&&o.enable(18),D.decodeVideoTexture&&o.enable(19),D.decodeVideoTextureEmissive&&o.enable(20),D.alphaToCoverage&&o.enable(21),D.numLightProbeGrids>0&&o.enable(22),D.hasPositionAttribute&&o.enable(23),_.push(o.mask)}function w(_){let D=f[_.type],E;if(D){let b=mi[D];E=Qn.clone(b.uniforms)}else E=_.uniforms;return E}function x(_,D){let E=h.get(D);return E!==void 0?++E.usedTimes:(E=new dv(s,D,_,i),c.push(E),h.set(D,E)),E}function A(_){if(--_.usedTimes===0){let D=c.indexOf(_);c[D]=c[c.length-1],c.pop(),h.delete(_.cacheKey),_.destroy()}}function T(_){a.remove(_)}function I(){a.dispose()}return{getParameters:v,getProgramCacheKey:m,getUniforms:w,acquireProgram:x,releaseProgram:A,releaseShaderCache:T,programs:c,dispose:I}}function gv(){let s=new WeakMap;function e(o){return s.has(o)}function t(o){let a=s.get(o);return a===void 0&&(a={},s.set(o,a)),a}function n(o){s.delete(o)}function i(o,a,l){s.get(o)[a]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function xv(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function Nf(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function Uf(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function o(d){let f=0;return d.isInstancedMesh&&(f+=2),d.isSkinnedMesh&&(f+=1),f}function a(d,f,g,v,m,p){let y=s[e];return y===void 0?(y={id:d.id,object:d,geometry:f,material:g,materialVariant:o(d),groupOrder:v,renderOrder:d.renderOrder,z:m,group:p},s[e]=y):(y.id=d.id,y.object=d,y.geometry=f,y.material=g,y.materialVariant=o(d),y.groupOrder=v,y.renderOrder=d.renderOrder,y.z=m,y.group=p),e++,y}function l(d,f,g,v,m,p){let y=a(d,f,g,v,m,p);g.transmission>0?n.push(y):g.transparent===!0?i.push(y):t.push(y)}function c(d,f,g,v,m,p){let y=a(d,f,g,v,m,p);g.transmission>0?n.unshift(y):g.transparent===!0?i.unshift(y):t.unshift(y)}function h(d,f,g){t.length>1&&t.sort(d||xv),n.length>1&&n.sort(f||Nf),i.length>1&&i.sort(f||Nf),g&&(t.reverse(),n.reverse(),i.reverse())}function u(){for(let d=e,f=s.length;d<f;d++){let g=s[d];if(g.id===null)break;g.id=null,g.object=null,g.geometry=null,g.material=null,g.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function _v(){let s=new WeakMap;function e(n,i){let r=s.get(n),o;return r===void 0?(o=new Uf,s.set(n,[o])):i>=r.length?(o=new Uf,r.push(o)):o=r[i],o}function t(){s=new WeakMap}return{get:e,dispose:t}}function vv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={direction:new z,color:new be};break;case"SpotLight":t={position:new z,direction:new z,color:new be,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new z,color:new be,distance:0,decay:0};break;case"HemisphereLight":t={direction:new z,skyColor:new be,groundColor:new be};break;case"RectAreaLight":t={color:new be,position:new z,halfWidth:new z,halfHeight:new z};break}return s[e.id]=t,t}}}function yv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Re};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Re};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new Re,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var Mv=0;function bv(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function Sv(s){let e=new vv,t=yv(),n={version:0,hash:{directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new z);let i=new z,r=new it,o=new it;function a(c){let h=0,u=0,d=0;for(let D=0;D<9;D++)n.probe[D].set(0,0,0);let f=0,g=0,v=0,m=0,p=0,y=0,w=0,x=0,A=0,T=0,I=0;c.sort(bv);for(let D=0,E=c.length;D<E;D++){let b=c[D],P=b.color,W=b.intensity,R=b.distance,U=null;if(b.shadow&&b.shadow.map&&(b.shadow.map.texture.format===is?U=b.shadow.map.texture:U=b.shadow.map.depthTexture||b.shadow.map.texture),b.isAmbientLight)h+=P.r*W,u+=P.g*W,d+=P.b*W;else if(b.isLightProbe){for(let F=0;F<9;F++)n.probe[F].addScaledVector(b.sh.coefficients[F],W);I++}else if(b.isDirectionalLight){let F=e.get(b);if(F.color.copy(b.color).multiplyScalar(b.intensity),b.castShadow){let S=b.shadow,X=t.get(b);X.shadowIntensity=S.intensity,X.shadowBias=S.bias,X.shadowNormalBias=S.normalBias,X.shadowRadius=S.radius,X.shadowMapSize=S.mapSize,n.directionalShadow[f]=X,n.directionalShadowMap[f]=U,n.directionalShadowMatrix[f]=b.shadow.matrix,y++}n.directional[f]=F,f++}else if(b.isSpotLight){let F=e.get(b);F.position.setFromMatrixPosition(b.matrixWorld),F.color.copy(P).multiplyScalar(W),F.distance=R,F.coneCos=Math.cos(b.angle),F.penumbraCos=Math.cos(b.angle*(1-b.penumbra)),F.decay=b.decay,n.spot[v]=F;let S=b.shadow;if(b.map&&(n.spotLightMap[A]=b.map,A++,S.updateMatrices(b),b.castShadow&&T++),n.spotLightMatrix[v]=S.matrix,b.castShadow){let X=t.get(b);X.shadowIntensity=S.intensity,X.shadowBias=S.bias,X.shadowNormalBias=S.normalBias,X.shadowRadius=S.radius,X.shadowMapSize=S.mapSize,n.spotShadow[v]=X,n.spotShadowMap[v]=U,x++}v++}else if(b.isRectAreaLight){let F=e.get(b);F.color.copy(P).multiplyScalar(W),F.halfWidth.set(b.width*.5,0,0),F.halfHeight.set(0,b.height*.5,0),n.rectArea[m]=F,m++}else if(b.isPointLight){let F=e.get(b);if(F.color.copy(b.color).multiplyScalar(b.intensity),F.distance=b.distance,F.decay=b.decay,b.castShadow){let S=b.shadow,X=t.get(b);X.shadowIntensity=S.intensity,X.shadowBias=S.bias,X.shadowNormalBias=S.normalBias,X.shadowRadius=S.radius,X.shadowMapSize=S.mapSize,X.shadowCameraNear=S.camera.near,X.shadowCameraFar=S.camera.far,n.pointShadow[g]=X,n.pointShadowMap[g]=U,n.pointShadowMatrix[g]=b.shadow.matrix,w++}n.point[g]=F,g++}else if(b.isHemisphereLight){let F=e.get(b);F.skyColor.copy(b.color).multiplyScalar(W),F.groundColor.copy(b.groundColor).multiplyScalar(W),n.hemi[p]=F,p++}}m>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=ke.LTC_FLOAT_1,n.rectAreaLTC2=ke.LTC_FLOAT_2):(n.rectAreaLTC1=ke.LTC_HALF_1,n.rectAreaLTC2=ke.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=d;let _=n.hash;(_.directionalLength!==f||_.pointLength!==g||_.spotLength!==v||_.rectAreaLength!==m||_.hemiLength!==p||_.numDirectionalShadows!==y||_.numPointShadows!==w||_.numSpotShadows!==x||_.numSpotMaps!==A||_.numLightProbes!==I)&&(n.directional.length=f,n.spot.length=v,n.rectArea.length=m,n.point.length=g,n.hemi.length=p,n.directionalShadow.length=y,n.directionalShadowMap.length=y,n.pointShadow.length=w,n.pointShadowMap.length=w,n.spotShadow.length=x,n.spotShadowMap.length=x,n.directionalShadowMatrix.length=y,n.pointShadowMatrix.length=w,n.spotLightMatrix.length=x+A-T,n.spotLightMap.length=A,n.numSpotLightShadowsWithMaps=T,n.numLightProbes=I,_.directionalLength=f,_.pointLength=g,_.spotLength=v,_.rectAreaLength=m,_.hemiLength=p,_.numDirectionalShadows=y,_.numPointShadows=w,_.numSpotShadows=x,_.numSpotMaps=A,_.numLightProbes=I,n.version=Mv++)}function l(c,h){let u=0,d=0,f=0,g=0,v=0,m=h.matrixWorldInverse;for(let p=0,y=c.length;p<y;p++){let w=c[p];if(w.isDirectionalLight){let x=n.directional[u];x.direction.setFromMatrixPosition(w.matrixWorld),i.setFromMatrixPosition(w.target.matrixWorld),x.direction.sub(i),x.direction.transformDirection(m),u++}else if(w.isSpotLight){let x=n.spot[f];x.position.setFromMatrixPosition(w.matrixWorld),x.position.applyMatrix4(m),x.direction.setFromMatrixPosition(w.matrixWorld),i.setFromMatrixPosition(w.target.matrixWorld),x.direction.sub(i),x.direction.transformDirection(m),f++}else if(w.isRectAreaLight){let x=n.rectArea[g];x.position.setFromMatrixPosition(w.matrixWorld),x.position.applyMatrix4(m),o.identity(),r.copy(w.matrixWorld),r.premultiply(m),o.extractRotation(r),x.halfWidth.set(w.width*.5,0,0),x.halfHeight.set(0,w.height*.5,0),x.halfWidth.applyMatrix4(o),x.halfHeight.applyMatrix4(o),g++}else if(w.isPointLight){let x=n.point[d];x.position.setFromMatrixPosition(w.matrixWorld),x.position.applyMatrix4(m),d++}else if(w.isHemisphereLight){let x=n.hemi[v];x.direction.setFromMatrixPosition(w.matrixWorld),x.direction.transformDirection(m),v++}}}return{setup:a,setupView:l,state:n}}function Ff(s){let e=new Sv(s),t=[],n=[],i=[];function r(d){u.camera=d,t.length=0,n.length=0,i.length=0}function o(d){t.push(d)}function a(d){n.push(d)}function l(d){i.push(d)}function c(){e.setup(t)}function h(d){e.setupView(t,d)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:o,pushShadow:a,pushLightProbeGrid:l}}function wv(s){let e=new WeakMap;function t(i,r=0){let o=e.get(i),a;return o===void 0?(a=new Ff(s),e.set(i,[a])):r>=o.length?(a=new Ff(s),o.push(a)):a=o[r],a}function n(){e=new WeakMap}return{get:t,dispose:n}}var Ev=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,Tv=`uniform sampler2D shadow_pass;
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
}`,Av=[new z(1,0,0),new z(-1,0,0),new z(0,1,0),new z(0,-1,0),new z(0,0,1),new z(0,0,-1)],Rv=[new z(0,-1,0),new z(0,-1,0),new z(0,0,1),new z(0,0,-1),new z(0,-1,0),new z(0,-1,0)],Of=new it,Zo=new z,Bh=new z;function Cv(s,e,t){let n=new gr,i=new Re,r=new Re,o=new xt,a=new nl,l=new il,c={},h=t.maxTextureSize,u={[Tn]:jt,[jt]:Tn,[At]:At},d=new pt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new Re},radius:{value:4}},vertexShader:Ev,fragmentShader:Tv}),f=d.clone();f.defines.HORIZONTAL_PASS=1;let g=new ht;g.setAttribute("position",new gt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let v=new Ke(g,d),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=Io;let p=this.type;this.render=function(T,I,_){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||T.length===0)return;this.type===pl&&(Ze("WebGLShadowMap: PCFSoftShadowMap has been deprecated. Using PCFShadowMap instead."),this.type=Io);let D=s.getRenderTarget(),E=s.getActiveCubeFace(),b=s.getActiveMipmapLevel(),P=s.state;P.setBlending(On),P.buffers.depth.getReversed()===!0?P.buffers.color.setClear(0,0,0,0):P.buffers.color.setClear(1,1,1,1),P.buffers.depth.setTest(!0),P.setScissorTest(!1);let W=p!==this.type;W&&I.traverse(function(R){R.material&&(Array.isArray(R.material)?R.material.forEach(U=>U.needsUpdate=!0):R.material.needsUpdate=!0)});for(let R=0,U=T.length;R<U;R++){let F=T[R],S=F.shadow;if(S===void 0){Ze("WebGLShadowMap:",F,"has no shadow.");continue}if(S.autoUpdate===!1&&S.needsUpdate===!1)continue;i.copy(S.mapSize);let X=S.getFrameExtents();i.multiply(X),r.copy(S.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/X.x),i.x=r.x*X.x,S.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/X.y),i.y=r.y*X.y,S.mapSize.y=r.y));let Z=s.state.buffers.depth.getReversed();if(S.camera._reversedDepth=Z,S.map===null||W===!0){if(S.map!==null&&(S.map.depthTexture!==null&&(S.map.depthTexture.dispose(),S.map.depthTexture=null),S.map.dispose()),this.type===Er){if(F.isPointLight){Ze("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}S.map=new zt(i.x,i.y,{format:is,type:Jt,minFilter:Ft,magFilter:Ft,generateMipmaps:!1}),S.map.texture.name=F.name+".shadowMap",S.map.depthTexture=new Ri(i.x,i.y,Cn),S.map.depthTexture.name=F.name+".shadowMapDepth",S.map.depthTexture.format=oi,S.map.depthTexture.compareFunction=null,S.map.depthTexture.minFilter=Wt,S.map.depthTexture.magFilter=Wt}else F.isPointLight?(S.map=new lc(i.x),S.map.depthTexture=new Za(i.x,$n)):(S.map=new zt(i.x,i.y),S.map.depthTexture=new Ri(i.x,i.y,$n)),S.map.depthTexture.name=F.name+".shadowMap",S.map.depthTexture.format=oi,this.type===Io?(S.map.depthTexture.compareFunction=Z?rc:sc,S.map.depthTexture.minFilter=Ft,S.map.depthTexture.magFilter=Ft):(S.map.depthTexture.compareFunction=null,S.map.depthTexture.minFilter=Wt,S.map.depthTexture.magFilter=Wt);S.camera.updateProjectionMatrix()}let V=S.map.isWebGLCubeRenderTarget?6:1;for(let ie=0;ie<V;ie++){if(S.map.isWebGLCubeRenderTarget)s.setRenderTarget(S.map,ie),s.clear();else{ie===0&&(s.setRenderTarget(S.map),s.clear());let fe=S.getViewport(ie);o.set(r.x*fe.x,r.y*fe.y,r.x*fe.z,r.y*fe.w),P.viewport(o)}if(F.isPointLight){let fe=S.camera,Ce=S.matrix,ze=F.distance||fe.far;ze!==fe.far&&(fe.far=ze,fe.updateProjectionMatrix()),Zo.setFromMatrixPosition(F.matrixWorld),fe.position.copy(Zo),Bh.copy(fe.position),Bh.add(Av[ie]),fe.up.copy(Rv[ie]),fe.lookAt(Bh),fe.updateMatrixWorld(),Ce.makeTranslation(-Zo.x,-Zo.y,-Zo.z),Of.multiplyMatrices(fe.projectionMatrix,fe.matrixWorldInverse),S._frustum.setFromProjectionMatrix(Of,fe.coordinateSystem,fe.reversedDepth)}else S.updateMatrices(F);n=S.getFrustum(),x(I,_,S.camera,F,this.type)}S.isPointLightShadow!==!0&&this.type===Er&&y(S,_),S.needsUpdate=!1}p=this.type,m.needsUpdate=!1,s.setRenderTarget(D,E,b)};function y(T,I){let _=e.update(v);d.defines.VSM_SAMPLES!==T.blurSamples&&(d.defines.VSM_SAMPLES=T.blurSamples,f.defines.VSM_SAMPLES=T.blurSamples,d.needsUpdate=!0,f.needsUpdate=!0),T.mapPass===null&&(T.mapPass=new zt(i.x,i.y,{format:is,type:Jt})),d.uniforms.shadow_pass.value=T.map.depthTexture,d.uniforms.resolution.value=T.mapSize,d.uniforms.radius.value=T.radius,s.setRenderTarget(T.mapPass),s.clear(),s.renderBufferDirect(I,null,_,d,v,null),f.uniforms.shadow_pass.value=T.mapPass.texture,f.uniforms.resolution.value=T.mapSize,f.uniforms.radius.value=T.radius,s.setRenderTarget(T.map),s.clear(),s.renderBufferDirect(I,null,_,f,v,null)}function w(T,I,_,D){let E=null,b=_.isPointLight===!0?T.customDistanceMaterial:T.customDepthMaterial;if(b!==void 0)E=b;else if(E=_.isPointLight===!0?l:a,s.localClippingEnabled&&I.clipShadows===!0&&Array.isArray(I.clippingPlanes)&&I.clippingPlanes.length!==0||I.displacementMap&&I.displacementScale!==0||I.alphaMap&&I.alphaTest>0||I.map&&I.alphaTest>0||I.alphaToCoverage===!0){let P=E.uuid,W=I.uuid,R=c[P];R===void 0&&(R={},c[P]=R);let U=R[W];U===void 0&&(U=E.clone(),R[W]=U,I.addEventListener("dispose",A)),E=U}if(E.visible=I.visible,E.wireframe=I.wireframe,D===Er?E.side=I.shadowSide!==null?I.shadowSide:I.side:E.side=I.shadowSide!==null?I.shadowSide:u[I.side],E.alphaMap=I.alphaMap,E.alphaTest=I.alphaToCoverage===!0?.5:I.alphaTest,E.map=I.map,E.clipShadows=I.clipShadows,E.clippingPlanes=I.clippingPlanes,E.clipIntersection=I.clipIntersection,E.displacementMap=I.displacementMap,E.displacementScale=I.displacementScale,E.displacementBias=I.displacementBias,E.wireframeLinewidth=I.wireframeLinewidth,E.linewidth=I.linewidth,_.isPointLight===!0&&E.isMeshDistanceMaterial===!0){let P=s.properties.get(E);P.light=_}return E}function x(T,I,_,D,E){if(T.visible===!1)return;if(T.layers.test(I.layers)&&(T.isMesh||T.isLine||T.isPoints)&&(T.castShadow||T.receiveShadow&&E===Er)&&(!T.frustumCulled||n.intersectsObject(T))){T.modelViewMatrix.multiplyMatrices(_.matrixWorldInverse,T.matrixWorld);let W=e.update(T),R=T.material;if(Array.isArray(R)){let U=W.groups;for(let F=0,S=U.length;F<S;F++){let X=U[F],Z=R[X.materialIndex];if(Z&&Z.visible){let V=w(T,Z,D,E);T.onBeforeShadow(s,T,I,_,W,V,X),s.renderBufferDirect(_,null,W,V,T,X),T.onAfterShadow(s,T,I,_,W,V,X)}}}else if(R.visible){let U=w(T,R,D,E);T.onBeforeShadow(s,T,I,_,W,U,null),s.renderBufferDirect(_,null,W,U,T,null),T.onAfterShadow(s,T,I,_,W,U,null)}}let P=T.children;for(let W=0,R=P.length;W<R;W++)x(P[W],I,_,D,E)}function A(T){T.target.removeEventListener("dispose",A);for(let _ in c){let D=c[_],E=T.target.uuid;E in D&&(D[E].dispose(),delete D[E])}}}function Pv(s,e){function t(){let N=!1,L=new xt,k=null,G=new xt(0,0,0,0);return{setMask:function(le){k!==le&&!N&&(s.colorMask(le,le,le,le),k=le)},setLocked:function(le){N=le},setClear:function(le,se,ge,Ae,at){at===!0&&(le*=Ae,se*=Ae,ge*=Ae),L.set(le,se,ge,Ae),G.equals(L)===!1&&(s.clearColor(le,se,ge,Ae),G.copy(L))},reset:function(){N=!1,k=null,G.set(-1,0,0,0)}}}function n(){let N=!1,L=!1,k=null,G=null,le=null;return{setReversed:function(se){if(L!==se){let ge=e.get("EXT_clip_control");se?ge.clipControlEXT(ge.LOWER_LEFT_EXT,ge.ZERO_TO_ONE_EXT):ge.clipControlEXT(ge.LOWER_LEFT_EXT,ge.NEGATIVE_ONE_TO_ONE_EXT),L=se;let Ae=le;le=null,this.setClear(Ae)}},getReversed:function(){return L},setTest:function(se){se?me(s.DEPTH_TEST):Oe(s.DEPTH_TEST)},setMask:function(se){k!==se&&!N&&(s.depthMask(se),k=se)},setFunc:function(se){if(L&&(se=ff[se]),G!==se){switch(se){case Ua:s.depthFunc(s.NEVER);break;case Fa:s.depthFunc(s.ALWAYS);break;case Oa:s.depthFunc(s.LESS);break;case Ms:s.depthFunc(s.LEQUAL);break;case Ba:s.depthFunc(s.EQUAL);break;case za:s.depthFunc(s.GEQUAL);break;case ka:s.depthFunc(s.GREATER);break;case Ha:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}G=se}},setLocked:function(se){N=se},setClear:function(se){le!==se&&(le=se,L&&(se=1-se),s.clearDepth(se))},reset:function(){N=!1,k=null,G=null,le=null,L=!1}}}function i(){let N=!1,L=null,k=null,G=null,le=null,se=null,ge=null,Ae=null,at=null;return{setTest:function(Qe){N||(Qe?me(s.STENCIL_TEST):Oe(s.STENCIL_TEST))},setMask:function(Qe){L!==Qe&&!N&&(s.stencilMask(Qe),L=Qe)},setFunc:function(Qe,Lt,rt){(k!==Qe||G!==Lt||le!==rt)&&(s.stencilFunc(Qe,Lt,rt),k=Qe,G=Lt,le=rt)},setOp:function(Qe,Lt,rt){(se!==Qe||ge!==Lt||Ae!==rt)&&(s.stencilOp(Qe,Lt,rt),se=Qe,ge=Lt,Ae=rt)},setLocked:function(Qe){N=Qe},setClear:function(Qe){at!==Qe&&(s.clearStencil(Qe),at=Qe)},reset:function(){N=!1,L=null,k=null,G=null,le=null,se=null,ge=null,Ae=null,at=null}}}let r=new t,o=new n,a=new i,l=new WeakMap,c=new WeakMap,h={},u={},d={},f=new WeakMap,g=[],v=null,m=!1,p=null,y=null,w=null,x=null,A=null,T=null,I=null,_=new be(0,0,0),D=0,E=!1,b=null,P=null,W=null,R=null,U=null,F=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),S=!1,X=0,Z=s.getParameter(s.VERSION);Z.indexOf("WebGL")!==-1?(X=parseFloat(/^WebGL (\d)/.exec(Z)[1]),S=X>=1):Z.indexOf("OpenGL ES")!==-1&&(X=parseFloat(/^OpenGL ES (\d)/.exec(Z)[1]),S=X>=2);let V=null,ie={},fe=s.getParameter(s.SCISSOR_BOX),Ce=s.getParameter(s.VIEWPORT),ze=new xt().fromArray(fe),He=new xt().fromArray(Ce);function ce(N,L,k,G){let le=new Uint8Array(4),se=s.createTexture();s.bindTexture(N,se),s.texParameteri(N,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(N,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let ge=0;ge<k;ge++)N===s.TEXTURE_3D||N===s.TEXTURE_2D_ARRAY?s.texImage3D(L,0,s.RGBA,1,1,G,0,s.RGBA,s.UNSIGNED_BYTE,le):s.texImage2D(L+ge,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,le);return se}let Me={};Me[s.TEXTURE_2D]=ce(s.TEXTURE_2D,s.TEXTURE_2D,1),Me[s.TEXTURE_CUBE_MAP]=ce(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),Me[s.TEXTURE_2D_ARRAY]=ce(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),Me[s.TEXTURE_3D]=ce(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),o.setClear(1),a.setClear(0),me(s.DEPTH_TEST),o.setFunc(Ms),ve(!1),Ue(mh),me(s.CULL_FACE),ee(On);function me(N){h[N]!==!0&&(s.enable(N),h[N]=!0)}function Oe(N){h[N]!==!1&&(s.disable(N),h[N]=!1)}function ue(N,L){return d[N]!==L?(s.bindFramebuffer(N,L),d[N]=L,N===s.DRAW_FRAMEBUFFER&&(d[s.FRAMEBUFFER]=L),N===s.FRAMEBUFFER&&(d[s.DRAW_FRAMEBUFFER]=L),!0):!1}function xe(N,L){let k=g,G=!1;if(N){k=f.get(L),k===void 0&&(k=[],f.set(L,k));let le=N.textures;if(k.length!==le.length||k[0]!==s.COLOR_ATTACHMENT0){for(let se=0,ge=le.length;se<ge;se++)k[se]=s.COLOR_ATTACHMENT0+se;k.length=le.length,G=!0}}else k[0]!==s.BACK&&(k[0]=s.BACK,G=!0);G&&s.drawBuffers(k)}function H(N){return v!==N?(s.useProgram(N),v=N,!0):!1}let K={[Yi]:s.FUNC_ADD,[Ld]:s.FUNC_SUBTRACT,[Dd]:s.FUNC_REVERSE_SUBTRACT};K[Nd]=s.MIN,K[Ud]=s.MAX;let Q={[Fd]:s.ZERO,[Od]:s.ONE,[Bd]:s.SRC_COLOR,[Da]:s.SRC_ALPHA,[Wd]:s.SRC_ALPHA_SATURATE,[Vd]:s.DST_COLOR,[kd]:s.DST_ALPHA,[zd]:s.ONE_MINUS_SRC_COLOR,[Na]:s.ONE_MINUS_SRC_ALPHA,[Gd]:s.ONE_MINUS_DST_COLOR,[Hd]:s.ONE_MINUS_DST_ALPHA,[Xd]:s.CONSTANT_COLOR,[qd]:s.ONE_MINUS_CONSTANT_COLOR,[Yd]:s.CONSTANT_ALPHA,[Zd]:s.ONE_MINUS_CONSTANT_ALPHA};function ee(N,L,k,G,le,se,ge,Ae,at,Qe){if(N===On){m===!0&&(Oe(s.BLEND),m=!1);return}if(m===!1&&(me(s.BLEND),m=!0),N!==Id){if(N!==p||Qe!==E){if((y!==Yi||A!==Yi)&&(s.blendEquation(s.FUNC_ADD),y=Yi,A=Yi),Qe)switch(N){case ys:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case kt:s.blendFunc(s.ONE,s.ONE);break;case gh:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case xh:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:$e("WebGLState: Invalid blending: ",N);break}else switch(N){case ys:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case kt:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case gh:$e("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case xh:$e("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:$e("WebGLState: Invalid blending: ",N);break}w=null,x=null,T=null,I=null,_.set(0,0,0),D=0,p=N,E=Qe}return}le=le||L,se=se||k,ge=ge||G,(L!==y||le!==A)&&(s.blendEquationSeparate(K[L],K[le]),y=L,A=le),(k!==w||G!==x||se!==T||ge!==I)&&(s.blendFuncSeparate(Q[k],Q[G],Q[se],Q[ge]),w=k,x=G,T=se,I=ge),(Ae.equals(_)===!1||at!==D)&&(s.blendColor(Ae.r,Ae.g,Ae.b,at),_.copy(Ae),D=at),p=N,E=!1}function ae(N,L){N.side===At?Oe(s.CULL_FACE):me(s.CULL_FACE);let k=N.side===jt;L&&(k=!k),ve(k),N.blending===ys&&N.transparent===!1?ee(On):ee(N.blending,N.blendEquation,N.blendSrc,N.blendDst,N.blendEquationAlpha,N.blendSrcAlpha,N.blendDstAlpha,N.blendColor,N.blendAlpha,N.premultipliedAlpha),o.setFunc(N.depthFunc),o.setTest(N.depthTest),o.setMask(N.depthWrite),r.setMask(N.colorWrite);let G=N.stencilWrite;a.setTest(G),G&&(a.setMask(N.stencilWriteMask),a.setFunc(N.stencilFunc,N.stencilRef,N.stencilFuncMask),a.setOp(N.stencilFail,N.stencilZFail,N.stencilZPass)),je(N.polygonOffset,N.polygonOffsetFactor,N.polygonOffsetUnits),N.alphaToCoverage===!0?me(s.SAMPLE_ALPHA_TO_COVERAGE):Oe(s.SAMPLE_ALPHA_TO_COVERAGE)}function ve(N){b!==N&&(N?s.frontFace(s.CW):s.frontFace(s.CCW),b=N)}function Ue(N){N!==Cd?(me(s.CULL_FACE),N!==P&&(N===mh?s.cullFace(s.BACK):N===Pd?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):Oe(s.CULL_FACE),P=N}function Se(N){N!==W&&(S&&s.lineWidth(N),W=N)}function je(N,L,k){N?(me(s.POLYGON_OFFSET_FILL),(R!==L||U!==k)&&(R=L,U=k,o.getReversed()&&(L=-L),s.polygonOffset(L,k))):Oe(s.POLYGON_OFFSET_FILL)}function Ge(N){N?me(s.SCISSOR_TEST):Oe(s.SCISSOR_TEST)}function st(N){N===void 0&&(N=s.TEXTURE0+F-1),V!==N&&(s.activeTexture(N),V=N)}function j(N,L,k){k===void 0&&(V===null?k=s.TEXTURE0+F-1:k=V);let G=ie[k];G===void 0&&(G={type:void 0,texture:void 0},ie[k]=G),(G.type!==N||G.texture!==L)&&(V!==k&&(s.activeTexture(k),V=k),s.bindTexture(N,L||Me[N]),G.type=N,G.texture=L)}function tt(){let N=ie[V];N!==void 0&&N.type!==void 0&&(s.bindTexture(N.type,null),N.type=void 0,N.texture=void 0)}function Je(){try{s.compressedTexImage2D(...arguments)}catch(N){$e("WebGLState:",N)}}function B(){try{s.compressedTexImage3D(...arguments)}catch(N){$e("WebGLState:",N)}}function M(){try{s.texSubImage2D(...arguments)}catch(N){$e("WebGLState:",N)}}function te(){try{s.texSubImage3D(...arguments)}catch(N){$e("WebGLState:",N)}}function he(){try{s.compressedTexSubImage2D(...arguments)}catch(N){$e("WebGLState:",N)}}function de(){try{s.compressedTexSubImage3D(...arguments)}catch(N){$e("WebGLState:",N)}}function Te(){try{s.texStorage2D(...arguments)}catch(N){$e("WebGLState:",N)}}function Le(){try{s.texStorage3D(...arguments)}catch(N){$e("WebGLState:",N)}}function pe(){try{s.texImage2D(...arguments)}catch(N){$e("WebGLState:",N)}}function _e(){try{s.texImage3D(...arguments)}catch(N){$e("WebGLState:",N)}}function De(N){return u[N]!==void 0?u[N]:s.getParameter(N)}function We(N,L){u[N]!==L&&(s.pixelStorei(N,L),u[N]=L)}function Pe(N){ze.equals(N)===!1&&(s.scissor(N.x,N.y,N.z,N.w),ze.copy(N))}function Fe(N){He.equals(N)===!1&&(s.viewport(N.x,N.y,N.z,N.w),He.copy(N))}function O(N,L){let k=c.get(L);k===void 0&&(k=new WeakMap,c.set(L,k));let G=k.get(N);G===void 0&&(G=s.getUniformBlockIndex(L,N.name),k.set(N,G))}function q(N,L){let G=c.get(L).get(N);l.get(L)!==G&&(s.uniformBlockBinding(L,G,N.__bindingPointIndex),l.set(L,G))}function Y(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),o.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},V=null,ie={},d={},f=new WeakMap,g=[],v=null,m=!1,p=null,y=null,w=null,x=null,A=null,T=null,I=null,_=new be(0,0,0),D=0,E=!1,b=null,P=null,W=null,R=null,U=null,ze.set(0,0,s.canvas.width,s.canvas.height),He.set(0,0,s.canvas.width,s.canvas.height),r.reset(),o.reset(),a.reset()}return{buffers:{color:r,depth:o,stencil:a},enable:me,disable:Oe,bindFramebuffer:ue,drawBuffers:xe,useProgram:H,setBlending:ee,setMaterial:ae,setFlipSided:ve,setCullFace:Ue,setLineWidth:Se,setPolygonOffset:je,setScissorTest:Ge,activeTexture:st,bindTexture:j,unbindTexture:tt,compressedTexImage2D:Je,compressedTexImage3D:B,texImage2D:pe,texImage3D:_e,pixelStorei:We,getParameter:De,updateUBOMapping:O,uniformBlockBinding:q,texStorage2D:Te,texStorage3D:Le,texSubImage2D:M,texSubImage3D:te,compressedTexSubImage2D:he,compressedTexSubImage3D:de,scissor:Pe,viewport:Fe,reset:Y}}function Iv(s,e,t,n,i,r,o){let a=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new Re,h=new WeakMap,u=new Set,d,f=new WeakMap,g=!1;try{g=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function v(B,M){return g?new OffscreenCanvas(B,M):lr("canvas")}function m(B,M,te){let he=1,de=Je(B);if((de.width>te||de.height>te)&&(he=te/Math.max(de.width,de.height)),he<1)if(typeof HTMLImageElement<"u"&&B instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&B instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&B instanceof ImageBitmap||typeof VideoFrame<"u"&&B instanceof VideoFrame){let Te=Math.floor(he*de.width),Le=Math.floor(he*de.height);d===void 0&&(d=v(Te,Le));let pe=M?v(Te,Le):d;return pe.width=Te,pe.height=Le,pe.getContext("2d").drawImage(B,0,0,Te,Le),Ze("WebGLRenderer: Texture has been resized from ("+de.width+"x"+de.height+") to ("+Te+"x"+Le+")."),pe}else return"data"in B&&Ze("WebGLRenderer: Image in DataTexture is too big ("+de.width+"x"+de.height+")."),B;return B}function p(B){return B.generateMipmaps}function y(B){s.generateMipmap(B)}function w(B){return B.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:B.isWebGL3DRenderTarget?s.TEXTURE_3D:B.isWebGLArrayRenderTarget||B.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function x(B,M,te,he,de,Te=!1){if(B!==null){if(s[B]!==void 0)return s[B];Ze("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+B+"'")}let Le;he&&(Le=e.get("EXT_texture_norm16"),Le||Ze("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let pe=M;if(M===s.RED&&(te===s.FLOAT&&(pe=s.R32F),te===s.HALF_FLOAT&&(pe=s.R16F),te===s.UNSIGNED_BYTE&&(pe=s.R8),te===s.UNSIGNED_SHORT&&Le&&(pe=Le.R16_EXT),te===s.SHORT&&Le&&(pe=Le.R16_SNORM_EXT)),M===s.RED_INTEGER&&(te===s.UNSIGNED_BYTE&&(pe=s.R8UI),te===s.UNSIGNED_SHORT&&(pe=s.R16UI),te===s.UNSIGNED_INT&&(pe=s.R32UI),te===s.BYTE&&(pe=s.R8I),te===s.SHORT&&(pe=s.R16I),te===s.INT&&(pe=s.R32I)),M===s.RG&&(te===s.FLOAT&&(pe=s.RG32F),te===s.HALF_FLOAT&&(pe=s.RG16F),te===s.UNSIGNED_BYTE&&(pe=s.RG8),te===s.UNSIGNED_SHORT&&Le&&(pe=Le.RG16_EXT),te===s.SHORT&&Le&&(pe=Le.RG16_SNORM_EXT)),M===s.RG_INTEGER&&(te===s.UNSIGNED_BYTE&&(pe=s.RG8UI),te===s.UNSIGNED_SHORT&&(pe=s.RG16UI),te===s.UNSIGNED_INT&&(pe=s.RG32UI),te===s.BYTE&&(pe=s.RG8I),te===s.SHORT&&(pe=s.RG16I),te===s.INT&&(pe=s.RG32I)),M===s.RGB_INTEGER&&(te===s.UNSIGNED_BYTE&&(pe=s.RGB8UI),te===s.UNSIGNED_SHORT&&(pe=s.RGB16UI),te===s.UNSIGNED_INT&&(pe=s.RGB32UI),te===s.BYTE&&(pe=s.RGB8I),te===s.SHORT&&(pe=s.RGB16I),te===s.INT&&(pe=s.RGB32I)),M===s.RGBA_INTEGER&&(te===s.UNSIGNED_BYTE&&(pe=s.RGBA8UI),te===s.UNSIGNED_SHORT&&(pe=s.RGBA16UI),te===s.UNSIGNED_INT&&(pe=s.RGBA32UI),te===s.BYTE&&(pe=s.RGBA8I),te===s.SHORT&&(pe=s.RGBA16I),te===s.INT&&(pe=s.RGBA32I)),M===s.RGB&&(te===s.UNSIGNED_SHORT&&Le&&(pe=Le.RGB16_EXT),te===s.SHORT&&Le&&(pe=Le.RGB16_SNORM_EXT),te===s.UNSIGNED_INT_5_9_9_9_REV&&(pe=s.RGB9_E5),te===s.UNSIGNED_INT_10F_11F_11F_REV&&(pe=s.R11F_G11F_B10F)),M===s.RGBA){let _e=Te?no:lt.getTransfer(de);te===s.FLOAT&&(pe=s.RGBA32F),te===s.HALF_FLOAT&&(pe=s.RGBA16F),te===s.UNSIGNED_BYTE&&(pe=_e===yt?s.SRGB8_ALPHA8:s.RGBA8),te===s.UNSIGNED_SHORT&&Le&&(pe=Le.RGBA16_EXT),te===s.SHORT&&Le&&(pe=Le.RGBA16_SNORM_EXT),te===s.UNSIGNED_SHORT_4_4_4_4&&(pe=s.RGBA4),te===s.UNSIGNED_SHORT_5_5_5_1&&(pe=s.RGB5_A1)}return(pe===s.R16F||pe===s.R32F||pe===s.RG16F||pe===s.RG32F||pe===s.RGBA16F||pe===s.RGBA32F)&&e.get("EXT_color_buffer_float"),pe}function A(B,M){let te;return B?M===null||M===$n||M===Rr?te=s.DEPTH24_STENCIL8:M===Cn?te=s.DEPTH32F_STENCIL8:M===Ar&&(te=s.DEPTH24_STENCIL8,Ze("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):M===null||M===$n||M===Rr?te=s.DEPTH_COMPONENT24:M===Cn?te=s.DEPTH_COMPONENT32F:M===Ar&&(te=s.DEPTH_COMPONENT16),te}function T(B,M){return p(B)===!0||B.isFramebufferTexture&&B.minFilter!==Wt&&B.minFilter!==Ft?Math.log2(Math.max(M.width,M.height))+1:B.mipmaps!==void 0&&B.mipmaps.length>0?B.mipmaps.length:B.isCompressedTexture&&Array.isArray(B.image)?M.mipmaps.length:1}function I(B){let M=B.target;M.removeEventListener("dispose",I),D(M),M.isVideoTexture&&h.delete(M),M.isHTMLTexture&&u.delete(M)}function _(B){let M=B.target;M.removeEventListener("dispose",_),b(M)}function D(B){let M=n.get(B);if(M.__webglInit===void 0)return;let te=B.source,he=f.get(te);if(he){let de=he[M.__cacheKey];de.usedTimes--,de.usedTimes===0&&E(B),Object.keys(he).length===0&&f.delete(te)}n.remove(B)}function E(B){let M=n.get(B);s.deleteTexture(M.__webglTexture);let te=B.source,he=f.get(te);delete he[M.__cacheKey],o.memory.textures--}function b(B){let M=n.get(B);if(B.depthTexture&&(B.depthTexture.dispose(),n.remove(B.depthTexture)),B.isWebGLCubeRenderTarget)for(let he=0;he<6;he++){if(Array.isArray(M.__webglFramebuffer[he]))for(let de=0;de<M.__webglFramebuffer[he].length;de++)s.deleteFramebuffer(M.__webglFramebuffer[he][de]);else s.deleteFramebuffer(M.__webglFramebuffer[he]);M.__webglDepthbuffer&&s.deleteRenderbuffer(M.__webglDepthbuffer[he])}else{if(Array.isArray(M.__webglFramebuffer))for(let he=0;he<M.__webglFramebuffer.length;he++)s.deleteFramebuffer(M.__webglFramebuffer[he]);else s.deleteFramebuffer(M.__webglFramebuffer);if(M.__webglDepthbuffer&&s.deleteRenderbuffer(M.__webglDepthbuffer),M.__webglMultisampledFramebuffer&&s.deleteFramebuffer(M.__webglMultisampledFramebuffer),M.__webglColorRenderbuffer)for(let he=0;he<M.__webglColorRenderbuffer.length;he++)M.__webglColorRenderbuffer[he]&&s.deleteRenderbuffer(M.__webglColorRenderbuffer[he]);M.__webglDepthRenderbuffer&&s.deleteRenderbuffer(M.__webglDepthRenderbuffer)}let te=B.textures;for(let he=0,de=te.length;he<de;he++){let Te=n.get(te[he]);Te.__webglTexture&&(s.deleteTexture(Te.__webglTexture),o.memory.textures--),n.remove(te[he])}n.remove(B)}let P=0;function W(){P=0}function R(){return P}function U(B){P=B}function F(){let B=P;return B>=i.maxTextures&&Ze("WebGLTextures: Trying to use "+B+" texture units while this GPU supports only "+i.maxTextures),P+=1,B}function S(B){let M=[];return M.push(B.wrapS),M.push(B.wrapT),M.push(B.wrapR||0),M.push(B.magFilter),M.push(B.minFilter),M.push(B.anisotropy),M.push(B.internalFormat),M.push(B.format),M.push(B.type),M.push(B.generateMipmaps),M.push(B.premultiplyAlpha),M.push(B.flipY),M.push(B.unpackAlignment),M.push(B.colorSpace),M.join()}function X(B,M){let te=n.get(B);if(B.isVideoTexture&&j(B),B.isRenderTargetTexture===!1&&B.isExternalTexture!==!0&&B.version>0&&te.__version!==B.version){let he=B.image;if(he===null)Ze("WebGLRenderer: Texture marked for update but no image data found.");else if(he.complete===!1)Ze("WebGLRenderer: Texture marked for update but image is incomplete");else{Oe(te,B,M);return}}else B.isExternalTexture&&(te.__webglTexture=B.sourceTexture?B.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,te.__webglTexture,s.TEXTURE0+M)}function Z(B,M){let te=n.get(B);if(B.isRenderTargetTexture===!1&&B.version>0&&te.__version!==B.version){Oe(te,B,M);return}else B.isExternalTexture&&(te.__webglTexture=B.sourceTexture?B.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,te.__webglTexture,s.TEXTURE0+M)}function V(B,M){let te=n.get(B);if(B.isRenderTargetTexture===!1&&B.version>0&&te.__version!==B.version){Oe(te,B,M);return}t.bindTexture(s.TEXTURE_3D,te.__webglTexture,s.TEXTURE0+M)}function ie(B,M){let te=n.get(B);if(B.isCubeDepthTexture!==!0&&B.version>0&&te.__version!==B.version){ue(te,B,M);return}t.bindTexture(s.TEXTURE_CUBE_MAP,te.__webglTexture,s.TEXTURE0+M)}let fe={[ri]:s.REPEAT,[Un]:s.CLAMP_TO_EDGE,[or]:s.MIRRORED_REPEAT},Ce={[Wt]:s.NEAREST,[_l]:s.NEAREST_MIPMAP_NEAREST,[Ns]:s.NEAREST_MIPMAP_LINEAR,[Ft]:s.LINEAR,[Tr]:s.LINEAR_MIPMAP_NEAREST,[Jn]:s.LINEAR_MIPMAP_LINEAR},ze={[sf]:s.NEVER,[cf]:s.ALWAYS,[rf]:s.LESS,[sc]:s.LEQUAL,[of]:s.EQUAL,[rc]:s.GEQUAL,[af]:s.GREATER,[lf]:s.NOTEQUAL};function He(B,M){if(M.type===Cn&&e.has("OES_texture_float_linear")===!1&&(M.magFilter===Ft||M.magFilter===Tr||M.magFilter===Ns||M.magFilter===Jn||M.minFilter===Ft||M.minFilter===Tr||M.minFilter===Ns||M.minFilter===Jn)&&Ze("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(B,s.TEXTURE_WRAP_S,fe[M.wrapS]),s.texParameteri(B,s.TEXTURE_WRAP_T,fe[M.wrapT]),(B===s.TEXTURE_3D||B===s.TEXTURE_2D_ARRAY)&&s.texParameteri(B,s.TEXTURE_WRAP_R,fe[M.wrapR]),s.texParameteri(B,s.TEXTURE_MAG_FILTER,Ce[M.magFilter]),s.texParameteri(B,s.TEXTURE_MIN_FILTER,Ce[M.minFilter]),M.compareFunction&&(s.texParameteri(B,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(B,s.TEXTURE_COMPARE_FUNC,ze[M.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(M.magFilter===Wt||M.minFilter!==Ns&&M.minFilter!==Jn||M.type===Cn&&e.has("OES_texture_float_linear")===!1)return;if(M.anisotropy>1||n.get(M).__currentAnisotropy){let te=e.get("EXT_texture_filter_anisotropic");s.texParameterf(B,te.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(M.anisotropy,i.getMaxAnisotropy())),n.get(M).__currentAnisotropy=M.anisotropy}}}function ce(B,M){let te=!1;B.__webglInit===void 0&&(B.__webglInit=!0,M.addEventListener("dispose",I));let he=M.source,de=f.get(he);de===void 0&&(de={},f.set(he,de));let Te=S(M);if(Te!==B.__cacheKey){de[Te]===void 0&&(de[Te]={texture:s.createTexture(),usedTimes:0},o.memory.textures++,te=!0),de[Te].usedTimes++;let Le=de[B.__cacheKey];Le!==void 0&&(de[B.__cacheKey].usedTimes--,Le.usedTimes===0&&E(M)),B.__cacheKey=Te,B.__webglTexture=de[Te].texture}return te}function Me(B,M,te){return Math.floor(Math.floor(B/te)/M)}function me(B,M,te,he){let Te=B.updateRanges;if(Te.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,M.width,M.height,te,he,M.data);else{Te.sort((We,Pe)=>We.start-Pe.start);let Le=0;for(let We=1;We<Te.length;We++){let Pe=Te[Le],Fe=Te[We],O=Pe.start+Pe.count,q=Me(Fe.start,M.width,4),Y=Me(Pe.start,M.width,4);Fe.start<=O+1&&q===Y&&Me(Fe.start+Fe.count-1,M.width,4)===q?Pe.count=Math.max(Pe.count,Fe.start+Fe.count-Pe.start):(++Le,Te[Le]=Fe)}Te.length=Le+1;let pe=t.getParameter(s.UNPACK_ROW_LENGTH),_e=t.getParameter(s.UNPACK_SKIP_PIXELS),De=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,M.width);for(let We=0,Pe=Te.length;We<Pe;We++){let Fe=Te[We],O=Math.floor(Fe.start/4),q=Math.ceil(Fe.count/4),Y=O%M.width,N=Math.floor(O/M.width),L=q,k=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,Y),t.pixelStorei(s.UNPACK_SKIP_ROWS,N),t.texSubImage2D(s.TEXTURE_2D,0,Y,N,L,k,te,he,M.data)}B.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,pe),t.pixelStorei(s.UNPACK_SKIP_PIXELS,_e),t.pixelStorei(s.UNPACK_SKIP_ROWS,De)}}function Oe(B,M,te){let he=s.TEXTURE_2D;(M.isDataArrayTexture||M.isCompressedArrayTexture)&&(he=s.TEXTURE_2D_ARRAY),M.isData3DTexture&&(he=s.TEXTURE_3D);let de=ce(B,M),Te=M.source;t.bindTexture(he,B.__webglTexture,s.TEXTURE0+te);let Le=n.get(Te);if(Te.version!==Le.__version||de===!0){if(t.activeTexture(s.TEXTURE0+te),(typeof ImageBitmap<"u"&&M.image instanceof ImageBitmap)===!1){let k=lt.getPrimaries(lt.workingColorSpace),G=M.colorSpace===Bn?null:lt.getPrimaries(M.colorSpace),le=M.colorSpace===Bn||k===G?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,M.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,M.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,le)}t.pixelStorei(s.UNPACK_ALIGNMENT,M.unpackAlignment);let _e=m(M.image,!1,i.maxTextureSize);_e=tt(M,_e);let De=r.convert(M.format,M.colorSpace),We=r.convert(M.type),Pe=x(M.internalFormat,De,We,M.normalized,M.colorSpace,M.isVideoTexture);He(he,M);let Fe,O=M.mipmaps,q=M.isVideoTexture!==!0,Y=Le.__version===void 0||de===!0,N=Te.dataReady,L=T(M,_e);if(M.isDepthTexture)Pe=A(M.format===ns,M.type),Y&&(q?t.texStorage2D(s.TEXTURE_2D,1,Pe,_e.width,_e.height):t.texImage2D(s.TEXTURE_2D,0,Pe,_e.width,_e.height,0,De,We,null));else if(M.isDataTexture)if(O.length>0){q&&Y&&t.texStorage2D(s.TEXTURE_2D,L,Pe,O[0].width,O[0].height);for(let k=0,G=O.length;k<G;k++)Fe=O[k],q?N&&t.texSubImage2D(s.TEXTURE_2D,k,0,0,Fe.width,Fe.height,De,We,Fe.data):t.texImage2D(s.TEXTURE_2D,k,Pe,Fe.width,Fe.height,0,De,We,Fe.data);M.generateMipmaps=!1}else q?(Y&&t.texStorage2D(s.TEXTURE_2D,L,Pe,_e.width,_e.height),N&&me(M,_e,De,We)):t.texImage2D(s.TEXTURE_2D,0,Pe,_e.width,_e.height,0,De,We,_e.data);else if(M.isCompressedTexture)if(M.isCompressedArrayTexture){q&&Y&&t.texStorage3D(s.TEXTURE_2D_ARRAY,L,Pe,O[0].width,O[0].height,_e.depth);for(let k=0,G=O.length;k<G;k++)if(Fe=O[k],M.format!==Pn)if(De!==null)if(q){if(N)if(M.layerUpdates.size>0){let le=Lh(Fe.width,Fe.height,M.format,M.type);for(let se of M.layerUpdates){let ge=Fe.data.subarray(se*le/Fe.data.BYTES_PER_ELEMENT,(se+1)*le/Fe.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,k,0,0,se,Fe.width,Fe.height,1,De,ge)}M.clearLayerUpdates()}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,k,0,0,0,Fe.width,Fe.height,_e.depth,De,Fe.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,k,Pe,Fe.width,Fe.height,_e.depth,0,Fe.data,0,0);else Ze("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else q?N&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,k,0,0,0,Fe.width,Fe.height,_e.depth,De,We,Fe.data):t.texImage3D(s.TEXTURE_2D_ARRAY,k,Pe,Fe.width,Fe.height,_e.depth,0,De,We,Fe.data)}else{q&&Y&&t.texStorage2D(s.TEXTURE_2D,L,Pe,O[0].width,O[0].height);for(let k=0,G=O.length;k<G;k++)Fe=O[k],M.format!==Pn?De!==null?q?N&&t.compressedTexSubImage2D(s.TEXTURE_2D,k,0,0,Fe.width,Fe.height,De,Fe.data):t.compressedTexImage2D(s.TEXTURE_2D,k,Pe,Fe.width,Fe.height,0,Fe.data):Ze("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):q?N&&t.texSubImage2D(s.TEXTURE_2D,k,0,0,Fe.width,Fe.height,De,We,Fe.data):t.texImage2D(s.TEXTURE_2D,k,Pe,Fe.width,Fe.height,0,De,We,Fe.data)}else if(M.isDataArrayTexture)if(q){if(Y&&t.texStorage3D(s.TEXTURE_2D_ARRAY,L,Pe,_e.width,_e.height,_e.depth),N)if(M.layerUpdates.size>0){let k=Lh(_e.width,_e.height,M.format,M.type);for(let G of M.layerUpdates){let le=_e.data.subarray(G*k/_e.data.BYTES_PER_ELEMENT,(G+1)*k/_e.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,G,_e.width,_e.height,1,De,We,le)}M.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,_e.width,_e.height,_e.depth,De,We,_e.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,Pe,_e.width,_e.height,_e.depth,0,De,We,_e.data);else if(M.isData3DTexture)q?(Y&&t.texStorage3D(s.TEXTURE_3D,L,Pe,_e.width,_e.height,_e.depth),N&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,_e.width,_e.height,_e.depth,De,We,_e.data)):t.texImage3D(s.TEXTURE_3D,0,Pe,_e.width,_e.height,_e.depth,0,De,We,_e.data);else if(M.isFramebufferTexture){if(Y)if(q)t.texStorage2D(s.TEXTURE_2D,L,Pe,_e.width,_e.height);else{let k=_e.width,G=_e.height;for(let le=0;le<L;le++)t.texImage2D(s.TEXTURE_2D,le,Pe,k,G,0,De,We,null),k>>=1,G>>=1}}else if(M.isHTMLTexture){if("texElementImage2D"in s){let k=s.canvas;if(k.hasAttribute("layoutsubtree")||k.setAttribute("layoutsubtree","true"),_e.parentNode!==k){k.appendChild(_e),u.add(M),k.onpaint=G=>{let le=G.changedElements;for(let se of u)le.includes(se.image)&&(se.needsUpdate=!0)},k.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,_e);else{let le=s.RGBA,se=s.RGBA,ge=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,le,se,ge,_e)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(O.length>0){if(q&&Y){let k=Je(O[0]);t.texStorage2D(s.TEXTURE_2D,L,Pe,k.width,k.height)}for(let k=0,G=O.length;k<G;k++)Fe=O[k],q?N&&t.texSubImage2D(s.TEXTURE_2D,k,0,0,De,We,Fe):t.texImage2D(s.TEXTURE_2D,k,Pe,De,We,Fe);M.generateMipmaps=!1}else if(q){if(Y){let k=Je(_e);t.texStorage2D(s.TEXTURE_2D,L,Pe,k.width,k.height)}N&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,De,We,_e)}else t.texImage2D(s.TEXTURE_2D,0,Pe,De,We,_e);p(M)&&y(he),Le.__version=Te.version,M.onUpdate&&M.onUpdate(M)}B.__version=M.version}function ue(B,M,te){if(M.image.length!==6)return;let he=ce(B,M),de=M.source;t.bindTexture(s.TEXTURE_CUBE_MAP,B.__webglTexture,s.TEXTURE0+te);let Te=n.get(de);if(de.version!==Te.__version||he===!0){t.activeTexture(s.TEXTURE0+te);let Le=lt.getPrimaries(lt.workingColorSpace),pe=M.colorSpace===Bn?null:lt.getPrimaries(M.colorSpace),_e=M.colorSpace===Bn||Le===pe?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,M.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,M.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,M.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,_e);let De=M.isCompressedTexture||M.image[0].isCompressedTexture,We=M.image[0]&&M.image[0].isDataTexture,Pe=[];for(let se=0;se<6;se++)!De&&!We?Pe[se]=m(M.image[se],!0,i.maxCubemapSize):Pe[se]=We?M.image[se].image:M.image[se],Pe[se]=tt(M,Pe[se]);let Fe=Pe[0],O=r.convert(M.format,M.colorSpace),q=r.convert(M.type),Y=x(M.internalFormat,O,q,M.normalized,M.colorSpace),N=M.isVideoTexture!==!0,L=Te.__version===void 0||he===!0,k=de.dataReady,G=T(M,Fe);He(s.TEXTURE_CUBE_MAP,M);let le;if(De){N&&L&&t.texStorage2D(s.TEXTURE_CUBE_MAP,G,Y,Fe.width,Fe.height);for(let se=0;se<6;se++){le=Pe[se].mipmaps;for(let ge=0;ge<le.length;ge++){let Ae=le[ge];M.format!==Pn?O!==null?N?k&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge,0,0,Ae.width,Ae.height,O,Ae.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge,Y,Ae.width,Ae.height,0,Ae.data):Ze("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):N?k&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge,0,0,Ae.width,Ae.height,O,q,Ae.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge,Y,Ae.width,Ae.height,0,O,q,Ae.data)}}}else{if(le=M.mipmaps,N&&L){le.length>0&&G++;let se=Je(Pe[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,G,Y,se.width,se.height)}for(let se=0;se<6;se++)if(We){N?k&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,0,0,0,Pe[se].width,Pe[se].height,O,q,Pe[se].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,0,Y,Pe[se].width,Pe[se].height,0,O,q,Pe[se].data);for(let ge=0;ge<le.length;ge++){let at=le[ge].image[se].image;N?k&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge+1,0,0,at.width,at.height,O,q,at.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge+1,Y,at.width,at.height,0,O,q,at.data)}}else{N?k&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,0,0,0,O,q,Pe[se]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,0,Y,O,q,Pe[se]);for(let ge=0;ge<le.length;ge++){let Ae=le[ge];N?k&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge+1,0,0,O,q,Ae.image[se]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+se,ge+1,Y,O,q,Ae.image[se])}}}p(M)&&y(s.TEXTURE_CUBE_MAP),Te.__version=de.version,M.onUpdate&&M.onUpdate(M)}B.__version=M.version}function xe(B,M,te,he,de,Te){let Le=r.convert(te.format,te.colorSpace),pe=r.convert(te.type),_e=x(te.internalFormat,Le,pe,te.normalized,te.colorSpace),De=n.get(M),We=n.get(te);if(We.__renderTarget=M,!De.__hasExternalTextures){let Pe=Math.max(1,M.width>>Te),Fe=Math.max(1,M.height>>Te);de===s.TEXTURE_3D||de===s.TEXTURE_2D_ARRAY?t.texImage3D(de,Te,_e,Pe,Fe,M.depth,0,Le,pe,null):t.texImage2D(de,Te,_e,Pe,Fe,0,Le,pe,null)}t.bindFramebuffer(s.FRAMEBUFFER,B),st(M)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,he,de,We.__webglTexture,0,Ge(M)):(de===s.TEXTURE_2D||de>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&de<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,he,de,We.__webglTexture,Te),t.bindFramebuffer(s.FRAMEBUFFER,null)}function H(B,M,te){if(s.bindRenderbuffer(s.RENDERBUFFER,B),M.depthBuffer){let he=M.depthTexture,de=he&&he.isDepthTexture?he.type:null,Te=A(M.stencilBuffer,de),Le=M.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;st(M)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Ge(M),Te,M.width,M.height):te?s.renderbufferStorageMultisample(s.RENDERBUFFER,Ge(M),Te,M.width,M.height):s.renderbufferStorage(s.RENDERBUFFER,Te,M.width,M.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,Le,s.RENDERBUFFER,B)}else{let he=M.textures;for(let de=0;de<he.length;de++){let Te=he[de],Le=r.convert(Te.format,Te.colorSpace),pe=r.convert(Te.type),_e=x(Te.internalFormat,Le,pe,Te.normalized,Te.colorSpace);st(M)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Ge(M),_e,M.width,M.height):te?s.renderbufferStorageMultisample(s.RENDERBUFFER,Ge(M),_e,M.width,M.height):s.renderbufferStorage(s.RENDERBUFFER,_e,M.width,M.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function K(B,M,te){let he=M.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,B),!(M.depthTexture&&M.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let de=n.get(M.depthTexture);if(de.__renderTarget=M,(!de.__webglTexture||M.depthTexture.image.width!==M.width||M.depthTexture.image.height!==M.height)&&(M.depthTexture.image.width=M.width,M.depthTexture.image.height=M.height,M.depthTexture.needsUpdate=!0),he){if(de.__webglInit===void 0&&(de.__webglInit=!0,M.depthTexture.addEventListener("dispose",I)),de.__webglTexture===void 0){de.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,de.__webglTexture),He(s.TEXTURE_CUBE_MAP,M.depthTexture);let De=r.convert(M.depthTexture.format),We=r.convert(M.depthTexture.type),Pe;M.depthTexture.format===oi?Pe=s.DEPTH_COMPONENT24:M.depthTexture.format===ns&&(Pe=s.DEPTH24_STENCIL8);for(let Fe=0;Fe<6;Fe++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+Fe,0,Pe,M.width,M.height,0,De,We,null)}}else X(M.depthTexture,0);let Te=de.__webglTexture,Le=Ge(M),pe=he?s.TEXTURE_CUBE_MAP_POSITIVE_X+te:s.TEXTURE_2D,_e=M.depthTexture.format===ns?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(M.depthTexture.format===oi)st(M)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,_e,pe,Te,0,Le):s.framebufferTexture2D(s.FRAMEBUFFER,_e,pe,Te,0);else if(M.depthTexture.format===ns)st(M)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,_e,pe,Te,0,Le):s.framebufferTexture2D(s.FRAMEBUFFER,_e,pe,Te,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function Q(B){let M=n.get(B),te=B.isWebGLCubeRenderTarget===!0;if(M.__boundDepthTexture!==B.depthTexture){let he=B.depthTexture;if(M.__depthDisposeCallback&&M.__depthDisposeCallback(),he){let de=()=>{delete M.__boundDepthTexture,delete M.__depthDisposeCallback,he.removeEventListener("dispose",de)};he.addEventListener("dispose",de),M.__depthDisposeCallback=de}M.__boundDepthTexture=he}if(B.depthTexture&&!M.__autoAllocateDepthBuffer)if(te)for(let he=0;he<6;he++)K(M.__webglFramebuffer[he],B,he);else{let he=B.texture.mipmaps;he&&he.length>0?K(M.__webglFramebuffer[0],B,0):K(M.__webglFramebuffer,B,0)}else if(te){M.__webglDepthbuffer=[];for(let he=0;he<6;he++)if(t.bindFramebuffer(s.FRAMEBUFFER,M.__webglFramebuffer[he]),M.__webglDepthbuffer[he]===void 0)M.__webglDepthbuffer[he]=s.createRenderbuffer(),H(M.__webglDepthbuffer[he],B,!1);else{let de=B.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Te=M.__webglDepthbuffer[he];s.bindRenderbuffer(s.RENDERBUFFER,Te),s.framebufferRenderbuffer(s.FRAMEBUFFER,de,s.RENDERBUFFER,Te)}}else{let he=B.texture.mipmaps;if(he&&he.length>0?t.bindFramebuffer(s.FRAMEBUFFER,M.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,M.__webglFramebuffer),M.__webglDepthbuffer===void 0)M.__webglDepthbuffer=s.createRenderbuffer(),H(M.__webglDepthbuffer,B,!1);else{let de=B.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Te=M.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,Te),s.framebufferRenderbuffer(s.FRAMEBUFFER,de,s.RENDERBUFFER,Te)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function ee(B,M,te){let he=n.get(B);M!==void 0&&xe(he.__webglFramebuffer,B,B.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),te!==void 0&&Q(B)}function ae(B){let M=B.texture,te=n.get(B),he=n.get(M);B.addEventListener("dispose",_);let de=B.textures,Te=B.isWebGLCubeRenderTarget===!0,Le=de.length>1;if(Le||(he.__webglTexture===void 0&&(he.__webglTexture=s.createTexture()),he.__version=M.version,o.memory.textures++),Te){te.__webglFramebuffer=[];for(let pe=0;pe<6;pe++)if(M.mipmaps&&M.mipmaps.length>0){te.__webglFramebuffer[pe]=[];for(let _e=0;_e<M.mipmaps.length;_e++)te.__webglFramebuffer[pe][_e]=s.createFramebuffer()}else te.__webglFramebuffer[pe]=s.createFramebuffer()}else{if(M.mipmaps&&M.mipmaps.length>0){te.__webglFramebuffer=[];for(let pe=0;pe<M.mipmaps.length;pe++)te.__webglFramebuffer[pe]=s.createFramebuffer()}else te.__webglFramebuffer=s.createFramebuffer();if(Le)for(let pe=0,_e=de.length;pe<_e;pe++){let De=n.get(de[pe]);De.__webglTexture===void 0&&(De.__webglTexture=s.createTexture(),o.memory.textures++)}if(B.samples>0&&st(B)===!1){te.__webglMultisampledFramebuffer=s.createFramebuffer(),te.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,te.__webglMultisampledFramebuffer);for(let pe=0;pe<de.length;pe++){let _e=de[pe];te.__webglColorRenderbuffer[pe]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,te.__webglColorRenderbuffer[pe]);let De=r.convert(_e.format,_e.colorSpace),We=r.convert(_e.type),Pe=x(_e.internalFormat,De,We,_e.normalized,_e.colorSpace,B.isXRRenderTarget===!0),Fe=Ge(B);s.renderbufferStorageMultisample(s.RENDERBUFFER,Fe,Pe,B.width,B.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+pe,s.RENDERBUFFER,te.__webglColorRenderbuffer[pe])}s.bindRenderbuffer(s.RENDERBUFFER,null),B.depthBuffer&&(te.__webglDepthRenderbuffer=s.createRenderbuffer(),H(te.__webglDepthRenderbuffer,B,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(Te){t.bindTexture(s.TEXTURE_CUBE_MAP,he.__webglTexture),He(s.TEXTURE_CUBE_MAP,M);for(let pe=0;pe<6;pe++)if(M.mipmaps&&M.mipmaps.length>0)for(let _e=0;_e<M.mipmaps.length;_e++)xe(te.__webglFramebuffer[pe][_e],B,M,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+pe,_e);else xe(te.__webglFramebuffer[pe],B,M,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+pe,0);p(M)&&y(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if(Le){for(let pe=0,_e=de.length;pe<_e;pe++){let De=de[pe],We=n.get(De),Pe=s.TEXTURE_2D;(B.isWebGL3DRenderTarget||B.isWebGLArrayRenderTarget)&&(Pe=B.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(Pe,We.__webglTexture),He(Pe,De),xe(te.__webglFramebuffer,B,De,s.COLOR_ATTACHMENT0+pe,Pe,0),p(De)&&y(Pe)}t.unbindTexture()}else{let pe=s.TEXTURE_2D;if((B.isWebGL3DRenderTarget||B.isWebGLArrayRenderTarget)&&(pe=B.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(pe,he.__webglTexture),He(pe,M),M.mipmaps&&M.mipmaps.length>0)for(let _e=0;_e<M.mipmaps.length;_e++)xe(te.__webglFramebuffer[_e],B,M,s.COLOR_ATTACHMENT0,pe,_e);else xe(te.__webglFramebuffer,B,M,s.COLOR_ATTACHMENT0,pe,0);p(M)&&y(pe),t.unbindTexture()}B.depthBuffer&&Q(B)}function ve(B){let M=B.textures;for(let te=0,he=M.length;te<he;te++){let de=M[te];if(p(de)){let Te=w(B),Le=n.get(de).__webglTexture;t.bindTexture(Te,Le),y(Te),t.unbindTexture()}}}let Ue=[],Se=[];function je(B){if(B.samples>0){if(st(B)===!1){let M=B.textures,te=B.width,he=B.height,de=s.COLOR_BUFFER_BIT,Te=B.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,Le=n.get(B),pe=M.length>1;if(pe)for(let De=0;De<M.length;De++)t.bindFramebuffer(s.FRAMEBUFFER,Le.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+De,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,Le.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+De,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,Le.__webglMultisampledFramebuffer);let _e=B.texture.mipmaps;_e&&_e.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Le.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Le.__webglFramebuffer);for(let De=0;De<M.length;De++){if(B.resolveDepthBuffer&&(B.depthBuffer&&(de|=s.DEPTH_BUFFER_BIT),B.stencilBuffer&&B.resolveStencilBuffer&&(de|=s.STENCIL_BUFFER_BIT)),pe){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,Le.__webglColorRenderbuffer[De]);let We=n.get(M[De]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,We,0)}s.blitFramebuffer(0,0,te,he,0,0,te,he,de,s.NEAREST),l===!0&&(Ue.length=0,Se.length=0,Ue.push(s.COLOR_ATTACHMENT0+De),B.depthBuffer&&B.resolveDepthBuffer===!1&&(Ue.push(Te),Se.push(Te),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Se)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,Ue))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),pe)for(let De=0;De<M.length;De++){t.bindFramebuffer(s.FRAMEBUFFER,Le.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+De,s.RENDERBUFFER,Le.__webglColorRenderbuffer[De]);let We=n.get(M[De]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,Le.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+De,s.TEXTURE_2D,We,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,Le.__webglMultisampledFramebuffer)}else if(B.depthBuffer&&B.resolveDepthBuffer===!1&&l){let M=B.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[M])}}}function Ge(B){return Math.min(i.maxSamples,B.samples)}function st(B){let M=n.get(B);return B.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&M.__useRenderToTexture!==!1}function j(B){let M=o.render.frame;h.get(B)!==M&&(h.set(B,M),B.update())}function tt(B,M){let te=B.colorSpace,he=B.format,de=B.type;return B.isCompressedTexture===!0||B.isVideoTexture===!0||te!==dn&&te!==Bn&&(lt.getTransfer(te)===yt?(he!==Pn||de!==yn)&&Ze("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):$e("WebGLTextures: Unsupported texture color space:",te)),M}function Je(B){return typeof HTMLImageElement<"u"&&B instanceof HTMLImageElement?(c.width=B.naturalWidth||B.width,c.height=B.naturalHeight||B.height):typeof VideoFrame<"u"&&B instanceof VideoFrame?(c.width=B.displayWidth,c.height=B.displayHeight):(c.width=B.width,c.height=B.height),c}this.allocateTextureUnit=F,this.resetTextureUnits=W,this.getTextureUnits=R,this.setTextureUnits=U,this.setTexture2D=X,this.setTexture2DArray=Z,this.setTexture3D=V,this.setTextureCube=ie,this.rebindTextures=ee,this.setupRenderTarget=ae,this.updateRenderTargetMipmap=ve,this.updateMultisampleRenderTarget=je,this.setupDepthRenderbuffer=Q,this.setupFrameBufferTexture=xe,this.useMultisampledRTT=st,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function Lv(s,e){function t(n,i=Bn){let r,o=lt.getTransfer(i);if(n===yn)return s.UNSIGNED_BYTE;if(n===yl)return s.UNSIGNED_SHORT_4_4_4_4;if(n===Ml)return s.UNSIGNED_SHORT_5_5_5_1;if(n===Mh)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===bh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===vh)return s.BYTE;if(n===yh)return s.SHORT;if(n===Ar)return s.UNSIGNED_SHORT;if(n===vl)return s.INT;if(n===$n)return s.UNSIGNED_INT;if(n===Cn)return s.FLOAT;if(n===Jt)return s.HALF_FLOAT;if(n===Sh)return s.ALPHA;if(n===wh)return s.RGB;if(n===Pn)return s.RGBA;if(n===oi)return s.DEPTH_COMPONENT;if(n===ns)return s.DEPTH_STENCIL;if(n===bl)return s.RED;if(n===Sl)return s.RED_INTEGER;if(n===is)return s.RG;if(n===wl)return s.RG_INTEGER;if(n===El)return s.RGBA_INTEGER;if(n===zo||n===ko||n===Ho||n===Vo)if(o===yt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===zo)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===ko)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===Ho)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===Vo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===zo)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===ko)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===Ho)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===Vo)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===Tl||n===Al||n===Rl||n===Cl)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===Tl)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===Al)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===Rl)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===Cl)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===Pl||n===Il||n===Ll||n===Dl||n===Nl||n===Go||n===Ul)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===Pl||n===Il)return o===yt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===Ll)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===Dl)return r.COMPRESSED_R11_EAC;if(n===Nl)return r.COMPRESSED_SIGNED_R11_EAC;if(n===Go)return r.COMPRESSED_RG11_EAC;if(n===Ul)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===Fl||n===Ol||n===Bl||n===zl||n===kl||n===Hl||n===Vl||n===Gl||n===Wl||n===Xl||n===ql||n===Yl||n===Zl||n===Kl)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===Fl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Ol)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===Bl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===zl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===kl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===Hl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===Vl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===Gl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===Wl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===Xl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===ql)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===Yl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===Zl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Kl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===jl||n===Jl||n===$l)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===jl)return o===yt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===Jl)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===$l)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===Ql||n===ec||n===Wo||n===tc)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===Ql)return r.COMPRESSED_RED_RGTC1_EXT;if(n===ec)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===Wo)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===tc)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Rr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var Dv=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,Nv=`
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

}`,qh=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new fo(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new pt({vertexShader:Dv,fragmentShader:Nv,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new Ke(new nn(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Yh=class extends Fn{constructor(e,t){super();let n=this,i=null,r=1,o=null,a="local-floor",l=1,c=null,h=null,u=null,d=null,f=null,g=null,v=typeof XRWebGLBinding<"u",m=new qh,p={},y=t.getContextAttributes(),w=null,x=null,A=[],T=[],I=new Re,_=null,D=new Zt;D.viewport=new xt;let E=new Zt;E.viewport=new xt;let b=[D,E],P=new ul,W=null,R=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(ce){let Me=A[ce];return Me===void 0&&(Me=new dr,A[ce]=Me),Me.getTargetRaySpace()},this.getControllerGrip=function(ce){let Me=A[ce];return Me===void 0&&(Me=new dr,A[ce]=Me),Me.getGripSpace()},this.getHand=function(ce){let Me=A[ce];return Me===void 0&&(Me=new dr,A[ce]=Me),Me.getHandSpace()};function U(ce){let Me=T.indexOf(ce.inputSource);if(Me===-1)return;let me=A[Me];me!==void 0&&(me.update(ce.inputSource,ce.frame,c||o),me.dispatchEvent({type:ce.type,data:ce.inputSource}))}function F(){i.removeEventListener("select",U),i.removeEventListener("selectstart",U),i.removeEventListener("selectend",U),i.removeEventListener("squeeze",U),i.removeEventListener("squeezestart",U),i.removeEventListener("squeezeend",U),i.removeEventListener("end",F),i.removeEventListener("inputsourceschange",S);for(let ce=0;ce<A.length;ce++){let Me=T[ce];Me!==null&&(T[ce]=null,A[ce].disconnect(Me))}W=null,R=null,m.reset();for(let ce in p)delete p[ce];e.setRenderTarget(w),f=null,d=null,u=null,i=null,x=null,He.stop(),n.isPresenting=!1,e.setPixelRatio(_),e.setSize(I.width,I.height,!1),n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(ce){r=ce,n.isPresenting===!0&&Ze("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(ce){a=ce,n.isPresenting===!0&&Ze("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||o},this.setReferenceSpace=function(ce){c=ce},this.getBaseLayer=function(){return d!==null?d:f},this.getBinding=function(){return u===null&&v&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return g},this.getSession=function(){return i},this.setSession=async function(ce){if(i=ce,i!==null){if(w=e.getRenderTarget(),i.addEventListener("select",U),i.addEventListener("selectstart",U),i.addEventListener("selectend",U),i.addEventListener("squeeze",U),i.addEventListener("squeezestart",U),i.addEventListener("squeezeend",U),i.addEventListener("end",F),i.addEventListener("inputsourceschange",S),y.xrCompatible!==!0&&await t.makeXRCompatible(),_=e.getPixelRatio(),e.getSize(I),v&&"createProjectionLayer"in XRWebGLBinding.prototype){let me=null,Oe=null,ue=null;y.depth&&(ue=y.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,me=y.stencil?ns:oi,Oe=y.stencil?Rr:$n);let xe={colorFormat:t.RGBA8,depthFormat:ue,scaleFactor:r};u=this.getBinding(),d=u.createProjectionLayer(xe),i.updateRenderState({layers:[d]}),e.setPixelRatio(1),e.setSize(d.textureWidth,d.textureHeight,!1),x=new zt(d.textureWidth,d.textureHeight,{format:Pn,type:yn,depthTexture:new Ri(d.textureWidth,d.textureHeight,Oe,void 0,void 0,void 0,void 0,void 0,void 0,me),stencilBuffer:y.stencil,colorSpace:e.outputColorSpace,samples:y.antialias?4:0,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1})}else{let me={antialias:y.antialias,alpha:!0,depth:y.depth,stencil:y.stencil,framebufferScaleFactor:r};f=new XRWebGLLayer(i,t,me),i.updateRenderState({baseLayer:f}),e.setPixelRatio(1),e.setSize(f.framebufferWidth,f.framebufferHeight,!1),x=new zt(f.framebufferWidth,f.framebufferHeight,{format:Pn,type:yn,colorSpace:e.outputColorSpace,stencilBuffer:y.stencil,resolveDepthBuffer:f.ignoreDepthValues===!1,resolveStencilBuffer:f.ignoreDepthValues===!1})}x.isXRRenderTarget=!0,this.setFoveation(l),c=null,o=await i.requestReferenceSpace(a),He.setContext(i),He.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function S(ce){for(let Me=0;Me<ce.removed.length;Me++){let me=ce.removed[Me],Oe=T.indexOf(me);Oe>=0&&(T[Oe]=null,A[Oe].disconnect(me))}for(let Me=0;Me<ce.added.length;Me++){let me=ce.added[Me],Oe=T.indexOf(me);if(Oe===-1){for(let xe=0;xe<A.length;xe++)if(xe>=T.length){T.push(me),Oe=xe;break}else if(T[xe]===null){T[xe]=me,Oe=xe;break}if(Oe===-1)break}let ue=A[Oe];ue&&ue.connect(me)}}let X=new z,Z=new z;function V(ce,Me,me){X.setFromMatrixPosition(Me.matrixWorld),Z.setFromMatrixPosition(me.matrixWorld);let Oe=X.distanceTo(Z),ue=Me.projectionMatrix.elements,xe=me.projectionMatrix.elements,H=ue[14]/(ue[10]-1),K=ue[14]/(ue[10]+1),Q=(ue[9]+1)/ue[5],ee=(ue[9]-1)/ue[5],ae=(ue[8]-1)/ue[0],ve=(xe[8]+1)/xe[0],Ue=H*ae,Se=H*ve,je=Oe/(-ae+ve),Ge=je*-ae;if(Me.matrixWorld.decompose(ce.position,ce.quaternion,ce.scale),ce.translateX(Ge),ce.translateZ(je),ce.matrixWorld.compose(ce.position,ce.quaternion,ce.scale),ce.matrixWorldInverse.copy(ce.matrixWorld).invert(),ue[10]===-1)ce.projectionMatrix.copy(Me.projectionMatrix),ce.projectionMatrixInverse.copy(Me.projectionMatrixInverse);else{let st=H+je,j=K+je,tt=Ue-Ge,Je=Se+(Oe-Ge),B=Q*K/j*st,M=ee*K/j*st;ce.projectionMatrix.makePerspective(tt,Je,B,M,st,j),ce.projectionMatrixInverse.copy(ce.projectionMatrix).invert()}}function ie(ce,Me){Me===null?ce.matrixWorld.copy(ce.matrix):ce.matrixWorld.multiplyMatrices(Me.matrixWorld,ce.matrix),ce.matrixWorldInverse.copy(ce.matrixWorld).invert()}this.updateCamera=function(ce){if(i===null)return;let Me=ce.near,me=ce.far;m.texture!==null&&(m.depthNear>0&&(Me=m.depthNear),m.depthFar>0&&(me=m.depthFar)),P.near=E.near=D.near=Me,P.far=E.far=D.far=me,(W!==P.near||R!==P.far)&&(i.updateRenderState({depthNear:P.near,depthFar:P.far}),W=P.near,R=P.far),P.layers.mask=ce.layers.mask|6,D.layers.mask=P.layers.mask&-5,E.layers.mask=P.layers.mask&-3;let Oe=ce.parent,ue=P.cameras;ie(P,Oe);for(let xe=0;xe<ue.length;xe++)ie(ue[xe],Oe);ue.length===2?V(P,D,E):P.projectionMatrix.copy(D.projectionMatrix),fe(ce,P,Oe)};function fe(ce,Me,me){me===null?ce.matrix.copy(Me.matrixWorld):(ce.matrix.copy(me.matrixWorld),ce.matrix.invert(),ce.matrix.multiply(Me.matrixWorld)),ce.matrix.decompose(ce.position,ce.quaternion,ce.scale),ce.updateMatrixWorld(!0),ce.projectionMatrix.copy(Me.projectionMatrix),ce.projectionMatrixInverse.copy(Me.projectionMatrixInverse),ce.isPerspectiveCamera&&(ce.fov=ws*2*Math.atan(1/ce.projectionMatrix.elements[5]),ce.zoom=1)}this.getCamera=function(){return P},this.getFoveation=function(){if(!(d===null&&f===null))return l},this.setFoveation=function(ce){l=ce,d!==null&&(d.fixedFoveation=ce),f!==null&&f.fixedFoveation!==void 0&&(f.fixedFoveation=ce)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(P)},this.getCameraTexture=function(ce){return p[ce]};let Ce=null;function ze(ce,Me){if(h=Me.getViewerPose(c||o),g=Me,h!==null){let me=h.views;f!==null&&(e.setRenderTargetFramebuffer(x,f.framebuffer),e.setRenderTarget(x));let Oe=!1;me.length!==P.cameras.length&&(P.cameras.length=0,Oe=!0);for(let K=0;K<me.length;K++){let Q=me[K],ee=null;if(f!==null)ee=f.getViewport(Q);else{let ve=u.getViewSubImage(d,Q);ee=ve.viewport,K===0&&(e.setRenderTargetTextures(x,ve.colorTexture,ve.depthStencilTexture),e.setRenderTarget(x))}let ae=b[K];ae===void 0&&(ae=new Zt,ae.layers.enable(K),ae.viewport=new xt,b[K]=ae),ae.matrix.fromArray(Q.transform.matrix),ae.matrix.decompose(ae.position,ae.quaternion,ae.scale),ae.projectionMatrix.fromArray(Q.projectionMatrix),ae.projectionMatrixInverse.copy(ae.projectionMatrix).invert(),ae.viewport.set(ee.x,ee.y,ee.width,ee.height),K===0&&(P.matrix.copy(ae.matrix),P.matrix.decompose(P.position,P.quaternion,P.scale)),Oe===!0&&P.cameras.push(ae)}let ue=i.enabledFeatures;if(ue&&ue.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&v){u=n.getBinding();let K=u.getDepthInformation(me[0]);K&&K.isValid&&K.texture&&m.init(K,i.renderState)}if(ue&&ue.includes("camera-access")&&v){e.state.unbindTexture(),u=n.getBinding();for(let K=0;K<me.length;K++){let Q=me[K].camera;if(Q){let ee=p[Q];ee||(ee=new fo,p[Q]=ee);let ae=u.getCameraImage(Q);ee.sourceTexture=ae}}}}for(let me=0;me<A.length;me++){let Oe=T[me],ue=A[me];Oe!==null&&ue!==void 0&&ue.update(Oe,Me,c||o)}Ce&&Ce(ce,Me),Me.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:Me}),g=null}let He=new Bf;He.setAnimationLoop(ze),this.setAnimationLoop=function(ce){Ce=ce},this.dispose=function(){}}},Uv=new it,Wf=new ot;Wf.set(-1,0,0,0,1,0,0,0,1);function Fv(s,e){function t(m,p){m.matrixAutoUpdate===!0&&m.updateMatrix(),p.value.copy(m.matrix)}function n(m,p){p.color.getRGB(m.fogColor.value,Ch(s)),p.isFog?(m.fogNear.value=p.near,m.fogFar.value=p.far):p.isFogExp2&&(m.fogDensity.value=p.density)}function i(m,p,y,w,x){p.isNodeMaterial?p.uniformsNeedUpdate=!1:p.isMeshBasicMaterial?r(m,p):p.isMeshLambertMaterial?(r(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshToonMaterial?(r(m,p),u(m,p)):p.isMeshPhongMaterial?(r(m,p),h(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshStandardMaterial?(r(m,p),d(m,p),p.isMeshPhysicalMaterial&&f(m,p,x)):p.isMeshMatcapMaterial?(r(m,p),g(m,p)):p.isMeshDepthMaterial?r(m,p):p.isMeshDistanceMaterial?(r(m,p),v(m,p)):p.isMeshNormalMaterial?r(m,p):p.isLineBasicMaterial?(o(m,p),p.isLineDashedMaterial&&a(m,p)):p.isPointsMaterial?l(m,p,y,w):p.isSpriteMaterial?c(m,p):p.isShadowMaterial?(m.color.value.copy(p.color),m.opacity.value=p.opacity):p.isShaderMaterial&&(p.uniformsNeedUpdate=!1)}function r(m,p){m.opacity.value=p.opacity,p.color&&m.diffuse.value.copy(p.color),p.emissive&&m.emissive.value.copy(p.emissive).multiplyScalar(p.emissiveIntensity),p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.bumpMap&&(m.bumpMap.value=p.bumpMap,t(p.bumpMap,m.bumpMapTransform),m.bumpScale.value=p.bumpScale,p.side===jt&&(m.bumpScale.value*=-1)),p.normalMap&&(m.normalMap.value=p.normalMap,t(p.normalMap,m.normalMapTransform),m.normalScale.value.copy(p.normalScale),p.side===jt&&m.normalScale.value.negate()),p.displacementMap&&(m.displacementMap.value=p.displacementMap,t(p.displacementMap,m.displacementMapTransform),m.displacementScale.value=p.displacementScale,m.displacementBias.value=p.displacementBias),p.emissiveMap&&(m.emissiveMap.value=p.emissiveMap,t(p.emissiveMap,m.emissiveMapTransform)),p.specularMap&&(m.specularMap.value=p.specularMap,t(p.specularMap,m.specularMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest);let y=e.get(p),w=y.envMap,x=y.envMapRotation;w&&(m.envMap.value=w,m.envMapRotation.value.setFromMatrix4(Uv.makeRotationFromEuler(x)).transpose(),w.isCubeTexture&&w.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(Wf),m.reflectivity.value=p.reflectivity,m.ior.value=p.ior,m.refractionRatio.value=p.refractionRatio),p.lightMap&&(m.lightMap.value=p.lightMap,m.lightMapIntensity.value=p.lightMapIntensity,t(p.lightMap,m.lightMapTransform)),p.aoMap&&(m.aoMap.value=p.aoMap,m.aoMapIntensity.value=p.aoMapIntensity,t(p.aoMap,m.aoMapTransform))}function o(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform))}function a(m,p){m.dashSize.value=p.dashSize,m.totalSize.value=p.dashSize+p.gapSize,m.scale.value=p.scale}function l(m,p,y,w){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.size.value=p.size*y,m.scale.value=w*.5,p.map&&(m.map.value=p.map,t(p.map,m.uvTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function c(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.rotation.value=p.rotation,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function h(m,p){m.specular.value.copy(p.specular),m.shininess.value=Math.max(p.shininess,1e-4)}function u(m,p){p.gradientMap&&(m.gradientMap.value=p.gradientMap)}function d(m,p){m.metalness.value=p.metalness,p.metalnessMap&&(m.metalnessMap.value=p.metalnessMap,t(p.metalnessMap,m.metalnessMapTransform)),m.roughness.value=p.roughness,p.roughnessMap&&(m.roughnessMap.value=p.roughnessMap,t(p.roughnessMap,m.roughnessMapTransform)),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)}function f(m,p,y){m.ior.value=p.ior,p.sheen>0&&(m.sheenColor.value.copy(p.sheenColor).multiplyScalar(p.sheen),m.sheenRoughness.value=p.sheenRoughness,p.sheenColorMap&&(m.sheenColorMap.value=p.sheenColorMap,t(p.sheenColorMap,m.sheenColorMapTransform)),p.sheenRoughnessMap&&(m.sheenRoughnessMap.value=p.sheenRoughnessMap,t(p.sheenRoughnessMap,m.sheenRoughnessMapTransform))),p.clearcoat>0&&(m.clearcoat.value=p.clearcoat,m.clearcoatRoughness.value=p.clearcoatRoughness,p.clearcoatMap&&(m.clearcoatMap.value=p.clearcoatMap,t(p.clearcoatMap,m.clearcoatMapTransform)),p.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=p.clearcoatRoughnessMap,t(p.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),p.clearcoatNormalMap&&(m.clearcoatNormalMap.value=p.clearcoatNormalMap,t(p.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(p.clearcoatNormalScale),p.side===jt&&m.clearcoatNormalScale.value.negate())),p.dispersion>0&&(m.dispersion.value=p.dispersion),p.iridescence>0&&(m.iridescence.value=p.iridescence,m.iridescenceIOR.value=p.iridescenceIOR,m.iridescenceThicknessMinimum.value=p.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=p.iridescenceThicknessRange[1],p.iridescenceMap&&(m.iridescenceMap.value=p.iridescenceMap,t(p.iridescenceMap,m.iridescenceMapTransform)),p.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=p.iridescenceThicknessMap,t(p.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),p.transmission>0&&(m.transmission.value=p.transmission,m.transmissionSamplerMap.value=y.texture,m.transmissionSamplerSize.value.set(y.width,y.height),p.transmissionMap&&(m.transmissionMap.value=p.transmissionMap,t(p.transmissionMap,m.transmissionMapTransform)),m.thickness.value=p.thickness,p.thicknessMap&&(m.thicknessMap.value=p.thicknessMap,t(p.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=p.attenuationDistance,m.attenuationColor.value.copy(p.attenuationColor)),p.anisotropy>0&&(m.anisotropyVector.value.set(p.anisotropy*Math.cos(p.anisotropyRotation),p.anisotropy*Math.sin(p.anisotropyRotation)),p.anisotropyMap&&(m.anisotropyMap.value=p.anisotropyMap,t(p.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=p.specularIntensity,m.specularColor.value.copy(p.specularColor),p.specularColorMap&&(m.specularColorMap.value=p.specularColorMap,t(p.specularColorMap,m.specularColorMapTransform)),p.specularIntensityMap&&(m.specularIntensityMap.value=p.specularIntensityMap,t(p.specularIntensityMap,m.specularIntensityMapTransform))}function g(m,p){p.matcap&&(m.matcap.value=p.matcap)}function v(m,p){let y=e.get(p).light;m.referencePosition.value.setFromMatrixPosition(y.matrixWorld),m.nearDistance.value=y.shadow.camera.near,m.farDistance.value=y.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function Ov(s,e,t,n){let i={},r={},o=[],a=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(x,A){let T=A.program;n.uniformBlockBinding(x,T)}function c(x,A){let T=i[x.id];T===void 0&&(m(x),T=h(x),i[x.id]=T,x.addEventListener("dispose",y));let I=A.program;n.updateUBOMapping(x,I);let _=e.render.frame;r[x.id]!==_&&(d(x),r[x.id]=_)}function h(x){let A=u();x.__bindingPointIndex=A;let T=s.createBuffer(),I=x.__size,_=x.usage;return s.bindBuffer(s.UNIFORM_BUFFER,T),s.bufferData(s.UNIFORM_BUFFER,I,_),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,A,T),T}function u(){for(let x=0;x<a;x++)if(o.indexOf(x)===-1)return o.push(x),x;return $e("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function d(x){let A=i[x.id],T=x.uniforms,I=x.__cache;s.bindBuffer(s.UNIFORM_BUFFER,A);for(let _=0,D=T.length;_<D;_++){let E=T[_];if(Array.isArray(E))for(let b=0,P=E.length;b<P;b++)f(E[b],_,b,I);else f(E,_,0,I)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function f(x,A,T,I){if(v(x,A,T,I)===!0){let _=x.__offset,D=x.value;if(Array.isArray(D)){let E=0;for(let b=0;b<D.length;b++){let P=D[b],W=p(P);g(P,x.__data,E),typeof P!="number"&&typeof P!="boolean"&&!P.isMatrix3&&!ArrayBuffer.isView(P)&&(E+=W.storage/Float32Array.BYTES_PER_ELEMENT)}}else g(D,x.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,_,x.__data)}}function g(x,A,T){typeof x=="number"||typeof x=="boolean"?A[0]=x:x.isMatrix3?(A[0]=x.elements[0],A[1]=x.elements[1],A[2]=x.elements[2],A[3]=0,A[4]=x.elements[3],A[5]=x.elements[4],A[6]=x.elements[5],A[7]=0,A[8]=x.elements[6],A[9]=x.elements[7],A[10]=x.elements[8],A[11]=0):ArrayBuffer.isView(x)?A.set(new x.constructor(x.buffer,x.byteOffset,A.length)):x.toArray(A,T)}function v(x,A,T,I){let _=x.value,D=A+"_"+T;if(I[D]===void 0)return typeof _=="number"||typeof _=="boolean"?I[D]=_:ArrayBuffer.isView(_)?I[D]=_.slice():I[D]=_.clone(),!0;{let E=I[D];if(typeof _=="number"||typeof _=="boolean"){if(E!==_)return I[D]=_,!0}else{if(ArrayBuffer.isView(_))return!0;if(E.equals(_)===!1)return E.copy(_),!0}}return!1}function m(x){let A=x.uniforms,T=0,I=16;for(let D=0,E=A.length;D<E;D++){let b=Array.isArray(A[D])?A[D]:[A[D]];for(let P=0,W=b.length;P<W;P++){let R=b[P],U=Array.isArray(R.value)?R.value:[R.value];for(let F=0,S=U.length;F<S;F++){let X=U[F],Z=p(X),V=T%I,ie=V%Z.boundary,fe=V+ie;T+=ie,fe!==0&&I-fe<Z.storage&&(T+=I-fe),R.__data=new Float32Array(Z.storage/Float32Array.BYTES_PER_ELEMENT),R.__offset=T,T+=Z.storage}}}let _=T%I;return _>0&&(T+=I-_),x.__size=T,x.__cache={},this}function p(x){let A={boundary:0,storage:0};return typeof x=="number"||typeof x=="boolean"?(A.boundary=4,A.storage=4):x.isVector2?(A.boundary=8,A.storage=8):x.isVector3||x.isColor?(A.boundary=16,A.storage=12):x.isVector4?(A.boundary=16,A.storage=16):x.isMatrix3?(A.boundary=48,A.storage=48):x.isMatrix4?(A.boundary=64,A.storage=64):x.isTexture?Ze("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(x)?(A.boundary=16,A.storage=x.byteLength):Ze("WebGLRenderer: Unsupported uniform value type.",x),A}function y(x){let A=x.target;A.removeEventListener("dispose",y);let T=o.indexOf(A.__bindingPointIndex);o.splice(T,1),s.deleteBuffer(i[A.id]),delete i[A.id],delete r[A.id]}function w(){for(let x in i)s.deleteBuffer(i[x]);o=[],i={},r={}}return{bind:l,update:c,dispose:w}}var Bv=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),pi=null;function zv(){return pi===null&&(pi=new Zi(Bv,16,16,is,Jt),pi.name="DFG_LUT",pi.minFilter=Ft,pi.magFilter=Ft,pi.wrapS=Un,pi.wrapT=Un,pi.generateMipmaps=!1,pi.needsUpdate=!0),pi}var cc=class{constructor(e={}){let{canvas:t=hf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:d=!1,outputBufferType:f=yn}=e;this.isWebGLRenderer=!0;let g;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");g=n.getContextAttributes().alpha}else g=o;let v=f,m=new Set([El,wl,Sl]),p=new Set([yn,$n,Ar,Rr,yl,Ml]),y=new Uint32Array(4),w=new Int32Array(4),x=new z,A=null,T=null,I=[],_=[],D=null;this.domElement=t,this.debug={checkShaderErrors:!0,onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=jn,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let E=this,b=!1,P=null,W=null,R=null,U=null;this._outputColorSpace=Ut;let F=0,S=0,X=null,Z=-1,V=null,ie=new xt,fe=new xt,Ce=null,ze=new be(0),He=0,ce=t.width,Me=t.height,me=1,Oe=null,ue=null,xe=new xt(0,0,ce,Me),H=new xt(0,0,ce,Me),K=!1,Q=new gr,ee=!1,ae=!1,ve=new it,Ue=new z,Se=new xt,je={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},Ge=!1;function st(){return X===null?me:1}let j=n;function tt(C,J){return t.getContext(C,J)}try{let C={alpha:!0,depth:i,stencil:r,antialias:a,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"185"}`),t.addEventListener("webglcontextlost",at,!1),t.addEventListener("webglcontextrestored",Qe,!1),t.addEventListener("webglcontextcreationerror",Lt,!1),j===null){let J="webgl2";if(j=tt(J,C),j===null)throw tt(J)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}}catch(C){throw $e("WebGLRenderer: "+C.message),C}let Je,B,M,te,he,de,Te,Le,pe,_e,De,We,Pe,Fe,O,q,Y,N,L,k,G,le,se;function ge(){Je=new qx(j),Je.init(),G=new Lv(j,Je),B=new Bx(j,Je,e,G),M=new Pv(j,Je),B.reversedDepthBuffer&&d&&M.buffers.depth.setReversed(!0),W=j.createFramebuffer(),R=j.createFramebuffer(),U=j.createFramebuffer(),te=new Kx(j),he=new gv,de=new Iv(j,Je,M,he,B,G,te),Te=new Xx(E),Le=new Qm(j),le=new Fx(j,Le),pe=new Yx(j,Le,te,le),_e=new Jx(j,pe,Le,le,te),N=new jx(j,B,de),O=new zx(he),De=new mv(E,Te,Je,B,le,O),We=new Fv(E,he),Pe=new _v,Fe=new wv(Je),Y=new Ux(E,Te,M,_e,g,l),q=new Cv(E,_e,B),se=new Ov(j,te,B,M),L=new Ox(j,Je,te),k=new Zx(j,Je,te),te.programs=De.programs,E.capabilities=B,E.extensions=Je,E.properties=he,E.renderLists=Pe,E.shadowMap=q,E.state=M,E.info=te}ge(),v!==yn&&(D=new Qx(v,t.width,t.height,a,i,r));let Ae=new Yh(E,j);this.xr=Ae,this.getContext=function(){return j},this.getContextAttributes=function(){return j.getContextAttributes()},this.forceContextLoss=function(){let C=Je.get("WEBGL_lose_context");C&&C.loseContext()},this.forceContextRestore=function(){let C=Je.get("WEBGL_lose_context");C&&C.restoreContext()},this.getPixelRatio=function(){return me},this.setPixelRatio=function(C){C!==void 0&&(me=C,this.setSize(ce,Me,!1))},this.getSize=function(C){return C.set(ce,Me)},this.setSize=function(C,J,oe=!0){if(Ae.isPresenting){Ze("WebGLRenderer: Can't change size while VR device is presenting.");return}ce=C,Me=J,t.width=Math.floor(C*me),t.height=Math.floor(J*me),oe===!0&&(t.style.width=C+"px",t.style.height=J+"px"),D!==null&&D.setSize(t.width,t.height),this.setViewport(0,0,C,J)},this.getDrawingBufferSize=function(C){return C.set(ce*me,Me*me).floor()},this.setDrawingBufferSize=function(C,J,oe){ce=C,Me=J,me=oe,t.width=Math.floor(C*oe),t.height=Math.floor(J*oe),this.setViewport(0,0,C,J)},this.setEffects=function(C){if(v===yn){$e("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(C){for(let J=0;J<C.length;J++)if(C[J].isOutputPass===!0){Ze("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}D.setEffects(C||[])},this.getCurrentViewport=function(C){return C.copy(ie)},this.getViewport=function(C){return C.copy(xe)},this.setViewport=function(C,J,oe,ne){C.isVector4?xe.set(C.x,C.y,C.z,C.w):xe.set(C,J,oe,ne),M.viewport(ie.copy(xe).multiplyScalar(me).round())},this.getScissor=function(C){return C.copy(H)},this.setScissor=function(C,J,oe,ne){C.isVector4?H.set(C.x,C.y,C.z,C.w):H.set(C,J,oe,ne),M.scissor(fe.copy(H).multiplyScalar(me).round())},this.getScissorTest=function(){return K},this.setScissorTest=function(C){M.setScissorTest(K=C)},this.setOpaqueSort=function(C){Oe=C},this.setTransparentSort=function(C){ue=C},this.getClearColor=function(C){return C.copy(Y.getClearColor())},this.setClearColor=function(){Y.setClearColor(...arguments)},this.getClearAlpha=function(){return Y.getClearAlpha()},this.setClearAlpha=function(){Y.setClearAlpha(...arguments)},this.clear=function(C=!0,J=!0,oe=!0){let ne=0;if(C){let re=!1;if(X!==null){let Ee=X.texture.format;re=m.has(Ee)}if(re){let Ee=X.texture.type,we=p.has(Ee),ye=Y.getClearColor(),Ne=Y.getClearAlpha(),Ie=ye.r,Xe=ye.g,nt=ye.b;we?(y[0]=Ie,y[1]=Xe,y[2]=nt,y[3]=Ne,j.clearBufferuiv(j.COLOR,0,y)):(w[0]=Ie,w[1]=Xe,w[2]=nt,w[3]=Ne,j.clearBufferiv(j.COLOR,0,w))}else ne|=j.COLOR_BUFFER_BIT}J&&(ne|=j.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),oe&&(ne|=j.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),ne!==0&&j.clear(ne)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(C){C.setRenderer(this),P=C},this.dispose=function(){t.removeEventListener("webglcontextlost",at,!1),t.removeEventListener("webglcontextrestored",Qe,!1),t.removeEventListener("webglcontextcreationerror",Lt,!1),Y.dispose(),Pe.dispose(),Fe.dispose(),he.dispose(),Te.dispose(),_e.dispose(),le.dispose(),se.dispose(),De.dispose(),Ae.dispose(),Ae.removeEventListener("sessionstart",na),Ae.removeEventListener("sessionend",Fi),ti.stop()};function at(C){C.preventDefault(),io("WebGLRenderer: Context Lost."),b=!0}function Qe(){io("WebGLRenderer: Context Restored."),b=!1;let C=te.autoReset,J=q.enabled,oe=q.autoUpdate,ne=q.needsUpdate,re=q.type;ge(),te.autoReset=C,q.enabled=J,q.autoUpdate=oe,q.needsUpdate=ne,q.type=re}function Lt(C){$e("WebGLRenderer: A WebGL context could not be created. Reason: ",C.statusMessage)}function rt(C){let J=C.target;J.removeEventListener("dispose",rt),Nt(J)}function Nt(C){Ot(C),he.remove(C)}function Ot(C){let J=he.get(C).programs;J!==void 0&&(J.forEach(function(oe){De.releaseProgram(oe)}),C.isShaderMaterial&&De.releaseShaderCache(C))}this.renderBufferDirect=function(C,J,oe,ne,re,Ee){J===null&&(J=je);let we=re.isMesh&&re.matrixWorld.determinantAffine()<0,ye=$(C,J,oe,ne,re);M.setMaterial(ne,we);let Ne=oe.index,Ie=1;if(ne.wireframe===!0){if(Ne=pe.getWireframeAttribute(oe),Ne===void 0)return;Ie=2}let Xe=oe.drawRange,nt=oe.attributes.position,qe=Xe.start*Ie,Et=(Xe.start+Xe.count)*Ie;Ee!==null&&(qe=Math.max(qe,Ee.start*Ie),Et=Math.min(Et,(Ee.start+Ee.count)*Ie)),Ne!==null?(qe=Math.max(qe,0),Et=Math.min(Et,Ne.count)):nt!=null&&(qe=Math.max(qe,0),Et=Math.min(Et,nt.count));let Ht=Et-qe;if(Ht<0||Ht===1/0)return;le.setup(re,ne,ye,oe,Ne);let Bt,Ct=L;if(Ne!==null&&(Bt=Le.get(Ne),Ct=k,Ct.setIndex(Bt)),re.isMesh)ne.wireframe===!0?(M.setLineWidth(ne.wireframeLinewidth*st()),Ct.setMode(j.LINES)):Ct.setMode(j.TRIANGLES);else if(re.isLine){let sn=ne.linewidth;sn===void 0&&(sn=1),M.setLineWidth(sn*st()),re.isLineSegments?Ct.setMode(j.LINES):re.isLineLoop?Ct.setMode(j.LINE_LOOP):Ct.setMode(j.LINE_STRIP)}else re.isPoints?Ct.setMode(j.POINTS):re.isSprite&&Ct.setMode(j.TRIANGLES);if(re.isBatchedMesh)if(Je.get("WEBGL_multi_draw"))Ct.renderMultiDraw(re._multiDrawStarts,re._multiDrawCounts,re._multiDrawCount);else{let sn=re._multiDrawStarts,Ve=re._multiDrawCounts,Sn=re._multiDrawCount,vt=Ne?Le.get(Ne).bytesPerElement:1,Ln=he.get(ne).currentProgram.getUniforms();for(let ni=0;ni<Sn;ni++)Ln.setValue(j,"_gl_DrawID",ni),Ct.render(sn[ni]/vt,Ve[ni])}else if(re.isInstancedMesh)Ct.renderInstances(qe,Ht,re.count);else if(oe.isInstancedBufferGeometry){let sn=oe._maxInstanceCount!==void 0?oe._maxInstanceCount:1/0,Ve=Math.min(oe.instanceCount,sn);Ct.renderInstances(qe,Ht,Ve)}else Ct.render(qe,Ht)};function Dt(C,J,oe){C.transparent===!0&&C.side===At&&C.forceSinglePass===!1?(C.side=jt,C.needsUpdate=!0,hs(C,J,oe),C.side=Tn,C.needsUpdate=!0,hs(C,J,oe),C.side=At):hs(C,J,oe)}this.compile=function(C,J,oe=null){oe===null&&(oe=C),T=Fe.get(oe),T.init(J),_.push(T),oe.traverseVisible(function(re){re.isLight&&re.layers.test(J.layers)&&(T.pushLight(re),re.castShadow&&T.pushShadow(re))}),C!==oe&&C.traverseVisible(function(re){re.isLight&&re.layers.test(J.layers)&&(T.pushLight(re),re.castShadow&&T.pushShadow(re))}),T.setupLights();let ne=new Set;return C.traverse(function(re){if(!(re.isMesh||re.isPoints||re.isLine||re.isSprite))return;let Ee=re.material;if(Ee)if(Array.isArray(Ee))for(let we=0;we<Ee.length;we++){let ye=Ee[we];Dt(ye,oe,re),ne.add(ye)}else Dt(Ee,oe,re),ne.add(Ee)}),T=_.pop(),ne},this.compileAsync=function(C,J,oe=null){let ne=this.compile(C,J,oe);return new Promise(re=>{function Ee(){if(ne.forEach(function(we){he.get(we).currentProgram.isReady()&&ne.delete(we)}),ne.size===0){re(C);return}setTimeout(Ee,10)}Je.get("KHR_parallel_shader_compile")!==null?Ee():setTimeout(Ee,10)})};let vi=null;function cs(C){vi&&vi(C)}function na(){ti.stop()}function Fi(){ti.start()}let ti=new Bf;ti.setAnimationLoop(cs),typeof self<"u"&&ti.setContext(self),this.setAnimationLoop=function(C){vi=C,Ae.setAnimationLoop(C),C===null?ti.stop():ti.start()},Ae.addEventListener("sessionstart",na),Ae.addEventListener("sessionend",Fi),this.render=function(C,J){if(J!==void 0&&J.isCamera!==!0){$e("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(b===!0)return;P!==null&&P.renderStart(C,J);let oe=Ae.enabled===!0&&Ae.isPresenting===!0,ne=D!==null&&(X===null||oe)&&D.begin(E,X);if(C.matrixWorldAutoUpdate===!0&&C.updateMatrixWorld(),J.parent===null&&J.matrixWorldAutoUpdate===!0&&J.updateMatrixWorld(),Ae.enabled===!0&&Ae.isPresenting===!0&&(D===null||D.isCompositing()===!1)&&(Ae.cameraAutoUpdate===!0&&Ae.updateCamera(J),J=Ae.getCamera()),C.isScene===!0&&C.onBeforeRender(E,C,J,X),T=Fe.get(C,_.length),T.init(J),T.state.textureUnits=de.getTextureUnits(),_.push(T),ve.multiplyMatrices(J.projectionMatrix,J.matrixWorldInverse),Q.setFromProjectionMatrix(ve,qn,J.reversedDepth),ae=this.localClippingEnabled,ee=O.init(this.clippingPlanes,ae),A=Pe.get(C,I.length),A.init(),I.push(A),Ae.enabled===!0&&Ae.isPresenting===!0){let we=E.xr.getDepthSensingMesh();we!==null&&Hs(we,J,-1/0,E.sortObjects)}Hs(C,J,0,E.sortObjects),A.finish(),E.sortObjects===!0&&A.sort(Oe,ue,J.reversedDepth),Ge=Ae.enabled===!1||Ae.isPresenting===!1||Ae.hasDepthSensing()===!1,Ge&&Y.addToRenderList(A,C),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),ee===!0&&O.beginShadows();let re=T.state.shadowsArray;if(q.render(re,C,J),ee===!0&&O.endShadows(),(ne&&D.hasRenderPass())===!1){let we=A.opaque,ye=A.transmissive;if(T.setupLights(),J.isArrayCamera){let Ne=J.cameras;if(ye.length>0)for(let Ie=0,Xe=Ne.length;Ie<Xe;Ie++){let nt=Ne[Ie];ia(we,ye,C,nt)}Ge&&Y.render(C);for(let Ie=0,Xe=Ne.length;Ie<Xe;Ie++){let nt=Ne[Ie];Hr(A,C,nt,nt.viewport)}}else ye.length>0&&ia(we,ye,C,J),Ge&&Y.render(C),Hr(A,C,J)}X!==null&&S===0&&(de.updateMultisampleRenderTarget(X),de.updateRenderTargetMipmap(X)),ne&&D.end(E),C.isScene===!0&&C.onAfterRender(E,C,J),le.resetDefaultState(),Z=-1,V=null,_.pop(),_.length>0?(T=_[_.length-1],de.setTextureUnits(T.state.textureUnits),ee===!0&&O.setGlobalState(E.clippingPlanes,T.state.camera)):T=null,I.pop(),I.length>0?A=I[I.length-1]:A=null,P!==null&&P.renderEnd()};function Hs(C,J,oe,ne){if(C.visible===!1)return;if(C.layers.test(J.layers)){if(C.isGroup)oe=C.renderOrder;else if(C.isLOD)C.autoUpdate===!0&&C.update(J);else if(C.isLightProbeGrid)T.pushLightProbeGrid(C);else if(C.isLight)T.pushLight(C),C.castShadow&&T.pushShadow(C);else if(C.isSprite){if(!C.frustumCulled||Q.intersectsSprite(C)){ne&&Se.setFromMatrixPosition(C.matrixWorld).applyMatrix4(ve);let we=_e.update(C),ye=C.material;ye.visible&&A.push(C,we,ye,oe,Se.z,null)}}else if((C.isMesh||C.isLine||C.isPoints)&&(!C.frustumCulled||Q.intersectsObject(C))){let we=_e.update(C),ye=C.material;if(ne&&(C.boundingSphere!==void 0?(C.boundingSphere===null&&C.computeBoundingSphere(),Se.copy(C.boundingSphere.center)):(we.boundingSphere===null&&we.computeBoundingSphere(),Se.copy(we.boundingSphere.center)),Se.applyMatrix4(C.matrixWorld).applyMatrix4(ve)),Array.isArray(ye)){let Ne=we.groups;for(let Ie=0,Xe=Ne.length;Ie<Xe;Ie++){let nt=Ne[Ie],qe=ye[nt.materialIndex];qe&&qe.visible&&A.push(C,we,qe,oe,Se.z,nt)}}else ye.visible&&A.push(C,we,ye,oe,Se.z,null)}}let Ee=C.children;for(let we=0,ye=Ee.length;we<ye;we++)Hs(Ee[we],J,oe,ne)}function Hr(C,J,oe,ne){let{opaque:re,transmissive:Ee,transparent:we}=C;T.setupLightsView(oe),ee===!0&&O.setGlobalState(E.clippingPlanes,oe),ne&&M.viewport(ie.copy(ne)),re.length>0&&Vs(re,J,oe),Ee.length>0&&Vs(Ee,J,oe),we.length>0&&Vs(we,J,oe),M.buffers.depth.setTest(!0),M.buffers.depth.setMask(!0),M.buffers.color.setMask(!0),M.setPolygonOffset(!1)}function ia(C,J,oe,ne){if((oe.isScene===!0?oe.overrideMaterial:null)!==null)return;if(T.state.transmissionRenderTarget[ne.id]===void 0){let qe=Je.has("EXT_color_buffer_half_float")||Je.has("EXT_color_buffer_float");T.state.transmissionRenderTarget[ne.id]=new zt(1,1,{generateMipmaps:!0,type:qe?Jt:yn,minFilter:Jn,samples:Math.max(4,B.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,colorSpace:lt.workingColorSpace})}let Ee=T.state.transmissionRenderTarget[ne.id],we=ne.viewport||ie;Ee.setSize(we.z*E.transmissionResolutionScale,we.w*E.transmissionResolutionScale);let ye=E.getRenderTarget(),Ne=E.getActiveCubeFace(),Ie=E.getActiveMipmapLevel();E.setRenderTarget(Ee),E.getClearColor(ze),He=E.getClearAlpha(),He<1&&E.setClearColor(16777215,.5),E.clear(),Ge&&Y.render(oe);let Xe=E.toneMapping;E.toneMapping=jn;let nt=ne.viewport;if(ne.viewport!==void 0&&(ne.viewport=void 0),T.setupLightsView(ne),ee===!0&&O.setGlobalState(E.clippingPlanes,ne),Vs(C,oe,ne),de.updateMultisampleRenderTarget(Ee),de.updateRenderTargetMipmap(Ee),Je.has("WEBGL_multisampled_render_to_texture")===!1){let qe=!1;for(let Et=0,Ht=J.length;Et<Ht;Et++){let Bt=J[Et],{object:Ct,geometry:sn,material:Ve,group:Sn}=Bt;if(Ve.side===At&&Ct.layers.test(ne.layers)){let vt=Ve.side;Ve.side=jt,Ve.needsUpdate=!0,Vr(Ct,oe,ne,sn,Ve,Sn),Ve.side=vt,Ve.needsUpdate=!0,qe=!0}}qe===!0&&(de.updateMultisampleRenderTarget(Ee),de.updateRenderTargetMipmap(Ee))}E.setRenderTarget(ye,Ne,Ie),E.setClearColor(ze,He),nt!==void 0&&(ne.viewport=nt),E.toneMapping=Xe}function Vs(C,J,oe){let ne=J.isScene===!0?J.overrideMaterial:null;for(let re=0,Ee=C.length;re<Ee;re++){let we=C[re],{object:ye,geometry:Ne,group:Ie}=we,Xe=we.material;Xe.allowOverride===!0&&ne!==null&&(Xe=ne),ye.layers.test(oe.layers)&&Vr(ye,J,oe,Ne,Xe,Ie)}}function Vr(C,J,oe,ne,re,Ee){C.onBeforeRender(E,J,oe,ne,re,Ee),C.modelViewMatrix.multiplyMatrices(oe.matrixWorldInverse,C.matrixWorld),C.normalMatrix.getNormalMatrix(C.modelViewMatrix),re.onBeforeRender(E,J,oe,ne,C,Ee),re.transparent===!0&&re.side===At&&re.forceSinglePass===!1?(re.side=jt,re.needsUpdate=!0,E.renderBufferDirect(oe,J,ne,re,C,Ee),re.side=Tn,re.needsUpdate=!0,E.renderBufferDirect(oe,J,ne,re,C,Ee),re.side=At):E.renderBufferDirect(oe,J,ne,re,C,Ee),C.onAfterRender(E,J,oe,ne,re,Ee)}function hs(C,J,oe){J.isScene!==!0&&(J=je);let ne=he.get(C),re=T.state.lights,Ee=T.state.shadowsArray,we=re.state.version,ye=De.getParameters(C,re.state,Ee,J,oe,T.state.lightProbeGridArray),Ne=De.getProgramCacheKey(ye),Ie=ne.programs;ne.environment=C.isMeshStandardMaterial||C.isMeshLambertMaterial||C.isMeshPhongMaterial?J.environment:null,ne.fog=J.fog;let Xe=C.isMeshStandardMaterial||C.isMeshLambertMaterial&&!C.envMap||C.isMeshPhongMaterial&&!C.envMap;ne.envMap=Te.get(C.envMap||ne.environment,Xe),ne.envMapRotation=ne.environment!==null&&C.envMap===null?J.environmentRotation:C.envMapRotation,Ie===void 0&&(C.addEventListener("dispose",rt),Ie=new Map,ne.programs=Ie);let nt=Ie.get(Ne);if(nt!==void 0){if(ne.currentProgram===nt&&ne.lightsStateVersion===we)return ra(C,ye),nt}else ye.uniforms=De.getUniforms(C),P!==null&&C.isNodeMaterial&&P.build(C,oe,ye),C.onBeforeCompile(ye,E),nt=De.acquireProgram(ye,Ne),Ie.set(Ne,nt),ne.uniforms=ye.uniforms;let qe=ne.uniforms;return(!C.isShaderMaterial&&!C.isRawShaderMaterial||C.clipping===!0)&&(qe.clippingPlanes=O.uniform),ra(C,ye),ne.needsLights=Ye(C),ne.lightsStateVersion=we,ne.needsLights&&(qe.ambientLightColor.value=re.state.ambient,qe.lightProbe.value=re.state.probe,qe.directionalLights.value=re.state.directional,qe.directionalLightShadows.value=re.state.directionalShadow,qe.spotLights.value=re.state.spot,qe.spotLightShadows.value=re.state.spotShadow,qe.rectAreaLights.value=re.state.rectArea,qe.ltc_1.value=re.state.rectAreaLTC1,qe.ltc_2.value=re.state.rectAreaLTC2,qe.pointLights.value=re.state.point,qe.pointLightShadows.value=re.state.pointShadow,qe.hemisphereLights.value=re.state.hemi,qe.directionalShadowMatrix.value=re.state.directionalShadowMatrix,qe.spotLightMatrix.value=re.state.spotLightMatrix,qe.spotLightMap.value=re.state.spotLightMap,qe.pointShadowMatrix.value=re.state.pointShadowMatrix),ne.lightProbeGrid=T.state.lightProbeGridArray.length>0,ne.currentProgram=nt,ne.uniformsList=null,nt}function sa(C){if(C.uniformsList===null){let J=C.currentProgram.getUniforms();C.uniformsList=Ir.seqWithValue(J.seq,C.uniforms)}return C.uniformsList}function ra(C,J){let oe=he.get(C);oe.outputColorSpace=J.outputColorSpace,oe.batching=J.batching,oe.batchingColor=J.batchingColor,oe.instancing=J.instancing,oe.instancingColor=J.instancingColor,oe.instancingMorph=J.instancingMorph,oe.skinning=J.skinning,oe.morphTargets=J.morphTargets,oe.morphNormals=J.morphNormals,oe.morphColors=J.morphColors,oe.morphTargetsCount=J.morphTargetsCount,oe.numClippingPlanes=J.numClippingPlanes,oe.numIntersection=J.numClipIntersection,oe.vertexAlphas=J.vertexAlphas,oe.vertexTangents=J.vertexTangents,oe.toneMapping=J.toneMapping}function us(C,J){if(C.length===0)return null;if(C.length===1)return C[0].texture!==null?C[0]:null;x.setFromMatrixPosition(J.matrixWorld);for(let oe=0,ne=C.length;oe<ne;oe++){let re=C[oe];if(re.texture!==null&&re.boundingBox.containsPoint(x))return re}return null}function $(C,J,oe,ne,re){J.isScene!==!0&&(J=je),de.resetTextureUnits();let Ee=J.fog,we=ne.isMeshStandardMaterial||ne.isMeshLambertMaterial||ne.isMeshPhongMaterial?J.environment:null,ye=X===null?E.outputColorSpace:X.isXRRenderTarget===!0?X.texture.colorSpace:lt.workingColorSpace,Ne=ne.isMeshStandardMaterial||ne.isMeshLambertMaterial&&!ne.envMap||ne.isMeshPhongMaterial&&!ne.envMap,Ie=Te.get(ne.envMap||we,Ne),Xe=ne.vertexColors===!0&&!!oe.attributes.color&&oe.attributes.color.itemSize===4,nt=!!oe.attributes.tangent&&(!!ne.normalMap||ne.anisotropy>0),qe=!!oe.morphAttributes.position,Et=!!oe.morphAttributes.normal,Ht=!!oe.morphAttributes.color,Bt=jn;ne.toneMapped&&(X===null||X.isXRRenderTarget===!0)&&(Bt=E.toneMapping);let Ct=oe.morphAttributes.position||oe.morphAttributes.normal||oe.morphAttributes.color,sn=Ct!==void 0?Ct.length:0,Ve=he.get(ne),Sn=T.state.lights;if(ee===!0&&(ae===!0||C!==V)){let It=C===V&&ne.id===Z;O.setState(ne,C,It)}let vt=!1;ne.version===Ve.__version?(Ve.needsLights&&Ve.lightsStateVersion!==Sn.state.version||Ve.outputColorSpace!==ye||re.isBatchedMesh&&Ve.batching===!1||!re.isBatchedMesh&&Ve.batching===!0||re.isBatchedMesh&&Ve.batchingColor===!0&&re.colorTexture===null||re.isBatchedMesh&&Ve.batchingColor===!1&&re.colorTexture!==null||re.isInstancedMesh&&Ve.instancing===!1||!re.isInstancedMesh&&Ve.instancing===!0||re.isSkinnedMesh&&Ve.skinning===!1||!re.isSkinnedMesh&&Ve.skinning===!0||re.isInstancedMesh&&Ve.instancingColor===!0&&re.instanceColor===null||re.isInstancedMesh&&Ve.instancingColor===!1&&re.instanceColor!==null||re.isInstancedMesh&&Ve.instancingMorph===!0&&re.morphTexture===null||re.isInstancedMesh&&Ve.instancingMorph===!1&&re.morphTexture!==null||Ve.envMap!==Ie||ne.fog===!0&&Ve.fog!==Ee||Ve.numClippingPlanes!==void 0&&(Ve.numClippingPlanes!==O.numPlanes||Ve.numIntersection!==O.numIntersection)||Ve.vertexAlphas!==Xe||Ve.vertexTangents!==nt||Ve.morphTargets!==qe||Ve.morphNormals!==Et||Ve.morphColors!==Ht||Ve.toneMapping!==Bt||Ve.morphTargetsCount!==sn||!!Ve.lightProbeGrid!=T.state.lightProbeGridArray.length>0)&&(vt=!0):(vt=!0,Ve.__version=ne.version);let Ln=Ve.currentProgram;vt===!0&&(Ln=hs(ne,J,re),P&&ne.isNodeMaterial&&P.onUpdateProgram(ne,Ln,Ve));let ni=!1,Oi=!1,Gs=!1,Pt=Ln.getUniforms(),Vt=Ve.uniforms;if(M.useProgram(Ln.program)&&(ni=!0,Oi=!0,Gs=!0),ne.id!==Z&&(Z=ne.id,Oi=!0),Ve.needsLights){let It=us(T.state.lightProbeGridArray,re);Ve.lightProbeGrid!==It&&(Ve.lightProbeGrid=It,Oi=!0)}if(ni||V!==C){M.buffers.depth.getReversed()&&C.reversedDepth!==!0&&(C._reversedDepth=!0,C.updateProjectionMatrix()),Pt.setValue(j,"projectionMatrix",C.projectionMatrix),Pt.setValue(j,"viewMatrix",C.matrixWorldInverse);let zi=Pt.map.cameraPosition;zi!==void 0&&zi.setValue(j,Ue.setFromMatrixPosition(C.matrixWorld)),B.logarithmicDepthBuffer&&Pt.setValue(j,"logDepthBufFC",2/(Math.log(C.far+1)/Math.LN2)),(ne.isMeshPhongMaterial||ne.isMeshToonMaterial||ne.isMeshLambertMaterial||ne.isMeshBasicMaterial||ne.isMeshStandardMaterial||ne.isShaderMaterial)&&Pt.setValue(j,"isOrthographic",C.isOrthographicCamera===!0),V!==C&&(V=C,Oi=!0,Gs=!0)}if(Ve.needsLights&&(Sn.state.directionalShadowMap.length>0&&Pt.setValue(j,"directionalShadowMap",Sn.state.directionalShadowMap,de),Sn.state.spotShadowMap.length>0&&Pt.setValue(j,"spotShadowMap",Sn.state.spotShadowMap,de),Sn.state.pointShadowMap.length>0&&Pt.setValue(j,"pointShadowMap",Sn.state.pointShadowMap,de)),re.isSkinnedMesh){Pt.setOptional(j,re,"bindMatrix"),Pt.setOptional(j,re,"bindMatrixInverse");let It=re.skeleton;It&&(It.boneTexture===null&&It.computeBoneTexture(),Pt.setValue(j,"boneTexture",It.boneTexture,de))}re.isBatchedMesh&&(Pt.setOptional(j,re,"batchingTexture"),Pt.setValue(j,"batchingTexture",re._matricesTexture,de),Pt.setOptional(j,re,"batchingIdTexture"),Pt.setValue(j,"batchingIdTexture",re._indirectTexture,de),Pt.setOptional(j,re,"batchingColorTexture"),re._colorsTexture!==null&&Pt.setValue(j,"batchingColorTexture",re._colorsTexture,de));let Bi=oe.morphAttributes;if((Bi.position!==void 0||Bi.normal!==void 0||Bi.color!==void 0)&&N.update(re,oe,Ln),(Oi||Ve.receiveShadow!==re.receiveShadow)&&(Ve.receiveShadow=re.receiveShadow,Pt.setValue(j,"receiveShadow",re.receiveShadow)),(ne.isMeshStandardMaterial||ne.isMeshLambertMaterial||ne.isMeshPhongMaterial)&&ne.envMap===null&&J.environment!==null&&(Vt.envMapIntensity.value=J.environmentIntensity),Vt.dfgLUT!==void 0&&(Vt.dfgLUT.value=zv()),Oi){if(Pt.setValue(j,"toneMappingExposure",E.toneMappingExposure),Ve.needsLights&&Be(Vt,Gs),Ee&&ne.fog===!0&&We.refreshFogUniforms(Vt,Ee),We.refreshMaterialUniforms(Vt,ne,me,Me,T.state.transmissionRenderTarget[C.id]),Ve.needsLights&&Ve.lightProbeGrid){let It=Ve.lightProbeGrid;Vt.probesSH.value=It.texture,Vt.probesMin.value.copy(It.boundingBox.min),Vt.probesMax.value.copy(It.boundingBox.max),Vt.probesResolution.value.copy(It.resolution)}Ir.upload(j,sa(Ve),Vt,de)}if(ne.isShaderMaterial&&ne.uniformsNeedUpdate===!0&&(Ir.upload(j,sa(Ve),Vt,de),ne.uniformsNeedUpdate=!1),ne.isSpriteMaterial&&Pt.setValue(j,"center",re.center),Pt.setValue(j,"modelViewMatrix",re.modelViewMatrix),Pt.setValue(j,"normalMatrix",re.normalMatrix),Pt.setValue(j,"modelMatrix",re.matrixWorld),ne.uniformsGroups!==void 0){let It=ne.uniformsGroups;for(let zi=0,Ws=It.length;zi<Ws;zi++){let ku=It[zi];se.update(ku,Ln),se.bind(ku,Ln)}}return Ln}function Be(C,J){C.ambientLightColor.needsUpdate=J,C.lightProbe.needsUpdate=J,C.directionalLights.needsUpdate=J,C.directionalLightShadows.needsUpdate=J,C.pointLights.needsUpdate=J,C.pointLightShadows.needsUpdate=J,C.spotLights.needsUpdate=J,C.spotLightShadows.needsUpdate=J,C.rectAreaLights.needsUpdate=J,C.hemisphereLights.needsUpdate=J}function Ye(C){return C.isMeshLambertMaterial||C.isMeshToonMaterial||C.isMeshPhongMaterial||C.isMeshStandardMaterial||C.isShadowMaterial||C.isShaderMaterial&&C.lights===!0}this.getActiveCubeFace=function(){return F},this.getActiveMipmapLevel=function(){return S},this.getRenderTarget=function(){return X},this.setRenderTargetTextures=function(C,J,oe){let ne=he.get(C);ne.__autoAllocateDepthBuffer=C.resolveDepthBuffer===!1,ne.__autoAllocateDepthBuffer===!1&&(ne.__useRenderToTexture=!1),he.get(C.texture).__webglTexture=J,he.get(C.depthTexture).__webglTexture=ne.__autoAllocateDepthBuffer?void 0:oe,ne.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(C,J){let oe=he.get(C);oe.__webglFramebuffer=J,oe.__useDefaultFramebuffer=J===void 0},this.setRenderTarget=function(C,J=0,oe=0){X=C,F=J,S=oe;let ne=null,re=!1,Ee=!1;if(C){let ye=he.get(C);if(ye.__useDefaultFramebuffer!==void 0){M.bindFramebuffer(j.FRAMEBUFFER,ye.__webglFramebuffer),ie.copy(C.viewport),fe.copy(C.scissor),Ce=C.scissorTest,M.viewport(ie),M.scissor(fe),M.setScissorTest(Ce),Z=-1;return}else if(ye.__webglFramebuffer===void 0)de.setupRenderTarget(C);else if(ye.__hasExternalTextures)de.rebindTextures(C,he.get(C.texture).__webglTexture,he.get(C.depthTexture).__webglTexture);else if(C.depthBuffer){let Xe=C.depthTexture;if(ye.__boundDepthTexture!==Xe){if(Xe!==null&&he.has(Xe)&&(C.width!==Xe.image.width||C.height!==Xe.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");de.setupDepthRenderbuffer(C)}}let Ne=C.texture;(Ne.isData3DTexture||Ne.isDataArrayTexture||Ne.isCompressedArrayTexture)&&(Ee=!0);let Ie=he.get(C).__webglFramebuffer;C.isWebGLCubeRenderTarget?(Array.isArray(Ie[J])?ne=Ie[J][oe]:ne=Ie[J],re=!0):C.samples>0&&de.useMultisampledRTT(C)===!1?ne=he.get(C).__webglMultisampledFramebuffer:Array.isArray(Ie)?ne=Ie[oe]:ne=Ie,ie.copy(C.viewport),fe.copy(C.scissor),Ce=C.scissorTest}else ie.copy(xe).multiplyScalar(me).floor(),fe.copy(H).multiplyScalar(me).floor(),Ce=K;if(oe!==0&&(ne=W),M.bindFramebuffer(j.FRAMEBUFFER,ne)&&M.drawBuffers(C,ne),M.viewport(ie),M.scissor(fe),M.setScissorTest(Ce),re){let ye=he.get(C.texture);j.framebufferTexture2D(j.FRAMEBUFFER,j.COLOR_ATTACHMENT0,j.TEXTURE_CUBE_MAP_POSITIVE_X+J,ye.__webglTexture,oe)}else if(Ee){let ye=J;for(let Ne=0;Ne<C.textures.length;Ne++){let Ie=he.get(C.textures[Ne]);j.framebufferTextureLayer(j.FRAMEBUFFER,j.COLOR_ATTACHMENT0+Ne,Ie.__webglTexture,oe,ye)}}else if(C!==null&&oe!==0){let ye=he.get(C.texture);j.framebufferTexture2D(j.FRAMEBUFFER,j.COLOR_ATTACHMENT0,j.TEXTURE_2D,ye.__webglTexture,oe)}Z=-1},this.readRenderTargetPixels=function(C,J,oe,ne,re,Ee,we,ye=0){if(!(C&&C.isWebGLRenderTarget)){$e("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let Ne=he.get(C).__webglFramebuffer;if(C.isWebGLCubeRenderTarget&&we!==void 0&&(Ne=Ne[we]),Ne){M.bindFramebuffer(j.FRAMEBUFFER,Ne);try{let Ie=C.textures[ye],Xe=Ie.format,nt=Ie.type;if(C.textures.length>1&&j.readBuffer(j.COLOR_ATTACHMENT0+ye),!B.textureFormatReadable(Xe)){$e("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(!B.textureTypeReadable(nt)){$e("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}J>=0&&J<=C.width-ne&&oe>=0&&oe<=C.height-re&&j.readPixels(J,oe,ne,re,G.convert(Xe),G.convert(nt),Ee)}finally{let Ie=X!==null?he.get(X).__webglFramebuffer:null;M.bindFramebuffer(j.FRAMEBUFFER,Ie)}}},this.readRenderTargetPixelsAsync=async function(C,J,oe,ne,re,Ee,we,ye=0){if(!(C&&C.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let Ne=he.get(C).__webglFramebuffer;if(C.isWebGLCubeRenderTarget&&we!==void 0&&(Ne=Ne[we]),Ne)if(J>=0&&J<=C.width-ne&&oe>=0&&oe<=C.height-re){M.bindFramebuffer(j.FRAMEBUFFER,Ne);let Ie=C.textures[ye],Xe=Ie.format,nt=Ie.type;if(C.textures.length>1&&j.readBuffer(j.COLOR_ATTACHMENT0+ye),!B.textureFormatReadable(Xe))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(!B.textureTypeReadable(nt))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let qe=j.createBuffer();j.bindBuffer(j.PIXEL_PACK_BUFFER,qe),j.bufferData(j.PIXEL_PACK_BUFFER,Ee.byteLength,j.STREAM_READ),j.readPixels(J,oe,ne,re,G.convert(Xe),G.convert(nt),0);let Et=X!==null?he.get(X).__webglFramebuffer:null;M.bindFramebuffer(j.FRAMEBUFFER,Et);let Ht=j.fenceSync(j.SYNC_GPU_COMMANDS_COMPLETE,0);return j.flush(),await df(j,Ht,4),j.bindBuffer(j.PIXEL_PACK_BUFFER,qe),j.getBufferSubData(j.PIXEL_PACK_BUFFER,0,Ee),j.deleteBuffer(qe),j.deleteSync(Ht),Ee}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(C,J=null,oe=0){let ne=Math.pow(2,-oe),re=Math.floor(C.image.width*ne),Ee=Math.floor(C.image.height*ne),we=J!==null?J.x:0,ye=J!==null?J.y:0;de.setTexture2D(C,0),j.copyTexSubImage2D(j.TEXTURE_2D,oe,0,0,we,ye,re,Ee),M.unbindTexture()},this.copyTextureToTexture=function(C,J,oe=null,ne=null,re=0,Ee=0){let we,ye,Ne,Ie,Xe,nt,qe,Et,Ht,Bt=C.isCompressedTexture?C.mipmaps[Ee]:C.image;if(oe!==null)we=oe.max.x-oe.min.x,ye=oe.max.y-oe.min.y,Ne=oe.isBox3?oe.max.z-oe.min.z:1,Ie=oe.min.x,Xe=oe.min.y,nt=oe.isBox3?oe.min.z:0;else{let Vt=Math.pow(2,-re);we=Math.floor(Bt.width*Vt),ye=Math.floor(Bt.height*Vt),C.isDataArrayTexture?Ne=Bt.depth:C.isData3DTexture?Ne=Math.floor(Bt.depth*Vt):Ne=1,Ie=0,Xe=0,nt=0}ne!==null?(qe=ne.x,Et=ne.y,Ht=ne.z):(qe=0,Et=0,Ht=0);let Ct=G.convert(J.format),sn=G.convert(J.type),Ve;J.isData3DTexture?(de.setTexture3D(J,0),Ve=j.TEXTURE_3D):J.isDataArrayTexture||J.isCompressedArrayTexture?(de.setTexture2DArray(J,0),Ve=j.TEXTURE_2D_ARRAY):(de.setTexture2D(J,0),Ve=j.TEXTURE_2D),M.activeTexture(j.TEXTURE0),M.pixelStorei(j.UNPACK_FLIP_Y_WEBGL,J.flipY),M.pixelStorei(j.UNPACK_PREMULTIPLY_ALPHA_WEBGL,J.premultiplyAlpha),M.pixelStorei(j.UNPACK_ALIGNMENT,J.unpackAlignment);let Sn=M.getParameter(j.UNPACK_ROW_LENGTH),vt=M.getParameter(j.UNPACK_IMAGE_HEIGHT),Ln=M.getParameter(j.UNPACK_SKIP_PIXELS),ni=M.getParameter(j.UNPACK_SKIP_ROWS),Oi=M.getParameter(j.UNPACK_SKIP_IMAGES);M.pixelStorei(j.UNPACK_ROW_LENGTH,Bt.width),M.pixelStorei(j.UNPACK_IMAGE_HEIGHT,Bt.height),M.pixelStorei(j.UNPACK_SKIP_PIXELS,Ie),M.pixelStorei(j.UNPACK_SKIP_ROWS,Xe),M.pixelStorei(j.UNPACK_SKIP_IMAGES,nt);let Gs=C.isDataArrayTexture||C.isData3DTexture,Pt=J.isDataArrayTexture||J.isData3DTexture;if(C.isDepthTexture){let Vt=he.get(C),Bi=he.get(J),It=he.get(Vt.__renderTarget),zi=he.get(Bi.__renderTarget);M.bindFramebuffer(j.READ_FRAMEBUFFER,It.__webglFramebuffer),M.bindFramebuffer(j.DRAW_FRAMEBUFFER,zi.__webglFramebuffer);for(let Ws=0;Ws<Ne;Ws++)Gs&&(j.framebufferTextureLayer(j.READ_FRAMEBUFFER,j.COLOR_ATTACHMENT0,he.get(C).__webglTexture,re,nt+Ws),j.framebufferTextureLayer(j.DRAW_FRAMEBUFFER,j.COLOR_ATTACHMENT0,he.get(J).__webglTexture,Ee,Ht+Ws)),j.blitFramebuffer(Ie,Xe,we,ye,qe,Et,we,ye,j.DEPTH_BUFFER_BIT,j.NEAREST);M.bindFramebuffer(j.READ_FRAMEBUFFER,null),M.bindFramebuffer(j.DRAW_FRAMEBUFFER,null)}else if(re!==0||C.isRenderTargetTexture||he.has(C)){let Vt=he.get(C),Bi=he.get(J);M.bindFramebuffer(j.READ_FRAMEBUFFER,R),M.bindFramebuffer(j.DRAW_FRAMEBUFFER,U);for(let It=0;It<Ne;It++)Gs?j.framebufferTextureLayer(j.READ_FRAMEBUFFER,j.COLOR_ATTACHMENT0,Vt.__webglTexture,re,nt+It):j.framebufferTexture2D(j.READ_FRAMEBUFFER,j.COLOR_ATTACHMENT0,j.TEXTURE_2D,Vt.__webglTexture,re),Pt?j.framebufferTextureLayer(j.DRAW_FRAMEBUFFER,j.COLOR_ATTACHMENT0,Bi.__webglTexture,Ee,Ht+It):j.framebufferTexture2D(j.DRAW_FRAMEBUFFER,j.COLOR_ATTACHMENT0,j.TEXTURE_2D,Bi.__webglTexture,Ee),re!==0?j.blitFramebuffer(Ie,Xe,we,ye,qe,Et,we,ye,j.COLOR_BUFFER_BIT,j.NEAREST):Pt?j.copyTexSubImage3D(Ve,Ee,qe,Et,Ht+It,Ie,Xe,we,ye):j.copyTexSubImage2D(Ve,Ee,qe,Et,Ie,Xe,we,ye);M.bindFramebuffer(j.READ_FRAMEBUFFER,null),M.bindFramebuffer(j.DRAW_FRAMEBUFFER,null)}else Pt?C.isDataTexture||C.isData3DTexture?j.texSubImage3D(Ve,Ee,qe,Et,Ht,we,ye,Ne,Ct,sn,Bt.data):J.isCompressedArrayTexture?j.compressedTexSubImage3D(Ve,Ee,qe,Et,Ht,we,ye,Ne,Ct,Bt.data):j.texSubImage3D(Ve,Ee,qe,Et,Ht,we,ye,Ne,Ct,sn,Bt):C.isDataTexture?j.texSubImage2D(j.TEXTURE_2D,Ee,qe,Et,we,ye,Ct,sn,Bt.data):C.isCompressedTexture?j.compressedTexSubImage2D(j.TEXTURE_2D,Ee,qe,Et,Bt.width,Bt.height,Ct,Bt.data):j.texSubImage2D(j.TEXTURE_2D,Ee,qe,Et,we,ye,Ct,sn,Bt);M.pixelStorei(j.UNPACK_ROW_LENGTH,Sn),M.pixelStorei(j.UNPACK_IMAGE_HEIGHT,vt),M.pixelStorei(j.UNPACK_SKIP_PIXELS,Ln),M.pixelStorei(j.UNPACK_SKIP_ROWS,ni),M.pixelStorei(j.UNPACK_SKIP_IMAGES,Oi),Ee===0&&J.generateMipmaps&&j.generateMipmap(Ve),M.unbindTexture()},this.initRenderTarget=function(C){he.get(C).__webglFramebuffer===void 0&&de.setupRenderTarget(C)},this.initTexture=function(C){C.isCubeTexture?de.setTextureCube(C,0):C.isData3DTexture?de.setTexture3D(C,0):C.isDataArrayTexture||C.isCompressedArrayTexture?de.setTexture2DArray(C,0):de.setTexture2D(C,0),M.unbindTexture()},this.resetState=function(){F=0,S=0,X=null,M.reset(),le.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return qn}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=lt._getDrawingBufferColorSpace(e),t.unpackColorSpace=lt._getUnpackColorSpace()}};function Zh(s,e){if(e===Eh)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Cr||e===Xo){let t=s.getIndex();if(t===null){let o=[],a=s.getAttribute("position");if(a!==void 0){for(let l=0;l<a.count;l++)o.push(l);s.setIndex(o),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Cr)for(let o=1;o<=n;o++)i.push(t.getX(0)),i.push(t.getX(o)),i.push(t.getX(o+1));else for(let o=0;o<n;o++)o%2===0?(i.push(t.getX(o)),i.push(t.getX(o+1)),i.push(t.getX(o+2))):(i.push(t.getX(o+2)),i.push(t.getX(o+1)),i.push(t.getX(o)));i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles.");let r=s.clone();return r.setIndex(i),r.clearGroups(),r}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function Xf(s){let e=new Map,t=new Map,n=s.clone();return qf(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,o=e.get(i),a=o.skeleton.bones;r.skeleton=o.skeleton.clone(),r.bindMatrix.copy(o.bindMatrix),r.skeleton.bones=a.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function qf(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)qf(s.children[n],e.children[n],t)}var os=class extends di{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new tu(t)}),this.register(function(t){return new nu(t)}),this.register(function(t){return new uu(t)}),this.register(function(t){return new du(t)}),this.register(function(t){return new fu(t)}),this.register(function(t){return new su(t)}),this.register(function(t){return new ru(t)}),this.register(function(t){return new ou(t)}),this.register(function(t){return new au(t)}),this.register(function(t){return new eu(t)}),this.register(function(t){return new lu(t)}),this.register(function(t){return new iu(t)}),this.register(function(t){return new hu(t)}),this.register(function(t){return new cu(t)}),this.register(function(t){return new $h(t)}),this.register(function(t){return new dc(t,mt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new dc(t,mt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new pu(t)})}load(e,t,n,i){let r=this,o;if(this.resourcePath!=="")o=this.resourcePath;else if(this.path!==""){let c=Di.extractUrlBase(e);o=Di.resolveURL(c,this.path)}else o=Di.extractUrlBase(e);this.manager.itemStart(e);let a=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new Sr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,o,function(h){t(h),r.manager.itemEnd(e)},a)}catch(h){a(h)}},n,a)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,o={},a={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===Jf){try{o[mt.KHR_BINARY_GLTF]=new mu(e)}catch(u){i&&i(u);return}r=JSON.parse(o[mt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new bu(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),a[u.name]=u,o[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],d=r.extensionsRequired||[];switch(u){case mt.KHR_MATERIALS_UNLIT:o[u]=new Qh;break;case mt.KHR_DRACO_MESH_COMPRESSION:o[u]=new gu(r,this.dracoLoader);break;case mt.KHR_TEXTURE_TRANSFORM:o[u]=new xu;break;case mt.KHR_MESH_QUANTIZATION:o[u]=new _u;break;default:d.indexOf(u)>=0&&a[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(o),c.setPlugins(a),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function kv(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function qt(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var mt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},$h=class{constructor(e){this.parser=e,this.name=mt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new be(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],dn);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new $i(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new Kn(h),c.distance=u;break;case"spot":c=new Eo(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),gi(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],a=(r.extensions&&r.extensions[this.name]||{}).light;return a===void 0?null:this._loadLight(a).then(function(l){return n._getNodeRef(t.cache,a,l)})}},Qh=class{constructor(){this.name=mt.KHR_MATERIALS_UNLIT}getMaterialType(){return Mt}extendParams(e,t,n){let i=[];e.color=new be(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let o=r.baseColorFactor;e.color.setRGB(o[0],o[1],o[2],dn),e.opacity=o[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,Ut))}return Promise.all(i)}},eu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},tu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new Re(r,r)}return Promise.all(i)}},nu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},iu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},su=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_SHEEN}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new be(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],dn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,Ut)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},ru=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},ou=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_VOLUME}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new be().setRGB(r[0],r[1],r[2],dn),Promise.all(i)}},au=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_IOR}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},lu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new be().setRGB(r[0],r[1],r[2],dn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,Ut)),Promise.all(i)}},cu=class{constructor(e){this.parser=e,this.name=mt.EXT_MATERIALS_BUMP}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},hu=class{constructor(e){this.parser=e,this.name=mt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return qt(this.parser,e,this.name)!==null?_n:null}extendMaterialParams(e,t){let n=qt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},uu=class{constructor(e){this.parser=e,this.name=mt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],o=t.options.ktx2Loader;if(!o){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,o)}},du=class{constructor(e){this.parser=e,this.name=mt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},fu=class{constructor(e){this.parser=e,this.name=mt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},dc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),o=this.parser.options.meshoptDecoder;if(!o||!o.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(a){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,d=new Uint8Array(a,l,c);return o.decodeGltfBufferAsync?o.decodeGltfBufferAsync(h,u,d,i.mode,i.filter).then(function(f){return f.buffer}):o.ready.then(function(){let f=new ArrayBuffer(h*u);return o.decodeGltfBuffer(new Uint8Array(f),h,u,d,i.mode,i.filter),f})})}else return null}},pu=class{constructor(e){this.name=mt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==kn.TRIANGLES&&c.mode!==kn.TRIANGLE_STRIP&&c.mode!==kn.TRIANGLE_FAN&&c.mode!==void 0)return null;let o=n.extensions[this.name].attributes,a=[],l={};for(let c in o)a.push(this.parser.getDependency("accessor",o[c]).then(h=>(l[c]=h,l[c])));return a.length<1?null:(a.push(this.parser.createNodeMesh(e)),Promise.all(a).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],d=c[0].count,f=[];for(let g of u){let v=new it,m=new z,p=new Gt,y=new z(1,1,1),w=new gn(g.geometry,g.material,d);for(let x=0;x<d;x++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,x),l.ROTATION&&p.fromBufferAttribute(l.ROTATION,x),l.SCALE&&y.fromBufferAttribute(l.SCALE,x),w.setMatrixAt(x,v.compose(m,p,y));for(let x in l)if(x==="_COLOR_0"){let A=l[x];w.instanceColor=new Ki(A.array,A.itemSize,A.normalized)}else x!=="TRANSLATION"&&x!=="ROTATION"&&x!=="SCALE"&&g.geometry.setAttribute(x,l[x]);bt.prototype.copy.call(w,g),this.parser.assignFinalMaterial(w),f.push(w)}return h.isGroup?(h.clear(),h.add(...f),h):f[0]}))}},Jf="glTF",jo=12,Yf={JSON:1313821514,BIN:5130562},mu=class{constructor(e){this.name=mt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,jo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==Jf)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-jo,r=new DataView(e,jo),o=0;for(;o<i;){let a=r.getUint32(o,!0);o+=4;let l=r.getUint32(o,!0);if(o+=4,l===Yf.JSON){let c=new Uint8Array(e,jo+o,a);this.content=n.decode(c)}else if(l===Yf.BIN){let c=jo+o;this.body=e.slice(c,c+a)}o+=a}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},gu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=mt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,o=e.extensions[this.name].attributes,a={},l={},c={};for(let h in o){let u=yu[h]||h.toLowerCase();a[u]=o[h]}for(let h in e.attributes){let u=yu[h]||h.toLowerCase();if(o[h]!==void 0){let d=n.accessors[e.attributes[h]],f=Nr[d.componentType];c[u]=f.name,l[u]=d.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,d){i.decodeDracoFile(h,function(f){for(let g in f.attributes){let v=f.attributes[g],m=l[g];m!==void 0&&(v.normalized=m)}u(f)},a,c,dn,d)})})}},xu=class{constructor(){this.name=mt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){return(t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0||(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),e.needsUpdate=!0),e}},_u=class{constructor(){this.name=mt.KHR_MESH_QUANTIZATION}},fc=class extends ui{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let o=0;o!==i;o++)t[o]=n[r+o];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=a*2,c=a*3,h=i-t,u=(n-t)/h,d=u*u,f=d*u,g=e*c,v=g-c,m=-2*f+3*d,p=f-d,y=1-m,w=p-d+u;for(let x=0;x!==a;x++){let A=o[v+x+a],T=o[v+x+l]*h,I=o[g+x+a],_=o[g+x]*h;r[x]=y*A+w*T+m*I+p*_}return r}},Hv=new Gt,vu=class extends fc{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return Hv.fromArray(r).normalize().toArray(r),r}},kn={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Nr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},Zf={9728:Wt,9729:Ft,9984:_l,9985:Tr,9986:Ns,9987:Jn},Kf={33071:Un,33648:or,10497:ri},Kh={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},yu={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},rs={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},Vv={CUBICSPLINE:void 0,LINEAR:Ss,STEP:bs},jh={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function Gv(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new xn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:Tn})),s.DefaultMaterial}function Os(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function gi(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function Wv(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let o=[],a=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let d=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;o.push(d)}if(i){let d=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;a.push(d)}if(r){let d=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(d)}}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],d=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=d),s.morphTargetsRelative=!0,s})}function Xv(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function qv(s){let e,t=s.extensions&&s.extensions[mt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+Jh(t.attributes):e=s.indices+":"+Jh(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+Jh(s.targets[n]);return e}function Jh(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function Mu(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function Yv(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var Zv=new it,bu=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new kv,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,o=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let a=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(a)===!0;let l=a.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=a.indexOf("Firefox")>-1,o=r?a.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&o<98?this.textureLoader=new Mo(this.options.manager):this.textureLoader=new To(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new Sr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(o){return o._markDefs&&o._markDefs()}),Promise.all(this._invokeAll(function(o){return o.beforeRoot&&o.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(o){let a={scene:o[0][i.scene||0],scenes:o[0],animations:o[1],cameras:o[2],asset:i.asset,parser:n,userData:{}};return Os(r,a,i),gi(a,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(a)})).then(function(){for(let l of a.scenes)l.updateMatrixWorld();e(a)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let o=t[i].joints;for(let a=0,l=o.length;a<l;a++)e[o[a]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let o=e[i];o.mesh!==void 0&&(this._addNodeRef(this.meshCache,o.mesh),o.skin!==void 0&&(n[o.mesh].isSkinnedMesh=!0)),o.camera!==void 0&&this._addNodeRef(this.cameraCache,o.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(o,a)=>{let l=this.associations.get(o);l!=null&&this.associations.set(a,l);for(let[c,h]of o.children.entries())r(h,a.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,o){return n.getDependency(e,o)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[mt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,o){n.load(Di.resolveURL(t.uri,i.path),r,void 0,function(){o(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let o=Kh[i.type],a=Nr[i.componentType],l=i.normalized===!0,c=new a(i.count*o);return Promise.resolve(new gt(c,o,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(o){let a=o[0],l=Kh[i.type],c=Nr[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,d=i.byteOffset||0,f=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,g=i.normalized===!0,v,m;if(f&&f!==u){let p=Math.floor(d/f),y="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+p+":"+i.count,w=t.cache.get(y);w||(v=new c(a,p*f,i.count*f/h),w=new fr(v,f/h),t.cache.add(y,w)),m=new pr(w,l,d%f/h,g)}else a===null?v=new c(i.count*l):v=new c(a,d,i.count*l),m=new gt(v,l,g);if(i.sparse!==void 0){let p=Kh.SCALAR,y=Nr[i.sparse.indices.componentType],w=i.sparse.indices.byteOffset||0,x=i.sparse.values.byteOffset||0,A=new y(o[1],w,i.sparse.count*p),T=new c(o[2],x,i.sparse.count*l);a!==null&&(m=new gt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let I=0,_=A.length;I<_;I++){let D=A[I];if(m.setX(D,T[I*l]),l>=2&&m.setY(D,T[I*l+1]),l>=3&&m.setZ(D,T[I*l+2]),l>=4&&m.setW(D,T[I*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=g}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,o=t.images[r],a=this.textureLoader;if(o.uri){let l=n.manager.getHandler(o.uri);l!==null&&(a=l)}return this.loadTextureImage(e,r,a)}loadTextureImage(e,t,n){let i=this,r=this.json,o=r.textures[e],a=r.images[t],l=(a.uri||a.bufferView)+":"+o.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=o.name||a.name||"",h.name===""&&typeof a.uri=="string"&&a.uri.startsWith("data:image/")===!1&&(h.name=a.uri);let d=(r.samplers||{})[o.sampler]||{};return h.magFilter=Zf[d.magFilter]||Ft,h.minFilter=Zf[d.minFilter]||Jn,h.wrapS=Kf[d.wrapS]||ri,h.wrapT=Kf[d.wrapT]||ri,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==Wt&&h.minFilter!==Ft,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let o=i.images[e],a=self.URL||self.webkitURL,l=o.uri||"",c=!1;if(o.bufferView!==void 0)l=n.getDependency("bufferView",o.bufferView).then(function(u){c=!0;let d=new Blob([u],{type:o.mimeType});return l=a.createObjectURL(d),l});else if(o.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(d,f){let g=d;t.isImageBitmapLoader===!0&&(g=function(v){let m=new Xt(v);m.needsUpdate=!0,d(m)}),t.load(Di.resolveURL(u,r.path),g,void 0,f)})}).then(function(u){return c===!0&&a.revokeObjectURL(l),gi(u,o),u.userData.mimeType=o.mimeType||Yv(o.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(o){if(!o)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(o=o.clone(),o.channel=n.texCoord),r.extensions[mt.KHR_TEXTURE_TRANSFORM]){let a=n.extensions!==void 0?n.extensions[mt.KHR_TEXTURE_TRANSFORM]:void 0;if(a){let l=r.associations.get(o);o=r.extensions[mt.KHR_TEXTURE_TRANSFORM].extendTexture(o,a),r.associations.set(o,l)}}return i!==void 0&&(o.colorSpace=i),e[t]=o,o})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,o=t.attributes.normal===void 0;if(e.isPoints){let a="PointsMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new xr,fn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(a,l)),n=l}else if(e.isLine){let a="LineBasicMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new li,fn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(a,l)),n=l}if(i||r||o){let a="ClonedMaterial:"+n.uuid+":";i&&(a+="derivative-tangents:"),r&&(a+="vertex-colors:"),o&&(a+="flat-shading:");let l=this.cache.get(a);l||(l=n.clone(),r&&(l.vertexColors=!0),o&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(a,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return xn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],o,a={},l=r.extensions||{},c=[];if(l[mt.KHR_MATERIALS_UNLIT]){let u=i[mt.KHR_MATERIALS_UNLIT];o=u.getMaterialType(),c.push(u.extendParams(a,r,t))}else{let u=r.pbrMetallicRoughness||{};if(a.color=new be(1,1,1),a.opacity=1,Array.isArray(u.baseColorFactor)){let d=u.baseColorFactor;a.color.setRGB(d[0],d[1],d[2],dn),a.opacity=d[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(a,"map",u.baseColorTexture,Ut)),a.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,a.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(a,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(a,"roughnessMap",u.metallicRoughnessTexture))),o=this._invokeOne(function(d){return d.getMaterialType&&d.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(d){return d.extendMaterialParams&&d.extendMaterialParams(e,a)})))}r.doubleSided===!0&&(a.side=At);let h=r.alphaMode||jh.OPAQUE;if(h===jh.BLEND?(a.transparent=!0,a.depthWrite=!1):(a.transparent=!1,h===jh.MASK&&(a.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&o!==Mt&&(c.push(t.assignTexture(a,"normalMap",r.normalTexture)),a.normalScale=new Re(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;a.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&o!==Mt&&(c.push(t.assignTexture(a,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(a.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&o!==Mt){let u=r.emissiveFactor;a.emissive=new be().setRGB(u[0],u[1],u[2],dn)}return r.emissiveTexture!==void 0&&o!==Mt&&c.push(t.assignTexture(a,"emissiveMap",r.emissiveTexture,Ut)),Promise.all(c).then(function(){let u=new o(a);return r.name&&(u.name=r.name),gi(u,r),t.associations.set(u,{materials:e}),r.extensions&&Os(i,u,r),u})}createUniqueName(e){let t=Tt.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(a){return n[mt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(a,t).then(function(l){return jf(l,a,t)})}let o=[];for(let a=0,l=e.length;a<l;a++){let c=e[a],h=qv(c),u=i[h];if(u)o.push(u.promise);else{let d;c.extensions&&c.extensions[mt.KHR_DRACO_MESH_COMPRESSION]?d=r(c):d=jf(new ht,c,t),i[h]={primitive:c,promise:d},o.push(d)}}return Promise.all(o)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],o=r.primitives,a=[];for(let l=0,c=o.length;l<c;l++){let h=o[l].material===void 0?Gv(this.cache):this.getDependency("material",o[l].material);a.push(h)}return a.push(t.loadGeometries(o)),Promise.all(a).then(function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let f=0,g=h.length;f<g;f++){let v=h[f],m=o[f],p,y=c[f];if(m.mode===kn.TRIANGLES||m.mode===kn.TRIANGLE_STRIP||m.mode===kn.TRIANGLE_FAN||m.mode===void 0)p=r.isSkinnedMesh===!0?new lo(v,y):new Ke(v,y),p.isSkinnedMesh===!0&&p.normalizeSkinWeights(),m.mode===kn.TRIANGLE_STRIP?p.geometry=Zh(p.geometry,Xo):m.mode===kn.TRIANGLE_FAN&&(p.geometry=Zh(p.geometry,Cr));else if(m.mode===kn.LINES)p=new Ts(v,y);else if(m.mode===kn.LINE_STRIP)p=new Ai(v,y);else if(m.mode===kn.LINE_LOOP)p=new ho(v,y);else if(m.mode===kn.POINTS)p=new ln(v,y);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(p.geometry.morphAttributes).length>0&&Xv(p,r),p.name=t.createUniqueName(r.name||"mesh_"+e),gi(p,r),m.extensions&&Os(i,p,m),t.assignFinalMaterial(p),u.push(p)}for(let f=0,g=u.length;f<g;f++)t.associations.set(u[f],{meshes:e,primitives:f});if(u.length===1)return r.extensions&&Os(i,u[0],r),u[0];let d=new dt;r.extensions&&Os(i,d,r),t.associations.set(d,{meshes:e});for(let f=0,g=u.length;f<g;f++)d.add(u[f]);return d})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new Zt(_t.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new fi(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),gi(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),o=i,a=[],l=[];for(let c=0,h=o.length;c<h;c++){let u=o[c];if(u){a.push(u);let d=new it;r!==null&&d.fromArray(r.array,c*16),l.push(d)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new co(a,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,o=[],a=[],l=[],c=[],h=[];for(let u=0,d=i.channels.length;u<d;u++){let f=i.channels[u],g=i.samplers[f.sampler],v=f.target,m=v.node,p=i.parameters!==void 0?i.parameters[g.input]:g.input,y=i.parameters!==void 0?i.parameters[g.output]:g.output;v.node!==void 0&&(o.push(this.getDependency("node",m)),a.push(this.getDependency("accessor",p)),l.push(this.getDependency("accessor",y)),c.push(g),h.push(v))}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let d=u[0],f=u[1],g=u[2],v=u[3],m=u[4],p=[];for(let w=0,x=d.length;w<x;w++){let A=d[w],T=f[w],I=g[w],_=v[w],D=m[w];if(A===void 0)continue;A.updateMatrix&&A.updateMatrix();let E=n._createAnimationTracks(A,T,I,_,D);if(E)for(let b=0;b<E.length;b++)p.push(E[b])}let y=new Ps(r,void 0,p);return gi(y,i),y})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let o=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&o.traverse(function(a){if(a.isMesh)for(let l=0,c=i.weights.length;l<c;l++)a.morphTargetInfluences[l]=i.weights[l]}),o})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),o=[],a=i.children||[];for(let c=0,h=a.length;c<h;c++)o.push(n.getDependency("node",a[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(o),l]).then(function(c){let h=c[0],u=c[1],d=c[2];d!==null&&h.traverse(function(f){f.isSkinnedMesh&&f.bind(d,Zv)});for(let f=0,g=u.length;f<g;f++)h.add(u[f]);if(h.userData.pivot!==void 0&&u.length>0){let f=h.userData.pivot,g=u[0];h.pivot=new z().fromArray(f),h.position.x-=f[0],h.position.y-=f[1],h.position.z-=f[2],g.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],o=r.name?i.createUniqueName(r.name):"",a=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&a.push(l),r.camera!==void 0&&a.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){a.push(c)}),this.nodeCache[e]=Promise.all(a).then(function(c){let h;if(r.isBone===!0?h=new mr:c.length>1?h=new dt:c.length===1?h=c[0]:h=new bt,h!==c[0])for(let u=0,d=c.length;u<d;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=o),gi(h,r),r.extensions&&Os(n,h,r),r.matrix!==void 0){let u=new it;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new dt;n.name&&(r.name=i.createUniqueName(n.name)),gi(r,n),n.extensions&&Os(t,r,n);let o=n.nodes||[],a=[];for(let l=0,c=o.length;l<c;l++)a.push(i.getDependency("node",o[l]));return Promise.all(a).then(function(l){for(let h=0,u=l.length;h<u;h++){let d=l[h];d.parent!==null?r.add(Xf(d)):r.add(d)}let c=h=>{let u=new Map;for(let[d,f]of i.associations)(d instanceof fn||d instanceof Xt)&&u.set(d,f);return h.traverse(d=>{let f=i.associations.get(d);f!=null&&u.set(d,f)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let o=[],a=e.name?e.name:e.uuid,l=[];function c(f){f.morphTargetInfluences&&l.push(f.name?f.name:f.uuid)}rs[r.path]===rs.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(a);let h;switch(rs[r.path]){case rs.weights:h=Pi;break;case rs.rotation:h=Ii;break;case rs.translation:case rs.scale:h=Ji;break;default:switch(n.itemSize){case 1:h=Pi;break;case 2:case 3:default:h=Ji;break}break}let u=i.interpolation!==void 0?Vv[i.interpolation]:Ss,d=this._getArrayFromAccessor(n);for(let f=0,g=l.length;f<g;f++){let v=new h(l[f]+"."+rs[r.path],t.array,d,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(v),o.push(v)}return o}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=Mu(t.constructor),i=new Float32Array(t.length);for(let r=0,o=t.length;r<o;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof Ii?vu:fc;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function Kv(s,e,t){let n=e.attributes,i=new Kt;if(n.POSITION!==void 0){let a=t.json.accessors[n.POSITION],l=a.min,c=a.max;if(l!==void 0&&c!==void 0){if(i.set(new z(l[0],l[1],l[2]),new z(c[0],c[1],c[2])),a.normalized){let h=Mu(Nr[a.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let a=new z,l=new z;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let d=t.json.accessors[u.POSITION],f=d.min,g=d.max;if(f!==void 0&&g!==void 0){if(l.setX(Math.max(Math.abs(f[0]),Math.abs(g[0]))),l.setY(Math.max(Math.abs(f[1]),Math.abs(g[1]))),l.setZ(Math.max(Math.abs(f[2]),Math.abs(g[2]))),d.normalized){let v=Mu(Nr[d.componentType]);l.multiplyScalar(v)}a.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(a)}s.boundingBox=i;let o=new an;i.getCenter(o.center),o.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=o}function jf(s,e,t){let n=e.attributes,i=[];function r(o,a){return t.getDependency("accessor",o).then(function(l){s.setAttribute(a,l)})}for(let o in n){let a=yu[o]||o.toLowerCase();a in s.attributes||i.push(r(n[o],a))}if(e.indices!==void 0&&!s.index){let o=t.getDependency("accessor",e.indices).then(function(a){s.setIndex(a)});i.push(o)}return lt.workingColorSpace!==dn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${lt.workingColorSpace}" not supported.`),gi(s,e),Kv(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?Wv(s,e.targets,t):s})}var $f={type:"change"},wu={type:"start"},ep={type:"end"},pc=new ai,Qf=new Nn,jv=Math.cos(70*_t.DEG2RAD),en=new z,Mn=2*Math.PI,Rt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},Su=1e-6,mc=class extends Po{constructor(e,t=null){super(e,t),this.state=Rt.NONE,this.target=new z,this.cursor=new z,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:Qi.ROTATE,MIDDLE:Qi.DOLLY,RIGHT:Qi.PAN},this.touches={ONE:es.ROTATE,TWO:es.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new z,this._lastQuaternion=new Gt,this._lastTargetPosition=new z,this._quat=new Gt().setFromUnitVectors(e.up,new z(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new wr,this._sphericalDelta=new wr,this._scale=1,this._panOffset=new z,this._rotateStart=new Re,this._rotateEnd=new Re,this._rotateDelta=new Re,this._panStart=new Re,this._panEnd=new Re,this._panDelta=new Re,this._dollyStart=new Re,this._dollyEnd=new Re,this._dollyDelta=new Re,this._dollyDirection=new z,this._mouse=new Re,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=$v.bind(this),this._onPointerDown=Jv.bind(this),this._onPointerUp=Qv.bind(this),this._onContextMenu=oy.bind(this),this._onMouseWheel=ny.bind(this),this._onKeyDown=iy.bind(this),this._onTouchStart=sy.bind(this),this._onTouchMove=ry.bind(this),this._onMouseDown=ey.bind(this),this._onMouseMove=ty.bind(this),this._interceptControlDown=ay.bind(this),this._interceptControlUp=ly.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents(),this.domElement.getRootNode().removeEventListener("keydown",this._interceptControlDown,{capture:!0}),this.domElement.style.touchAction=""}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent($f),this.update(),this.state=Rt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;en.copy(t).sub(this.target),en.applyQuaternion(this._quat),this._spherical.setFromVector3(en),this.autoRotate&&this.state===Rt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=Mn:n>Math.PI&&(n-=Mn),i<-Math.PI?i+=Mn:i>Math.PI&&(i-=Mn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let o=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=o!=this._spherical.radius}if(en.setFromSpherical(this._spherical),en.applyQuaternion(this._quatInverse),t.copy(this.target).add(en),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let o=null;if(this.object.isPerspectiveCamera){let a=en.length();o=this._clampDistance(a*this._scale);let l=a-o;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let a=new z(this._mouse.x,this._mouse.y,0);a.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new z(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(a),this.object.updateMatrixWorld(),o=en.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;o!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(o).add(this.object.position):(pc.origin.copy(this.object.position),pc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(pc.direction))<jv?this.object.lookAt(this.target):(Qf.setFromNormalAndCoplanarPoint(this.object.up,this.target),pc.intersectPlane(Qf,this.target))))}else if(this.object.isOrthographicCamera){let o=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),o!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>Su||8*(1-this._lastQuaternion.dot(this.object.quaternion))>Su||this._lastTargetPosition.distanceToSquared(this.target)>Su?(this.dispatchEvent($f),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?Mn/60*this.autoRotateSpeed*e:Mn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){en.setFromMatrixColumn(t,0),en.multiplyScalar(-e),this._panOffset.add(en)}_panUp(e,t){this.screenSpacePanning===!0?en.setFromMatrixColumn(t,1):(en.setFromMatrixColumn(t,0),en.crossVectors(this.object.up,en)),en.multiplyScalar(e),this._panOffset.add(en)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;en.copy(i).sub(this.target);let r=en.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,o=n.width,a=n.height;this._mouse.x=i/o*2-1,this._mouse.y=-(r/a)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Mn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Mn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(Mn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-Mn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(Mn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-Mn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Mn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Mn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let o=(e.pageX+t.x)*.5,a=(e.pageY+t.y)*.5;this._updateZoomParameters(o,a)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new Re,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function Jv(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function $v(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function Qv(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(ep),this.state=Rt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function ey(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case Qi.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Rt.DOLLY;break;case Qi.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Rt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Rt.ROTATE}break;case Qi.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Rt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Rt.PAN}break;default:this.state=Rt.NONE}this.state!==Rt.NONE&&this.dispatchEvent(wu)}function ty(s){switch(this.state){case Rt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Rt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Rt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function ny(s){this.enabled===!1||this.enableZoom===!1||this.state!==Rt.NONE||(s.preventDefault(),this.dispatchEvent(wu),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(ep))}function iy(s){this.enabled!==!1&&this._handleKeyDown(s)}function sy(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case es.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Rt.TOUCH_ROTATE;break;case es.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Rt.TOUCH_PAN;break;default:this.state=Rt.NONE}break;case 2:switch(this.touches.TWO){case es.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Rt.TOUCH_DOLLY_PAN;break;case es.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Rt.TOUCH_DOLLY_ROTATE;break;default:this.state=Rt.NONE}break;default:this.state=Rt.NONE}this.state!==Rt.NONE&&this.dispatchEvent(wu)}function ry(s){switch(this._trackPointer(s),this.state){case Rt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Rt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Rt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Rt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Rt.NONE}}function oy(s){this.enabled!==!1&&s.preventDefault()}function ay(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function ly(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var gc=class extends Es{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new ci;e.deleteAttribute("uv");let t=new xn({side:jt}),n=new xn,i=new Kn(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new Ke(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let o=new gn(e,n,6),a=new bt;a.position.set(-10.906,2.009,1.846),a.rotation.set(0,-.195,0),a.scale.set(2.328,7.905,4.651),a.updateMatrix(),o.setMatrixAt(0,a.matrix),a.position.set(-5.607,-.754,-.758),a.rotation.set(0,.994,0),a.scale.set(1.97,1.534,3.955),a.updateMatrix(),o.setMatrixAt(1,a.matrix),a.position.set(6.167,.857,7.803),a.rotation.set(0,.561,0),a.scale.set(3.927,6.285,3.687),a.updateMatrix(),o.setMatrixAt(2,a.matrix),a.position.set(-2.017,.018,6.124),a.rotation.set(0,.333,0),a.scale.set(2.002,4.566,2.064),a.updateMatrix(),o.setMatrixAt(3,a.matrix),a.position.set(2.291,-.756,-2.621),a.rotation.set(0,-.286,0),a.scale.set(1.546,1.552,1.496),a.updateMatrix(),o.setMatrixAt(4,a.matrix),a.position.set(-2.193,-.369,-5.547),a.rotation.set(0,.516,0),a.scale.set(3.875,3.487,2.986),a.updateMatrix(),o.setMatrixAt(5,a.matrix),this.add(o);let l=new Ke(e,Ur(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new Ke(e,Ur(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new Ke(e,Ur(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new Ke(e,Ur(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let d=new Ke(e,Ur(20));d.position.set(3.235,11.486,-12.541),d.scale.set(2.5,2,.1),this.add(d);let f=new Ke(e,Ur(100));f.position.set(0,20,0),f.scale.set(1,.1,1),this.add(f)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Ur(s){return new _o({color:0,emissive:16777215,emissiveIntensity:s})}var Ni={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var In=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},cy=new fi(-1,1,1,-1,0,1),Eu=class extends ht{constructor(){super(),this.setAttribute("position",new et([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new et([0,2,0,0,2,0],2))}},hy=new Eu,xi=class{constructor(e){this._mesh=new Ke(hy,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,cy)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Fr=class extends In{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof pt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=Qn.clone(e.uniforms),this.material=new pt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new xi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Jo=class extends In{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},xc=class extends In{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var _c=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new Re);this._width=n.width,this._height=n.height,t=new zt(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Jt}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Fr(Ni),this.copyPass.material.blending=On,this.timer=new Ao}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let o=this.passes[i];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),o.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),o.needsSwap){if(n){let a=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}Jo!==void 0&&(o instanceof Jo?n=!0:o instanceof xc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new Re);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var tp={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new be(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Or=class s extends In{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new Re(e.x,e.y):new Re(256,256),this.clearColor=new be(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new zt(r,o,{type:Jt}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new zt(r,o,{type:Jt});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let d=new zt(r,o,{type:Jt});d.texture.name="UnrealBloomPass.v"+h,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),r=Math.round(r/2),o=Math.round(o/2)}let a=tp;this.highPassUniforms=Qn.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new pt({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new Re(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new z(1,1,1),new z(1,1,1),new z(1,1,1),new z(1,1,1),new z(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=Qn.clone(Ni.uniforms),this.blendMaterial=new pt({uniforms:this.copyUniforms,vertexShader:Ni.vertexShader,fragmentShader:Ni.fragmentShader,premultipliedAlpha:!0,blending:kt,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new be,this._oldClearAlpha=1,this._basic=new Mt,this._fsQuad=new xi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new Re(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let o=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let a=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),a=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=o}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let i=0;i<e;i++)t.push(.39894*Math.exp(-.5*i*i/(n*n))/n);return new pt({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new Re(.5,.5)},direction:{value:new Re(.5,.5)},gaussianCoefficients:{value:t}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new pt({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};Or.BlurDirectionX=new Re(1,0);Or.BlurDirectionY=new Re(0,1);var $o={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var vc=class extends In{constructor(){super(),this.isOutputPass=!0,this.uniforms=Qn.clone($o.uniforms),this.material=new br({name:$o.name,uniforms:this.uniforms,vertexShader:$o.vertexShader,fragmentShader:$o.fragmentShader}),this._fsQuad=new xi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},lt.getTransfer(this._outputColorSpace)===yt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Lo?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Do?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===No?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Ls?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Fo?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Oo?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Uo&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var ei=512,uy=7.5;function Tu(s,e=uy){let t=new mo,n=s.map(([a,l])=>new z(a,0,l)),i=[],r=[],o=n.length;for(let a=0;a<o;a++){let l=n[a],c=n[(a+o-1)%o],h=n[(a+1)%o],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let a=0;a<o;a++)t.add(new Zn(i[a],n[a],r[a])),t.add(new yr(r[a],i[(a+1)%o]));return t}function dy(s){let e=Tu(s),t=e.getSpacedPoints(ei),n=[];for(let i=0;i<ei;i++){let r=t[(i+ei-1)%ei],o=t[(i+1)%ei];n.push(new z(o.x-r.x,0,o.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function np(s){let e=[],t=(n,i,r,o)=>{let a=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*a+r*l,z:n.z-i*l+r*a,r:o,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function ip({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:o=()=>!1}){let a=s.map(dy),l=s.map((E,b)=>({route:b,s:a[b].length*(b*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[b]??5.1+b*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new z,h=new z,u=new z;function d(E,b,P,W,R){let U=(b/E.length%1+1)%1*ei,F=Math.floor(U)%ei,S=U-Math.floor(U);W.lerpVectors(E.points[F],E.points[(F+1)%ei],S),R.lerpVectors(E.tangents[F],E.tangents[(F+1)%ei],S).normalize().multiplyScalar(P)}let f=E=>Math.atan2(Math.sin(E),Math.cos(E));function g(E){d(a[E.route],E.s,E.dir,c,h),E.fx=h.x,E.fz=h.z,E.rx=-h.z,E.rz=h.x,E.x=c.x+E.rx*E.lane,E.z=c.z+E.rz*E.lane}function v(E,b){for(let P of e){let W=P.r+t;if((E-P.x)**2+(b-P.z)**2<W*W)return!0}return!1}function m(E,b){let P=a[E.route];for(let W of[-2.5,0,1,3,6,9,12,15,18])if(d(P,E.s+E.dir*W,E.dir,c,h),u.set(c.x-h.z*b,0,c.z+h.x*b),v(u.x,u.z)||W>=0&&o(E,u.x,u.z))return!1;return!0}function p(E){E.dir=-E.dir,E.lane=-E.lane,E.laneTarget=i,E.laneVel=0,E.state="turn",E.cooldown=4,E.wait=0,E.target=0,E.turns++}function y(E,b){if(E.held){E.target=0,E.speed=0,E.laneVel=0;return}if(E.cooldown>0&&(E.cooldown-=b),E.state==="turn"){if(E.target=0,E.speed<.05){let U=Math.atan2(E.fx,E.fz),F=f(U-E.heading);E.heading+=F*Math.min(1,b*3.2),Math.abs(F)<.1&&(E.heading=U,E.state="cruise")}return}let P=null,W=[...n].sort((U,F)=>Math.abs(U-i)-Math.abs(F-i)||F-U);for(let U of W)if(m(E,U)){P=U;break}if(P===null){E.cooldown<=0?p(E):E.target=0;return}E.laneTarget=P,E.target=E.cruise,Math.abs(E.laneTarget-E.lane)>.5&&!m(E,E.lane)&&(E.target=Math.min(E.target,2.4));let R=!1;for(let U of l){if(U===E)continue;let F=I(E,E.laneTarget,U);if(!F)continue;R=!0;let S=U.fx*E.fx+U.fz*E.fz,X=U.speed<.3;if(X||S<-.2){let Z=x.get(U)[w.length-1],V=X?0:Math.sign((Z.x-E.x)*E.rx+(Z.z-E.z)*E.rz)||-1,ie=null;for(let fe of W)if(!(V&&Math.sign(fe-E.lane)===V)&&m(E,fe)&&!I(E,fe,U)){ie=fe;break}ie!==null?E.laneTarget=ie:E.target=0}else S>.5?F.da>=F.db&&(E.target=Math.min(E.target,F.da<3.8?0:U.speed*.9)):(F.da>F.db||F.da===F.db&&E.route>U.route)&&(E.target=Math.min(E.target,F.da<=4?0:2))}R&&E.target===0?E.wait+=b:E.wait=0,E.wait>2.5&&E.cooldown<=0&&p(E)}let w=[0,2,4,6,8,10,12],x=new Map,A=new z;function T(E,b,P,W){d(a[E.route],E.s+E.dir*b,E.dir,c,h),W.set(c.x-h.z*P,0,c.z+h.x*P)}function I(E,b,P){let W=x.get(P),R=(t*2+.6)**2,U=P.speed<.3;for(let F=0;F<w.length;F++){T(E,w[F],b,A);let S=w[F]/Math.max(E.speed,1.5);for(let X=0;X<w.length&&!(U&&X>0);X++){let Z=w[X]/Math.max(P.speed,1.5);if(!(w[F]>4&&Math.abs(S-Z)>1.6)&&A.distanceToSquared(W[X])<R)return{da:w[F],db:w[X]}}}return null}function _(E){if(E>0){for(let b of l){g(b),x.has(b)||x.set(b,w.map(()=>new z));let P=x.get(b);w.forEach((W,R)=>T(b,b.state==="cruise"?W:0,b.lane,P[R]))}for(let b of l)y(b,E);for(let b=0;b<l.length;b++)for(let P=b+1;P<l.length;P++){let W=l[b],R=l[P],U=R.x-W.x,F=R.z-W.z,S=Math.hypot(U,F),X=t*2+.2;if(S>=X)continue;let Z=W.state==="turn"||W.speed<.3,V=R.state==="turn"||R.speed<.3,ie=Z===V?.5:Z?0:1,fe=Math.min(X-Math.max(S,.01),2.5*E),Ce=U*W.rx+F*W.rz,ze=-(U*R.rx+F*R.rz);W.lane=_t.clamp(W.lane-Math.sign(Ce||1)*fe*ie,-4.2,4.2),R.lane=_t.clamp(R.lane-Math.sign(ze||1)*fe*(1-ie),-4.2,4.2),W.target=Math.min(W.target,.5),R.target=Math.min(R.target,.5)}for(let b of l){let P=b.x,W=b.z,R=b.target>b.speed?6:9;if(b.held){b.vx=b.vz=0;continue}b.speed+=_t.clamp(b.target-b.speed,-R*E,R*E),b.speed<0&&(b.speed=0),b.s+=b.dir*b.speed*E;let U=b.laneTarget-b.lane,F=b.state==="turn"?0:Math.min(3,b.speed*.6),S=Math.sign(U)*Math.min(F,Math.sqrt(8*Math.abs(U)),Math.abs(U)/E);if(b.laneVel+=_t.clamp(S-b.laneVel,-4*E,4*E),b.lane+=b.laneVel*E,g(b),b.vx=(b.x-P)/E,b.vz=(b.z-W)/E,b.state==="cruise"){let X=b.speed>.25?Math.atan2(b.vx,b.vz):Math.atan2(b.fx,b.fz);b.heading+=f(X-b.heading)*Math.min(1,E*(b.speed>.25?8:3.2))}}}}let D=[];for(let E of l){for(let b=0;b<600&&(g(E),!(D.every(W=>Math.hypot(E.x-W.x,E.z-W.z)>t*2+6)&&m(E,E.lane)));b++)E.s+=1;E.heading=Math.atan2(E.fx,E.fz),D.push(E)}return{agents:l,step:_,hitsObstacle:v,seek(E,b){E.s=b,g(E),E.heading=Math.atan2(E.fx,E.fz)},adopt(E,b){let P=a[E.route],W=0,R=1/0;for(let U=0;U<ei;U++){let F=P.points[U],S=Math.hypot(F.x-b.x,F.z-b.z);S<R&&(R=S,W=U)}E.s=W/ei*P.length,d(P,E.s,E.dir,c,h),E.lane=(b.x-c.x)*-h.z+(b.z-c.z)*h.x,E.laneTarget=i,E.speed=0,E.laneVel=0,E.state="cruise",E.heading=b.heading,g(E)},stats:()=>({states:l.map(E=>E.state),turns:l.reduce((E,b)=>E+b.turns,0),lanes:l.map(E=>Math.round(E.lane*10)/10)})}}var fy=(s,e,t)=>Math.max(e,Math.min(t,s)),bc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),Qo=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function Ui(s,e=1){let t=s.min.map(d=>d*e),n=s.max.map(d=>d*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,o=i>r,a=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(a/Math.max(.3,l)))),h=a/c,u=[];for(let d=0;d<c;d++){let f=-a+h+d*h*2;u.push({x:(t[0]+n[0])/2+(o?f:0),z:(t[2]+n[2])/2+(o?0:f),r:Math.hypot(l,h)})}return{circles:u,minY:t[1],maxY:n[1],reach:Math.max(...u.map(d=>Math.hypot(d.x,d.z)+d.r))}}function Br(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function Mc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let o=(t-s)/r,a=(n-s)/r;return i[0]=Math.max(i[0],Math.min(o,a)),i[1]=Math.min(i[1],Math.max(o,a)),i[0]<=i[1]}function py(s,e,t,n,i,r){let o=[0,1];return Mc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,o)?o:null}function yc(s,e,t,n,i,r){let o=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>o||Math.min(i.x,r.x)-Math.max(e.x,t.x)>o||Math.min(e.z,t.z)-Math.max(i.z,r.z)>o||Math.min(i.z,r.z)-Math.max(e.z,t.z)>o)return!1;let a=py(s,e,t,n,i,r);if(!a)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(bc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(bc(i.heading,r.heading))*Math.hypot(c.x,c.z),u=Br(l,e),d=Br(l,t),f=Br(c,i),g=Br(c,r),v=u.x-f.x,m=u.z-f.z,p=d.x-g.x-v,y=d.z-g.z-m,w=fy(-(v*p+m*y)/Math.max(1e-12,p*p+y*y),...a),x=l.r+c.r+h;if((v+p*w)**2+(m+y*w)**2<x*x)return!0}return!1}function Au(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),o=Math.sin(n.heading||0),a=l=>({x:(l.x-n.x)*r-(l.z-n.z)*o,z:(l.x-n.x)*o+(l.z-n.z)*r});for(let l of s.circles){let c=a(Br(l,e)),h=a(Br(l,t)),u=l.r+.06+Math.abs(bc(e.heading,t.heading))*Math.hypot(l.x,l.z),d=[0,1];if(Mc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,d)&&Mc(c.x,h.x,n.min[0]-u,n.max[0]+u,d)&&Mc(c.z,h.z,n.min[2]-u,n.max[2]+u,d))return!0}return!1}function Sc(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,o=0,a=(g,v)=>g.id===v.id||g.ignore===v.id||v.ignore===g.id||g.enabled===!1||v.enabled===!1;function l(g,v,m,p=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let y of e.values())if(y.enabled!==!1&&!(y.owner&&(y.owner===g.ignore||y.owner===g.id))&&Au(g,v,m,y))return y.id;if(p){for(let y of s.values())if(!a(g,y)&&yc(g,v,m,y,y,y))return y.id}return null}let c=(g,v,m,p=!0)=>!l(g,v,m,p);function h(g,v,m,p=0){if(s.has(g))return s.get(g);let y={id:g,...v,...Qo(m),priority:p,enabled:!0,blocked:0};return c(y,y,y)?(s.set(g,y),y):null}function u(g,v,m=12){let p={...g,enabled:!0};for(let y=0;y<=m;y+=.5)for(let w=0;w<(y?24:1);w++){let x={...Qo(v),x:v.x+Math.cos(w*Math.PI/12)*y,z:v.z+Math.sin(w*Math.PI/12)*y};if(c(p,x,x))return x}return null}function d(g,v,m=12){let p=u(g,v,m);return p?(Object.assign(g,p),!0):!1}function f(g){let v=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let p=t.get(m.id),y=p?.next||Qo(m);return{body:m,from:Qo(m),next:y,request:p,accepted:c(m,m,y,!1)}});for(let m=0;m<=v.length;m++){let p=!1;for(let y of v)y.accepted&&y.body.follow&&v.some(w=>w.body.id===y.body.follow&&!w.accepted)&&(y.accepted=!1,p=!0);for(let y=0;y<v.length;y++)for(let w=y+1;w<v.length;w++){let x=v[y],A=v[w];if(a(x.body,A.body)||!x.accepted&&!A.accepted)continue;let T=x.accepted?x.next:x.from,I=A.accepted?A.next:A.from;if(!yc(x.body,x.from,T,A.body,A.from,I))continue;let _=Z=>Z.request&&Z.accepted&&(Math.hypot(Z.next.x-Z.from.x,Z.next.y-Z.from.y,Z.next.z-Z.from.z)>1e-8||Math.abs(bc(Z.from.heading,Z.next.heading))>1e-8),D=_(x),E=_(A);if(!D&&!E)continue;let b=x.next.x-x.from.x,P=x.next.z-x.from.z,W=A.next.x-A.from.x,R=A.next.z-A.from.z,U=b*W+P*R>.8*Math.hypot(b,P)*Math.hypot(W,R),F=D&&!yc(x.body,x.from,x.next,A.body,A.from,A.from),S=E&&!yc(x.body,x.from,x.from,A.body,A.from,A.next),X=D?E?F&&!S?A:S&&!F?x:x.body.priority!==A.body.priority?x.body.priority<A.body.priority?x:A:U?(A.from.x-x.from.x)*b+(A.from.z-x.from.z)*P>0?x:A:x.body.id>A.body.id?x:A:x:A;X.accepted=!1,p=!0}if(!p)break}for(let m of v)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=g,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of v)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=g}return{register:h,relocate:d,findFree:u,clear:c,obstruction:l,solve:f,begin(){t.clear()},propose(g,v,m){g?.enabled!==!1&&g&&t.set(g.id,{next:Qo(v),commit:m})},remove(g){s.delete(g),t.delete(g)},solid(g,v){let m=e.get(g),p={id:g,x:0,z:0,heading:0,...v};if(m&&m.x===p.x&&m.z===p.z&&m.heading===p.heading&&m.enabled===p.enabled&&m.min.every((T,I)=>T===p.min[I])&&m.max.every((T,I)=>T===p.max[I]))return;let y=Math.cos(p.heading),w=Math.sin(p.heading),x=[],A=[];for(let T of[p.min[0],p.max[0]])for(let I of[p.min[2],p.max[2]])x.push(p.x+T*y+I*w),A.push(p.z-T*w+I*y);p.world=[Math.min(...x),Math.min(...A),Math.max(...x),Math.max(...A)],e.set(g,p),o++},removeSolid(g){e.delete(g)&&o++},removeOwner(g){for(let[v,m]of e)m.owner===g&&(e.delete(v),o++)},revision:()=>o,solidClear(g){return[...s.values()].every(v=>v.enabled===!1||g.owner&&v.ignore===g.owner||!Au(v,v,v,{x:0,z:0,heading:0,...g}))},ceiling(g,v,m){let p=0;for(let y of e.values())y.enabled!==!1&&Au(g,{x:v,y:y.min[1],z:m,heading:0},{x:v,y:y.min[1],z:m,heading:0},y)&&(p=Math.max(p,y.max[1]-g.minY+1));return p},body:g=>s.get(g),neighbors(g,v=12){return[...s.values()].filter(m=>!a(g,m)&&Math.abs(m.y-g.y)<4&&Math.hypot(m.x-g.x,m.z-g.z)<v)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(g=>g.enabled!==!1).map(g=>({id:g.id,x:g.x,y:g.y,z:g.z,heading:g.heading,blocked:g.blocked,obstruction:g.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function sp(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var rp=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function op(s,e,t){let n=!1,i=0,r=!1,o=!1,a=0,l=!1,c=new dt;c.name="city-life",s.add(c);let h=t.traffic,u=[],d=ip({routes:rp,obstacles:t.obstacles||[],blocked:(ue,xe,H)=>{let K=u[ue.route],Q=K?{...K,x:xe,z:H}:null;return K?!h.clear(K,Q,Q):!1}}),f=new Set,g=new Set,v=new Set,m=[],p=new AbortController,y=[],w=[],x=ue=>(f.add(ue),ue),A=ue=>(g.add(ue),ue),T=x(new Rn(1,.013,5,64));for(let ue of e){let xe=A(new Mt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),H=new Ke(T,xe);H.rotation.x=Math.PI/2,H.position.set(ue.x,.6,ue.z),H.scale.setScalar(ue.radius*1.12),c.add(H),w.push({id:ue.id,ring:H,material:xe,state:"unknown",eventUntil:0,color:new be(7438733)})}let I=new Map,_=[],D=new Map,E=Date.now(),b=3e3,P=e.find(ue=>ue.id==="agent"),W=new z(0,0,1),R=new z,U={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(P){for(let ue of e)if(ue!==P){let xe=new z(P.x,P.height+3,P.z),H=new z(ue.x,ue.height+4,ue.z),K=xe.clone().lerp(H,.5);K.y=Math.max(xe.y,H.y)+12;let Q=new Zn(xe,K,H),ee=A(new li({color:U[ue.id],transparent:!0,opacity:0,depthWrite:!1,blending:kt,toneMapped:!1})),ae=new Ai(x(new ht().setFromPoints(Q.getPoints(48))),ee);ae.visible=!1,c.add(ae),I.set(ue.id,{curve:Q,line:ae})}}let F=x(new Rn(1,.06,5,40,Math.PI*.95));for(let ue=0;ue<12;ue++){let xe=[];for(let H=0;H<4;H++){let K=A(new Mt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:kt,toneMapped:!1})),Q=new Ke(H===3?T:F,K);Q.visible=!1,c.add(Q),xe.push(Q)}_.push({at:-1/0,waves:xe,link:null})}function S(ue,xe){if(!l||t.active?.()===!1||ue.at<E||xe-ue.at>b||ue.at>xe||!["started","succeeded","failed","sanitized","progress"].includes(ue.state))return;let H=ue.to==="agent",K=H?ue.from:ue.to;if(!H&&ue.from!=="agent"||!I.has(K))return;let Q=K+":"+H;if(ue.at-(D.get(Q)??-1/0)<450)return;D.set(Q,ue.at);let ee=_.reduce((ae,ve)=>ae.at<ve.at?ae:ve);Object.assign(ee,{at:ue.at,link:I.get(K),incoming:H,from:ue.from,to:ue.to,state:ue.state}),ee.waves.forEach(ae=>ae.material.color.setHex(ue.state==="failed"?16737881:U[K]))}function X(ue){let xe=Date.now();I.forEach(H=>{H.line.visible=!1});for(let H of _){(!ue||t.active?.()===!1)&&(H.at=-1/0);let K=(xe-H.at)/b,Q=K>=0&&K<1;if(H.waves.forEach(ae=>{ae.visible=!1}),!Q||!H.link)continue;H.link.line.visible=!0,H.link.line.material.opacity=.12*Math.sin(K*Math.PI);for(let ae=0;ae<3;ae++){let ve=(K-ae*.075)/.75;if(ve<0||ve>1)continue;let Ue=H.incoming?1-ve:ve,Se=H.waves[ae];H.link.curve.getPoint(Ue,Se.position),H.link.curve.getTangent(Ue,R),H.incoming&&R.negate(),Se.quaternion.setFromUnitVectors(W,R),Se.rotateZ(Math.PI*.525),Se.scale.setScalar(1.2+Math.sin(ve*Math.PI)*3.5),Se.material.opacity=Math.sin(ve*Math.PI)*.8,Se.visible=!0}let ee=(K-.74)/.26;if(ee>0){let ae=H.waves[3];H.link.curve.getPoint(H.incoming?0:1,ae.position),ae.rotation.set(-Math.PI/2,0,0),ae.scale.setScalar(1+ee*8),ae.material.opacity=(1-ee)*.6,ae.visible=!0}}}let Z=x(new nn(7,7)),V=A(new pt({transparent:!0,depthWrite:!1,blending:kt,uniforms:{color:{value:new be(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),ie=x(new Rs(.42,1.3,12,1,!0)),fe=A(new Mt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:kt}));rp.forEach((ue,xe)=>{let H=new dt;H.name="city-white-robot-"+(xe+1);let K=new dt;H.add(K),c.add(H);let Q=new Ke(Z,V);Q.rotation.x=-Math.PI/2,Q.position.y=.58,c.add(Q);let ee=new Ke(ie,fe);ee.rotation.z=Math.PI,ee.position.y=-.45,H.add(ee),H.visible=!1,Q.visible=!1,y.push({root:H,body:K,glow:Q,jet:ee,agent:d.agents[xe]})});function Ce(ue){ue.traverse(xe=>{if(xe.isMesh){f.add(xe.geometry);for(let H of Array.isArray(xe.material)?xe.material:[xe.material]){g.add(H);for(let K of Object.values(H))K?.isTexture&&v.add(K)}}})}function ze(){f.forEach(xe=>xe.dispose()),g.forEach(xe=>xe.dispose());let ue=new Set;v.forEach(xe=>{xe.image&&ue.add(xe.image),xe.dispose()}),ue.forEach(xe=>xe.close?.()),f.clear(),g.clear(),v.clear()}async function He(){let ue=setTimeout(()=>p.abort(),15e3);try{let xe=await fetch(t.robotURL,{signal:p.signal});if(!xe.ok)throw Error("Robot unavailable");let H=await xe.arrayBuffer();if(n)return;let{scene:K}=await new os().parseAsync(H,"");if(Ce(K),n){ze();return}let Q=new Kt().setFromObject(K),ee=Q.getSize(new z),ae=Math.max(ee.x,ee.y,ee.z);if(!Number.isFinite(ae)||ae<=0)throw Error("Invalid robot bounds");K.scale.setScalar(6/ae),K.updateMatrixWorld(!0),Q.setFromObject(K);let ve=Q.getCenter(new z);K.position.set(-ve.x,-Q.min.y,-ve.z),K.rotation.y=-Math.PI/2,K.updateMatrixWorld(!0),Q.setFromObject(K),Q.expandByScalar(.4);let Ue=Ui({min:Q.min.toArray(),max:Q.max.toArray()});K.traverse(Se=>{Se.isMesh&&(Se.castShadow=!1,Se.receiveShadow=!0)}),y.forEach((Se,je)=>{if(Se.body.add(K.clone(!0)),Se.root.visible=!0,Se.glow.visible=!0,h){let Ge=Se.agent,st=null;for(let j=0;j<600&&!st;j++)st=h.register("patrol-"+je,Ue,{x:Ge.x,y:1.6,z:Ge.z,heading:Ge.heading},1),st||d.seek(Ge,Ge.s+1);st?(u[je]=st,t.society?.patrol(st.id,st,Se.root,(j,tt)=>{Ge.held=j,tt&&d.adopt(Ge,tt)})):(Se.root.visible=!1,Se.glow.visible=!1,Ge.held=!0)}}),r=!0,me(0,!1)}catch(xe){n||(o=!0,t.onError?.(xe))}finally{clearTimeout(ue)}}function ce(ue){m.forEach(H=>{H.dispose(),g.delete(H)}),m.length=0,ue.children.find(H=>H.userData.district==="operations")?.getObjectByName("signal")?.traverse(H=>{if(!H.isMesh)return;let K=Q=>{let ee=A(Q.clone());return m.push(ee),ee};H.material=Array.isArray(H.material)?H.material.map(K):K(H.material)})}function Me(ue,xe=[]){for(let K of w){let Q=ue.find(ee=>ee.id===K.id);K.state=!Q||Q.stale?"unknown":Q.state,K.color.setHex(K.state==="error"?16730430:K.state==="running"?7400403:K.state==="unknown"?7438733:7452878),K.material.color.copy(K.color)}let H=Date.now();for(let K of[...xe].reverse())if(K.id>a&&(S(K,H),l&&K.at>=E&&t.active?.()!==!1&&t.society?.event(K),H-K.at<6e3)){let Q=w.find(ee=>ee.id===K.district);Q&&(Q.eventUntil=K.at+5e3)}a=Math.max(a,...xe.map(K=>K.id))}function me(ue,xe){if(n)return;l=xe,X(xe);let H=xe?Math.min(.1,Math.max(0,ue)):0,K=h?d.agents.map(Q=>({...Q})):null;xe&&H>0&&(i+=H,d.step(H));for(let[Q,ee]of y.entries()){let ae=ee.agent,ve=Math.hypot(ae.vx,ae.vz),Ue=(!xe||ae.held)&&u[Q]?u[Q]:ae;ee.root.position.set(Ue.x,1.6+Math.sin(i*1.6+Q*1.9)*.18,Ue.z),ee.root.rotation.y=Ue.heading;let Se=(ae.vx*ae.rx+ae.vz*ae.rz)/Math.max(1,ve);if(ee.body.rotation.z=Math.sin(i*.85+Q)*.035-Se*.08,ee.body.rotation.x=-.035+Math.sin(i*1.1+Q)*.018-Math.min(.09,ve*.015),ee.glow.position.x=Ue.x,ee.glow.position.z=Ue.z,ee.jet.scale.y=1+Math.sin(i*4+Q)*.12+ve*.04,xe&&u[Q]&&!ae.held){let je=u[Q],Ge={x:ae.x,y:1.6,z:ae.z,heading:ae.heading};ee.escape&&i<ee.escape.until?(Ge.x=je.x+ee.escape.x*H*1.8,Ge.z=je.z+ee.escape.z*H*1.8,Ge.heading=je.heading):ee.escape=null,!h.clear(je,je,Ge,!1)&&h.clear(je,je,{...Ge,heading:je.heading},!1)&&(Ge.heading=je.heading);let st=Math.hypot(Ge.x-je.x,Ge.z-je.z);h.propose(je,Ge,(j,tt)=>{if(ee.wait=j&&st>.002?0:(ee.wait||0)+H,ee.escape&&j&&d.adopt(ae,tt),ee.wait>2.5){let Je=Math.sin(tt.heading),B=Math.cos(tt.heading);for(let[M,te]of[[B,-Je],[-B,Je],[Je,B],[-Je,-B]])if(h.clear(tt,tt,{...tt,x:tt.x+M*2.5,z:tt.z+te*2.5})){ee.escape={x:M,z:te,until:i+2};break}ee.wait=0}if(j)ae.trafficWait=0,ae.heading=tt.heading;else{let Je=(ae.trafficWait||0)+H;Object.assign(ae,K[Q]),ae.speed=0,ae.vx=ae.vz=0,ae.trafficWait=Je,Je>2.5&&(ae.state!=="turn"?(ae.dir=-ae.dir,ae.lane=-ae.lane,ae.laneTarget=2.2,ae.state="turn",ae.turns++):ae.state="cruise",ae.trafficWait=0)}ee.root.position.x=tt.x,ee.root.position.z=tt.z,ee.root.rotation.y=tt.heading,ee.glow.position.x=tt.x,ee.glow.position.z=tt.z})}}for(let Q of w){let ee=xe?.5+.5*Math.sin(i*(Q.state==="error"?3.2:1.8)):.5,ae=Q.state==="error"||Q.state==="running"||Q.eventUntil>Date.now();if(Q.material.opacity=Q.state==="unknown"?.03:ae?.2+ee*.46:.065,Q.ring.scale.setScalar(e.find(ve=>ve.id===Q.id).radius*1.12*(1+(ae&&xe?ee*.035:0))),Q.id==="operations")for(let ve of m)ve.color.copy(Q.color),ve.emissive?.copy(Q.color),ve.emissiveIntensity=Q.state==="error"?1.1+ee*2.2:Q.state==="unknown"?.08:.55}}function Oe(){n||(n=!0,p.abort(),t.signal?.removeEventListener("abort",Oe),c.removeFromParent(),ze(),y.length=0,m.length=0,u.forEach(ue=>h.remove(ue.id)))}return t.signal?.addEventListener("abort",Oe,{once:!0}),t.signal?.aborted?Oe():He(),{update:me,setData:Me,attachLandmarks:ce,dispose:Oe,stats:()=>({robots:r?y.filter(ue=>ue.root.visible).length:0,robotError:o,time:i,navigation:d.stats(),positions:y.map(ue=>ue.root.position.toArray()),transmissions:_.filter(ue=>ue.waves.some(xe=>xe.visible)).map(ue=>({from:ue.from,to:ue.to,state:ue.state,age:Date.now()-ue.at})),signals:w.map(ue=>({id:ue.id,state:ue.state,intensity:ue.material.opacity}))})}}var ea='Geist, "Segoe UI", system-ui, sans-serif',wc=13,my=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},Ru={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},gy={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},xy={vertex:Ru.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},ap={vertex:Ru.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},_y={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function zr(s,e,t={}){return new pt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:kt,side:At,...t})}function vy(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function lp(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,o="";for(let c of e.split(" ")){if(i.length>=n)break;let h=o?o+" "+c:c;if(r(h)){o=h;continue}if(o&&(i.push(o),o="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let d=u.length-1;for(;d>1&&!r(u.slice(0,d));)d--;i.push(u.slice(0,d)),u=u.slice(d)}o=u}o&&i.length<n&&(i.push(o),o="");let a=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(a<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function cp(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new dt;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let o=[],a=[],l=[],c=(H,K)=>(o.push(H),a.push(K),new Ke(H,K)),h=new be(6544639),u=new be(10980351),d=new be(14218751),f={time:{value:0}},g=!1,v=0,m=[],p=[],y=0,w="none",x=!1,A=c(new nn(11,11),zr(xy,{time:f.time,strength:{value:.9},color:{value:h}}));A.rotation.x=-Math.PI/2,A.position.y=.18,r.add(A);let T=c(new ji(7.4,2.3,wc,56,1,!0),zr(gy,{time:f.time,strength:{value:.32},base:{value:h},top:{value:u}}));T.position.y=wc/2+.3,r.add(T);let I=c(new Rn(7.5,.055,6,128),zr(ap,{time:f.time,strength:{value:1.6},color:{value:h}}));I.rotation.x=Math.PI/2,I.position.y=wc+.3,r.add(I);let _=c(new Rn(2.6,.05,6,96),zr(ap,{time:f.time,strength:{value:2},color:{value:d}}));_.rotation.x=Math.PI/2,_.position.y=.55,r.add(_);let D=new Mr(1.35,1),E=new xo(D),b=new li({color:10217983,transparent:!0,opacity:.75,blending:kt,depthWrite:!1,toneMapped:!1}),P=new Ts(E,b);P.position.y=4.6,r.add(P),o.push(D,E),a.push(b);let W=new Mt({color:4175871,transparent:!0,opacity:.12,blending:kt,depthWrite:!1,toneMapped:!1}),R=c(new Mr(1.1,1),W);P.add(R);let U=160,F=new Float32Array(U);for(let H=0;H<U;H++)F[H]=H*.618033988749895%1;let S=new ht;S.setAttribute("position",new gt(new Float32Array(U*3),3)),S.setAttribute("seed",new gt(F,1)),S.boundingSphere=new an(new z(0,wc/2,0),12);let X=zr(_y,{time:f.time,pointScale:{value:800},color:{value:d},strength:{value:.9}}),Z=new ln(S,X);Z.frustumCulled=!1,r.add(Z),o.push(S),a.push(X);let V=new Kn(6478079,26,46,2);V.position.y=6,r.add(V);function ie(H,K,Q,ee){let ae=document.createElement("canvas");ae.width=Q,ae.height=ee;let ve=ae.getContext("2d"),Ue=new As(ae);Ue.colorSpace=Ut,Ue.generateMipmaps=!1,Ue.minFilter=Ft,l.push(Ue);let Se=zr(Ru,{map:{value:Ue},time:f.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new be(16777215)}},{side:Tn}),je=c(new nn(H,K),Se);return{canvas:ae,context:ve,texture:Ue,material:Se,mesh:je,text:"",targetFade:1,next:0}}let fe=ie(17.5,8.55,1024,500),Ce=[ie(8.6,2.1,640,156),ie(8.6,2.1,640,156)];fe.mesh.position.y=10.4,r.add(fe.mesh),Ce.forEach((H,K)=>{H.mesh.position.y=K?14.4:5.9,H.orbit=K?-.22:.27,H.angle=K*2.3,r.add(H.mesh)});function ze(H,K,Q){let{context:ee,canvas:ae}=fe,ve=ae.width,Ue=ae.height;ee.clearRect(0,0,ve,Ue),ee.fillStyle="rgba(48,150,214,0.14)",ee.beginPath(),ee.roundRect(14,14,ve-28,Ue-28,22),ee.fill(),ee.strokeStyle="rgba(150,230,255,0.85)",ee.lineWidth=3;for(let[Se,je,Ge,st]of[[18,18,1,1],[ve-18,18,-1,1],[18,Ue-18,1,-1],[ve-18,Ue-18,-1,-1]])ee.beginPath(),ee.moveTo(Se,je+st*42),ee.lineTo(Se,je),ee.lineTo(Se+Ge*42,je),ee.stroke();ee.fillStyle="rgba(150,230,255,0.55)",ee.fillRect(48,108,ve-96,2),ee.font="600 27px "+ea,ee.textBaseline="middle",ee.fillStyle="rgba(160,232,255,0.92)",ee.textAlign="left",ee.fillText("\u258C "+i+(Q?"  \xB7  "+String(K+1).padStart(2,"0")+" / "+String(Q).padStart(2,"0"):""),50,72),ee.textAlign="right",ee.font="500 25px "+ea,ee.fillStyle="rgba(190,150,255,0.85)",ee.fillText("0x"+my(H||i).toUpperCase(),ve-52,72),ee.textAlign="left",ee.shadowColor="rgba(120,225,255,0.9)",ee.shadowBlur=16,H?(ee.font="600 54px "+ea,ee.fillStyle="rgba(232,250,255,0.97)",lp(ee,H,ve-110,4).forEach((je,Ge)=>ee.fillText(je,54,172+Ge*72))):(ee.font="600 40px "+ea,ee.fillStyle="rgba(180,235,255,0.7)",ee.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),ee.shadowBlur=0,fe.texture.needsUpdate=!0}function He(H,K){let{context:Q,canvas:ee}=H,ae=ee.width,ve=ee.height;Q.clearRect(0,0,ae,ve),Q.fillStyle="rgba(48,150,214,0.12)",Q.beginPath(),Q.roundRect(6,6,ae-12,ve-12,14),Q.fill(),Q.fillStyle="rgba(190,150,255,0.8)",Q.fillRect(20,26,6,ve-52),Q.font="600 42px "+ea,Q.textBaseline="middle",Q.textAlign="left",Q.shadowColor="rgba(160,140,255,0.9)",Q.shadowBlur=12,Q.fillStyle="rgba(236,240,255,0.95)",Q.fillText(K?lp(Q,K,ae-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,ve/2),Q.shadowBlur=0,H.texture.needsUpdate=!0}function ce(){return m.length?(y>=p.length&&(p=m.map((H,K)=>K).sort(()=>Math.random()-.5),y=0),m[p[y++]]):""}let Me=.8,me=[3.5,6.5];function Oe(H){let K=ce();fe.text=K,v++,ze(K,m.indexOf(K),m.length),fe.material.uniforms.glitch.value=H?1:0,fe.material.uniforms.fade.value=H?.35:1,Me=g?12:4.5+Math.random()*3}function ue(H,K){let Q=Ce[H],ee=ce();Q.text=ee,He(Q,ee.length>46?ee.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":ee),Q.material.uniforms.glitch.value=K?.7:0,Q.material.uniforms.fade.value=K?.3:1,me[H]=g?15:6+Math.random()*4}ze("",0,0),Ce.forEach(H=>He(H,"")),document.fonts?.ready?.then(()=>{x||(ze(fe.text,m.indexOf(fe.text),m.length),Ce.forEach(H=>He(H,H.text)))});let xe=new z;return{group:r,setTexts(H,K="live"){let Q=vy(H),ee=Q.length!==m.length||Q.some((ae,ve)=>ae!==m[ve]);m=Q,w=Q.length?K:"none",ee&&(p=[],y=0,(!fe.text||!m.includes(fe.text))&&(Me=Math.min(Me,.6)))},setPointScale(H){X.uniforms.pointScale.value=H},setReducedMotion(H){g=!!H},update(H,K,Q){if(x)return;let ee=Q&&!g,ae=ee?Math.min(.1,Math.max(0,H)):0;f.time.value+=ae,Me-=H,Me<=0&&Oe(ee),me.forEach((ve,Ue)=>{me[Ue]=ve-H,me[Ue]<=0&&ue(Ue,ee)});for(let ve of[fe,...Ce]){let Ue=ve.material.uniforms;Ue.glitch.value=Math.max(0,Ue.glitch.value-H*2.4),Ue.fade.value=Math.min(1,Ue.fade.value+H*1.8),ve.mesh.quaternion.copy(K.quaternion)}Ce.forEach(ve=>{ve.angle+=ve.orbit*ae,ve.mesh.position.x=Math.cos(ve.angle)*6.4,ve.mesh.position.z=Math.sin(ve.angle)*6.4,xe.copy(ve.mesh.position).add(r.position).sub(K.position).normalize();let Ue=-(xe.x*Math.cos(ve.angle)+xe.z*Math.sin(ve.angle));ve.material.uniforms.gain.value=1.35*_t.clamp(.35+Ue*.9,.15,1.2)}),fe.mesh.position.y=9.6+Math.sin(f.time.value*.9)*.22,P.rotation.y+=ae*.7,P.rotation.x+=ae*.31,R.rotation.y-=ae*1.1,I.rotation.z+=ae*.18,_.rotation.z-=ae*.42,V.intensity=26+Math.sin(f.time.value*2.1)*6+fe.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:w,switches:v,shown:fe.text?m.indexOf(fe.text):-1}),dispose(){x||(x=!0,r.removeFromParent(),o.forEach(H=>H.dispose()),a.forEach(H=>H.dispose()),l.forEach(H=>H.dispose()),V.dispose())}}}var yy=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function hp(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new Re(.5,.5)},shaftColor:{value:new be(12376319)},shadowTint:{value:new be(6686)},highlightTint:{value:new be(1444864)}},t=new Fr(new pt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:yy}));t.enabled=!1;let n=new z,i=new be(16761992),r=new be(12376319),o=!1;return{pass:t,setCinematic(a){o=!!a},update(a,l,c,{day:h,dusk:u,cloud:d,animated:f}){e.aspect.value=l.aspect;let g=o?1:0;e.letterbox.value=f?e.letterbox.value+(g-e.letterbox.value)*Math.min(1,a*2.2):g,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let v=(n.x+1)/2,m=(n.y+1)/2,p=n.z<1?Math.max(0,1-Math.max(0,Math.abs(v-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(v,m),e.shafts.value=p*(.06+u*.14+(1-h)*.03)*(1-d*.75),e.shaftColor.value.copy(r).lerp(i,u),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+u*.03,.01+u*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var Pu=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,Cu="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",My="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",by=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${Pu}${My}
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
gl_FragColor=vec4(col,1.0);}`,up={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},dp={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${Pu}
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
}`},Sy=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Pu}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,fp={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},wy=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,Ey=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,pp={vertex:"attribute float lift;varying float vLift;void main(){vLift=lift;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:"uniform vec3 haze;uniform float shade;varying float vLift;void main(){gl_FragColor=vec4(haze*mix(1.0,shade,smoothstep(0.0,0.6,vLift)),1.0);}"},mp={vertex:`attribute float phase;uniform float time,pointScale,strength;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=strength*(0.75+0.25*sin(time*(0.7+fract(phase)*1.3)+phase));gl_PointSize=clamp(2.2*pointScale/max(1.0,-mv.z),1.5,4.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(1.4,0.95,0.55)*pow(1.0-d,1.5)*vA,1.0);}"},gp={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},xp={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function _p(s,e={}){let t=[],n=[],i=new dt;i.name="city-atmosphere",s.add(i);let r=(G,le)=>(t.push(G),n.push(le),new Ke(G,le)),o={value:0},a={value:900},l={value:0},c={value:0},h={value:.25},u={value:0},d={value:5},f=new be(4860440),g=(e.sunDirection||new z(-70,145,85)).clone().normalize(),v=new be(330010),m=new be(1323596),p=new be(8011050),y=new be(s.fog?.color||528926),w=(G,le,se,ge={})=>new pt({uniforms:se,vertexShader:G,fragmentShader:le,...ge}),x={transparent:!0,depthWrite:!1,blending:kt},A=r(new hi(1500,48,24),w(Cu,by,{time:o,day:l,dusk:c,cloud:h,flash:u,octaves:d,glow:{value:f},zenith:{value:v},horizon:{value:m},haze:{value:y},warm:{value:p},sunDir:{value:g}},{side:jt,depthWrite:!1,fog:!1}));A.frustumCulled=!1,A.renderOrder=-10,i.add(A);let T=1400,I=new Float32Array(T*3),_=new Float32Array(T),D=new Float32Array(T),E=new Float32Array(T),b=12345,P=()=>(b=Math.imul(b,1664525)+1013904223>>>0)/4294967296;for(let G=0;G<T;G++){let le=P()*Math.PI*2,se=.03+P()*.97,ge=Math.sqrt(1-se*se);I.set([Math.cos(le)*ge*1450,se*1450,Math.sin(le)*ge*1450],G*3),_[G]=P()*Math.PI*2,D[G]=.4+P()*1.6,E[G]=8e-4+P()*P()*.0022}let W=new ht;W.setAttribute("position",new gt(I,3)),W.setAttribute("phase",new gt(_,1)),W.setAttribute("speed",new gt(D,1)),W.setAttribute("size",new gt(E,1));let R=w(up.vertex,up.fragment,{time:o,pointScale:a,cloud:h},{...x,fog:!1}),U=new ln(W,R);U.frustumCulled=!1,U.renderOrder=-9,i.add(U),t.push(W),n.push(R);let F=r(new nn(2800,2800),w(dp.vertex,dp.fragment,{time:o,day:l,glow:{value:f},island:{value:new xt(0,-7,85,87)},deep:{value:new be(398368)},shallow:{value:new be(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:g},sunColor:{value:new be(14676223)},fogColor:{value:new be},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));F.rotation.x=-Math.PI/2,F.position.y=-3.2,i.add(F);let S=r(new nn(1900,1900),w(Cu,Sy,{time:o,strength:{value:.55},color:{value:new be(1717320)},island:{value:new xt(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));S.rotation.x=-Math.PI/2,S.position.y=-2.4,i.add(S);let X=500,Z=new Float32Array(X*3),V=new Float32Array(X*3);for(let G=0;G<X;G++)Z.set([(P()-.5)*180,2+P()*P()*75,-95+P()*170],G*3),V.set([P(),P(),P()],G*3);let ie=new ht;ie.setAttribute("position",new gt(Z,3)),ie.setAttribute("seed",new gt(V,3));let fe=w(fp.vertex,fp.fragment,{time:o,pointScale:a,color:{value:new be(10475775)},strength:{value:.32}},x),Ce=new ln(ie,fe);Ce.frustumCulled=!1,i.add(Ce),t.push(ie),n.push(fe);let ze=new dt;ze.position.set(0,86.5,-12),i.add(ze);let He=w(Cu,wy,{strength:{value:.45},color:{value:new be(9430783)}},{...x,side:At}),ce=new nn(100,5.2);ce.translate(50,0,0),t.push(ce),n.push(He);let Me=new Ke(ce,He),me=new Ke(ce,He);me.rotation.x=Math.PI/2;let Oe=new dt;Oe.add(Me,me),Oe.rotation.z=-.07,ze.add(Oe);let ue=new Mt({color:12579583,toneMapped:!1}),xe=r(new hi(.9,16,12),ue);ze.add(xe);let H=(e.lamps||[]).slice(0,64),K=w("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",Ey,{color:{value:new be(16767392)},strength:{value:.3}},{...x,side:At}),Q=new Rs(2.7,6.1,18,1,!0),ee=new gn(Q,K,Math.max(1,H.length));t.push(Q),n.push(K);let ae=new bt;H.forEach((G,le)=>{ae.position.set(G.x+1.9*Math.cos(G.angle||0),3.35,G.z-1.9*Math.sin(G.angle||0)),ae.updateMatrix(),ee.setMatrixAt(le,ae.matrix)}),ee.count=H.length,ee.instanceMatrix.needsUpdate=!0,i.add(ee);let ve=[],Ue=[],Se=[],je=[],Ge=[],st=360,j=G=>Math.max(0,.45+.3*Math.sin(G*5+1.3)+.2*Math.sin(G*13+.4)+.12*Math.sin(G*31+2.1))*(.35+.65*Math.max(0,Math.sin(G*2+.6)));for(let G=0;G<=st;G++){let le=Math.PI*.8+Math.PI*1.45*G/st,se=1180+40*Math.sin(le*7),ge=6+74*j(le);if(ve.push(Math.cos(le)*se,-3.5,Math.sin(le)*se,Math.cos(le)*se,ge,Math.sin(le)*se),Ue.push(0,ge/80),G<st){let Ae=G*2;Se.push(Ae,Ae+1,Ae+2,Ae+1,Ae+3,Ae+2)}}for(let G=0;G<90;G++){let le=Math.PI*.82+Math.PI*1.41*P(),se=1170+40*Math.sin(le*7);j(le)<.08||(je.push(Math.cos(le)*se,1+P()*P()*28,Math.sin(le)*se),Ge.push(P()*20))}let tt=new ht;tt.setAttribute("position",new et(ve,3)),tt.setAttribute("lift",new et(Ue,1)),tt.setIndex(Se);let Je={value:.8},B=r(tt,w(pp.vertex,pp.fragment,{haze:{value:y},shade:Je},{fog:!1,side:At}));B.frustumCulled=!1,B.renderOrder=-8,i.add(B);let M=new ht;M.setAttribute("position",new et(je,3)),M.setAttribute("phase",new et(Ge,1));let te={value:0},he=w(mp.vertex,mp.fragment,{time:o,pointScale:a,strength:te},{...x,fog:!1}),de=new ln(M,he);de.frustumCulled=!1,de.renderOrder=-7,i.add(de),t.push(M),n.push(he);let Te=w(gp.vertex,gp.fragment,{color:{value:new be(11128063)},strength:{value:0}},{...x,side:At,fog:!1}),Le=new ji(11,.7,280,20,1,!0);Le.translate(0,140,0),t.push(Le),n.push(Te);let pe=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([G,le,se])=>{let ge=new Ke(Le,Te);return ge.position.set(G,-3,le),ge.rotation.order="YXZ",ge.userData.offset=se,ge.frustumCulled=!1,i.add(ge),ge}),_e=96,De=new Float32Array(_e*3),We=new Float32Array(_e);for(let G=0;G<_e;G++)We[G]=P()*Math.PI*2;let Pe=new ht;Pe.setAttribute("position",new gt(De,3)),Pe.setAttribute("phase",new gt(We,1)),Pe.setDrawRange(0,0);let Fe=w(xp.vertex,xp.fragment,{time:o,pointScale:a},{...x,fog:!1}),O=new ln(Pe,Fe);O.frustumCulled=!1,i.add(O),t.push(Pe),n.push(Fe);let q=hp(o),Y=q.pass,N=!1,L="high",k=0;return{post:Y,sea:F,setLighting(G,le,se){g.copy(se).normalize(),l.value=G,c.value=Math.min(1,le)*(1-G),v.set(330010).lerp(new be(3108776),G).lerp(new be(1909062),c.value*.8),m.set(1323596).lerp(new be(11126484),G).lerp(new be(12609598),c.value*.85),p.set(le>.1?16747077:8011050),f.set(4860440).lerp(new be(7027234),c.value),y.copy(s.fog.color),U.visible=G<.35,F.material.uniforms.sky.value.copy(m).lerp(v,c.value*.65).multiplyScalar(.7),F.material.uniforms.deep.value.set(398368).lerp(new be(669270),G),F.material.uniforms.shallow.value.set(930640).lerp(new be(1929112),G),S.material.uniforms.strength.value=.55*(1-G*.85),Je.value=.82-.22*(1-G)-.2*c.value,te.value=Math.max(0,1-G*1.6)*(1-c.value*.5),F.material.uniforms.sunColor.value.set(14676223).lerp(new be(16752736),c.value)},setWeather(G){h.value=G==="rain"?.95:G==="fog"?.7:.25},setFlash(G){u.value=Math.max(0,Math.min(1,G))},setCinematic(G){q.setCinematic(G)},setAviation(G){let le=Math.min(_e,G.length);for(let se=0;se<le;se++)De.set([G[se].x,G[se].y,G[se].z],se*3);Pe.attributes.position.needsUpdate=!0,Pe.setDrawRange(0,le)},setTier(G){L=G;let le=L!=="low";S.visible=le,Ce.visible=le,ee.visible=le,pe.forEach(se=>{se.visible=le}),d.value=L==="low"?3:L==="medium"?4:5,Y.enabled=L==="high"||L==="ultra"},setBusy(G){N=!!G},setPointScale(G){a.value=G},update(G,le,se,ge){let Ae=ge?Math.min(.1,Math.max(0,G)):0;o.value+=Ae,s.fog&&(F.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(F.material.uniforms.fogDensity.value=s.fog.density)),F.position.x=se.position.x,F.position.z=se.position.z;let at=N?1.15:.32,Qe=ze.userData.rate??at;ze.userData.rate=Qe+(at-Qe)*Math.min(1,Ae*1.5),Oe.rotation.y+=Ae*ze.userData.rate,He.uniforms.strength.value=N?.85:.45,xe.scale.setScalar(1+Math.sin(o.value*(N?6:2.2))*.18),k+=Ae;let Lt=1-l.value;Te.uniforms.strength.value=Lt*Lt*.075*(1-c.value*.7)*(1-h.value*.45),pe.forEach((rt,Nt)=>{let Ot=rt.userData.offset;rt.rotation.y=Math.PI+Math.sin(k*(.11+Nt*.025)+Ot)*.95,rt.rotation.x=.22+.14*Math.sin(k*.13+Ot*1.7)}),Y.enabled&&q.update(Ae,se,g,{day:l.value,dusk:c.value,cloud:h.value,animated:ge})},stats:()=>({tier:L,busy:N,stars:T,dust:X,lamps:H.length,post:Y.enabled,coast:Se.length/6,shoreLights:Ge.length,clouds:+h.value.toFixed(2),searchlights:pe.length,aviation:Pe.drawRange.count,cinema:q.stats()}),dispose(){i.removeFromParent(),ee.dispose(),t.forEach(G=>G.dispose()),n.forEach(G=>G.dispose()),q.dispose()}}}var vp=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function yp(s,e={}){let t=new dt;t.name="city-drones",s.add(t);let n=[],i=[],r=[],o=new z,a=new z,l=new z,c=new hi(.16,8,6);n.push(c);let h=x=>{let A=new Mt({color:x,toneMapped:!1});return i.push(A),A},u=h(16730684),d=h(5046154),f=h(16777215);[...vp,...vp].forEach((x,A)=>{let T=new vr(x.points.map(E=>new z(E[0],E[1]+(A>=3?8:0),E[2])),!0,"centripetal",.6),I=new dt;I.name="city-drone-"+(A+1),I.visible=!1;let _=new dt;I.add(_);let D=[new Ke(c,u),new Ke(c,d),new Ke(c,f)];D[0].position.set(-2.6,.8,0),D[1].position.set(2.6,.8,0),D[2].position.set(0,2.1,-.4),I.add(...D),t.add(I),r.push({root:I,body:_,lights:D,curve:T,length:T.getLength(),speed:x.speed,t:A*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let g=!0,v="high",m=!1,p=new Map,y=x=>{if(!p.has(x)){let A=x.clone();e.surfaces?.apply(A,"object",x.name),p.set(x,A)}return p.get(x)};function w(x,A){let{curve:T,root:I}=x,_=x.t;if(x.t=(x.t+A*x.speed*x.dir/x.length+1)%1,T.getPointAt(x.t,I.position),T.getTangentAt(x.t,o),o.multiplyScalar(x.dir),x.collider&&e.traffic){let b=x.collider,P=e.traffic;T.getPointAt((x.t+x.dir*20/x.length+1)%1,l),x.avoidTime=Math.max(0,(x.avoidTime||0)-A);let W=Math.max(I.position.y,P.ceiling(b,I.position.x,I.position.z),P.ceiling(b,l.x,l.z),x.avoidTime?x.avoidHeight:0);I.position.y=b.y+_t.clamp(W-b.y,-A*5,A*7);let R=Math.atan2(o.x,o.z),U=b.heading+Math.atan2(Math.sin(R-b.heading),Math.cos(R-b.heading))*Math.min(1,A*3),F={...I.position,heading:U};if(x.obstruction=P.obstruction(b,b,F),x.obstruction){let S=P.body(x.obstruction),X=x.obstruction.startsWith("drone-")&&(Math.abs(b.y-S.y)>1?b.y<S.y:b.id<S.id);X||(x.avoidHeight=Math.min(170,b.y+8),x.avoidTime=8),F={...b,y:X?b.y:Math.min(170,b.y+A*5)},x.t=_}P.propose(b,F,(S,X)=>{S?x.waiting=0:(x.t=_,x.waiting+=A,x.waiting>2.5&&(x.dir=-x.dir,x.waiting=0)),I.position.set(X.x,X.y,X.z),I.rotation.set(0,X.heading,0)})}T.getTangentAt((x.t+.015)%1,a),x.collider||I.rotation.set(0,Math.atan2(o.x,o.z),0);let D=Math.atan2(a.x,a.z)-Math.atan2(o.x,o.z),E=Math.atan2(Math.sin(D),Math.cos(D));x.roll+=(_t.clamp(E*12,-.55,.55)-x.roll)*(A>0?Math.min(1,A*3):1),x.body.rotation.z=x.roll}return{setTier(x){v=x},setTemplate(x){if(!x)return;m=!0;let A=p;p=new Map;for(let[T,I]of r.entries()){I.body.clear(),I.rotors.length=0;let _=x.clone(!0);_.scale.setScalar(1.9),_.updateMatrixWorld(!0);let D=new Kt().setFromObject(_);if(D.expandByScalar(.5),D.min.y-=1,D.max.y+=1,_.traverse(E=>{E.isMesh&&(E.castShadow=!1,E.receiveShadow=!1,E.material=Array.isArray(E.material)?E.material.map(y):y(E.material)),/^rotor_/.test(E.name)&&I.rotors.push(E)}),I.body.add(_),I.root.visible=T<(v==="low"?2:v==="medium"?3:6),e.traffic&&!I.collider){w(I,0);let E=Ui({min:D.min.toArray(),max:D.max.toArray()});I.root.position.y=Math.max(I.root.position.y,e.traffic.ceiling(E,I.root.position.x,I.root.position.z)),I.collider=e.traffic.register("drone-"+T,E,{...I.root.position,heading:I.root.rotation.y},0),I.collider||(I.root.visible=!1)}}A.forEach(T=>T.dispose())},update(x,A,T){g=T;let I=T?Math.min(.1,Math.max(0,x)):0;r.forEach((_,D)=>{if(_.root.visible=m&&D<(v==="low"?2:v==="medium"?3:6)&&(!e.traffic||!!_.collider),_.collider&&(_.root.visible&&!_.collider.enabled&&!e.traffic.relocate(_.collider,_.collider)&&(_.root.visible=!1),_.collider.enabled=_.root.visible),!_.root.visible)return;w(_,I),_.rotors.forEach((b,P)=>{b.rotation.y+=I*(P%2?-46:46)});let E=Math.sin(A*5+D*1.7);_.lights[0].visible=_.lights[1].visible=!T||E>-.2,_.lights[2].visible=!T||E>.93,_.lights[2].scale.setScalar(T?1.6:1)})},stats:()=>({drones:r.filter(x=>x.root.visible).length,animated:g,positions:r.map(x=>x.root.position.toArray().map(A=>Math.round(A))),obstructions:r.map(x=>x.obstruction)}),dispose(){t.removeFromParent(),n.forEach(x=>x.dispose()),i.forEach(x=>x.dispose()),p.forEach(x=>x.dispose()),r.forEach(x=>{x.collider&&e.traffic.remove(x.collider.id)}),r.length=0}}}var bn={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80},ut={ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16},hn=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),Hn=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],Iu=[[-67,-77],[-67,59],[67,59],[67,-77]],Mp=(s,e)=>{let t=s*Math.PI/180;return{x:Math.sin(t)*e,z:-12-Math.cos(t)*e,angle:Math.PI-t}},St={x:0,z:-12,floor:72.2,inner:4.2,outer:8,bridge:1,lift:{x:9.8,z:-12,base:.88,cab:.03,travel:71.32},telescopes:[45,135,225,315].map(s=>({id:"deck-"+s,...Mp(s,7.25)})),benches:[0,180,270].map(s=>Mp(s,6.6))};function Sp(s,e,t){let n=Math.hypot(s-St.x,e-St.z);return n>=St.inner&&n<=St.outer?!0:t&&s>=St.outer-.5&&s<=St.lift.x+1&&Math.abs(e-St.lift.z)<St.bridge}var _i={x:78,z:30},Ec=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function bp(s,e,t=bn.halfWidth){return s>=bn.minX&&s<=bn.maxX&&e>=bn.minZ&&e<=bn.maxZ&&(bn.xs.some(n=>Math.abs(s-n)<=t)||bn.zs.some(n=>Math.abs(e-n)<=t))}function wp(s,e){return bp(s,e)?bp(s,e,bn.laneHalfWidth)?ut.road:ut.pavement:ut.ground}function Ty(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function Ep(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function Tp(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,o)=>{r>n&&o>i&&s.push({x:(n+r)/2,z:(i+o)/2,sx:(r-n)/4,sz:(o-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,o,a,l]=[n-2,i-2,n+2,i+2],[c,h,u,d]=[Math.max(r,e[0]),Math.max(o,e[1]),Math.min(a,e[2]),Math.min(l,e[3])];if(c>=u||h>=d){t(r,o,a,l);continue}t(r,o,c,l),t(u,o,a,l),t(c,o,u,h),t(c,d,u,l)}return s}function Lu(s,e,t,n){return Ty(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Du={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},Ay={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]},Nu={agent:[[5.9,8,13],[5.4,13,29],[4.7,29,42.5],[4.2,42.5,61.5],[3.6,61.5,72.2],[2.6,72.2,87]]};function Ap(s,e,t){for(let n of t){let i=Ay[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function Ry(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Du[n.id]||[]).some(([o,a,l,c])=>i>o&&i<l&&r>a&&r<c))return!0}return Ec.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function Bs(s,e){return hn.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function as(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(Bs(s,e))return ut.room;for(let t of Hn){let n=!!t.angle&&Math.abs(t.angle)!==Math.PI;if(Math.abs(s-t.platformX)<(n?2:4.5)&&Math.abs(e-t.platformZ)<(n?4.5:2))return ut.pavement+.3}return Math.max(wp(s,e),e>=74&&e<=80&&s>=-80&&s<=80?ut.quay:ut.ground)}function Rp(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=Bs(s,e),o=Bs(t.x,t.z);if(r||o){let a=r||o;return!(r?.id!==o?.id&&(Math.abs(s-a.x)>1.15||Math.abs(t.x-a.x)>1.15||Math.min((e-a.doorZ)*a.front,(t.z-a.doorZ)*a.front)<-.9||!n(a.id))||r&&(Math.abs(s-a.x)>5.55||(e-a.z)*a.front<-5.55))}return!Ry(s,e,i)}function Tc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var ls=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}];function Cp(s,e){let t=[],n=new Map,i=-1;for(let o of[-73,-61,-24,-12,12,24,61,73])for(let a of[-83,-71,-38,-26,7,19,53,65])t.push({x:o,z:a,y:e(o,a),heading:0});function r(o,a){i!==s.revision()&&(n.clear(),i=s.revision());let l=o.circles.map(y=>[y.x,y.z,y.r].join(",")).join(";")+":"+o.maxY;if(!n.has(l)){let y=t.filter(x=>s.clear(o,x,x,!1)),w=y.map(()=>[]);for(let x=0;x<y.length;x++)for(let A=x+1;A<y.length;A++){let T=y[x],I=y[A],_=Math.hypot(T.x-I.x,T.z-I.z);_<48&&s.clear(o,T,I,!1)&&(w[x].push([A,_]),w[A].push([x,_]))}n.set(l,{nodes:y,edges:w})}let c=n.get(l),h=[...c.nodes,{x:o.x,y:o.y,z:o.z,heading:o.heading},{...a,heading:0}],u=h.length-2,d=h.length-1,f=c.edges.map(y=>[...y]);f.push([],[]);for(let y of[u,d])for(let w=0;w<y;w++){let x=h[y],A=h[w],T=Math.hypot(x.x-A.x,x.z-A.z);(T<48||w===u)&&s.clear(o,x,A,!1)&&(f[y].push([w,T]),f[w].push([y,T]))}let g=h.map(()=>1/0),v=h.map(()=>-1),m=new Set(h.map((y,w)=>w));for(g[u]=0;m.size;){let y=-1;for(let w of m)(y<0||g[w]<g[y])&&(y=w);if(!Number.isFinite(g[y])||y===d)break;m.delete(y);for(let[w,x]of f[y])g[y]+x<g[w]&&(g[w]=g[y]+x,v[w]=y)}if(!Number.isFinite(g[d]))return[];let p=[];for(let y=d;y!==u;y=v[y]){if(y<0)return[];p.unshift(h[y])}return p}return{route:r,points:t}}function Pp(s){let e=new dt;e.name="city-conversations",s.add(e);let t=new Rn(1,.025,5,40,Math.PI*1.45),n=new Cs(.92,1,48),i=new z(0,0,1),r=new z,o=[],a=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let u=new Mt({color:8445392,transparent:!0,opacity:0,blending:kt,depthWrite:!1,side:At,toneMapped:!1}),d=new Ke(h===3?n:t,u);d.visible=!1,e.add(d),c.push(d)}o.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let u=o.find(d=>d.age>=2.4);return u?(Object.assign(u,{from:l,to:c,age:0,source:h}),a++,u.meshes.forEach(d=>d.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of o){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let u=h.from,d=h.to,f=d.x-u.x,g=d.z-u.z;r.set(f,d.y+Math.min(2,d.maxY*.7)-(u.y+Math.min(2,u.maxY*.7)),g).normalize();for(let m=0;m<3;m++){let p=(h.age-m*.18)/1.55;if(p<0||p>1)continue;let y=h.meshes[m];y.visible=!0,y.position.set(u.x+f*p,_t.lerp(u.y+Math.min(2,u.maxY*.7),d.y+Math.min(2,d.maxY*.7),p)+Math.sin(p*Math.PI)*.65,u.z+g*p),y.quaternion.setFromUnitVectors(i,r),y.rotateZ(m*.65),y.scale.setScalar(.25+Math.sin(p*Math.PI)*.65),y.material.opacity=Math.sin(Math.PI*p)*.75}let v=(h.age-1.55)/.85;if(v>0){let m=h.meshes[3];m.visible=!0,m.position.set(d.x,d.y+Math.min(2,d.maxY*.7),d.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-v)+.12),m.material.opacity=Math.sin(v*Math.PI)*.85}}},clear(){o.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:a,active:o.filter(l=>l.age<2.4).length,sources:o.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),o.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var Uu=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5);function Ip(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:o}){let a=[],l=new Map,c=[],h=[],u=Cp(e,n),d=Pp(s),f=0,g=null,v=!1,m="high",p=!1,y=0,w="",x=()=>m==="low"?3:m==="medium"?10:19;function A(R){R.slot&&(R.slot.owner=null,R.slot=null),R.path=[],R.goal=null}function T(R){return R?{id:R.id,role:R.role,state:R.state,source:R.source||"ambient",guided:!!R.guide}:null}function I(R){g=R?.id||null,w=R?.state||"",i?.(T(R))}function _(R,U,F){let S="resident-"+U,X=Ui(F,1.15),Z=["courier","technician","archivist"][U%3],V=null;X.circles=[{x:0,z:0,r:X.reach}];let fe=[...c.filter(ze=>ze.role===Z).map(ze=>({x:ze.x,z:ze.z+2.2,y:n(ze.x,ze.z+2.2),heading:Math.PI})),...u.points.map((ze,He)=>u.points[(U*7+He)%u.points.length])];for(let ze of fe)if(V=e.register(S,X,ze,1),V)break;if(!V)return R.node.visible=!1,null;let Ce={...R,id:S,index:U,body:V,role:Z,state:"idle",path:[],until:U*.3,trip:U,cooldown:8+U*.7,source:"ambient",carrying:!1,parcels:[]};return R.node.traverse(ze=>{ze.isMesh&&/parcel/.test(ze.name)&&(Ce.parcels.push(ze),ze.visible=!1)}),a.push(Ce),R.node.position.set(V.x,V.y,V.z),Ce}function D(R,U,F,S){let X={id:R,body:U,node:F,pause:S,role:"patrol",state:"patrol",cooldown:7+a.length,path:[]};return a.push(X),X}function E(R){A(R),R.source="ambient",R.state="idle",R.until=f+2;let U=c.filter(F=>!F.owner&&(R.role==="courier"?F.place===(R.carrying?"meeting-charge":"parcel-sorter"):F.role===R.role||F.place==="meeting-charge"));for(let F=0;F<U.length;F++){let S=U[(R.trip+F)%U.length],X=u.route(R.body,S);if(X.length){S.owner=R.id,R.slot=S,R.path=X,R.goal=S,R.trip++,R.state=R.carrying?"carry":"walk";return}}for(let F=0;F<8;F++){let S=u.points[(R.trip++*13+R.index)%u.points.length],X=u.route(R.body,S);if(X.length){R.path=X,R.state="walk";return}}}function b(R){for(let U of[R.a,R.b])A(U),U.meeting=null,U.cooldown=f+12+(U.index||0),U.pause?.(!1),U.state=U.pause?"patrol":"idle",U.until=f+1}function P(){h.forEach(b),h.length=0,d.clear(),g=null,i?.(null);for(let R of a)R.guide=null,R.pause?.(!1,R.body),R.trafficYield=!1,R.yieldTram=null,R.pause||(A(R),R.state="idle",R.until=f+1)}function W(R,U){if(p)return;U&&(f+=R);for(let S of a){let X=S.pause||S.index<x();if(X&&!S.body.enabled){if(!e.relocate(S.body,S.body)){S.node.visible=!1;continue}S.body.enabled=!0,S.node.position.set(S.body.x,S.body.y,S.body.z)}if(!X&&S.body.enabled&&(A(S),S.body.enabled=!1,S.meeting&&b(S.meeting)),S.node.visible=!!X,!X||!U)continue;let Z=e.neighbors(S.body,22).sort((V,ie)=>Math.hypot(V.x-S.body.x,V.z-S.body.z)-Math.hypot(ie.x-S.body.x,ie.z-S.body.z)).find(V=>{if(!V.id.startsWith("tram-"))return!1;let ie=S.body.x-V.x,fe=S.body.z-V.z,Ce=Math.cos(V.heading),ze=Math.sin(V.heading);return Math.abs(ie*Ce-fe*ze)<S.body.reach+(S.trafficYield?4:2.3)&&ie*ze+fe*Ce>-8-S.body.reach&&ie*ze+fe*Ce<14});if(!Z&&S.yieldTram){let V=e.body(S.yieldTram),ie=V?S.body.x-V.x:0,fe=V?S.body.z-V.z:0;if(V?.enabled&&Math.hypot(ie,fe)<26&&ie*Math.sin(V.heading)+fe*Math.cos(V.heading)>-V.reach-S.body.reach-1){S.pause?.(!0),S.play?.("idle");continue}S.yieldTram=null}if(Z){if(S.trafficYield=!0,S.yieldTram=Z.id,S.meeting){let me=S.meeting;b(me),h.splice(h.indexOf(me),1)}S.pause?.(!0);let V=Math.cos(Z.heading),ie=Math.sin(Z.heading),fe=Math.sign((S.body.x-Z.x)*V-(S.body.z-Z.z)*ie)||1,Ce=(S.body.x-Z.x)*V-(S.body.z-Z.z)*ie,ze=Math.sign((S.body.x-Z.x)*ie+(S.body.z-Z.z)*V)||1,He=[[V*fe,-ie*fe,S.body.reach+4.5-Math.abs(Ce)],[-V*fe,ie*fe,S.body.reach+4.5+Math.abs(Ce)],[ie*ze,V*ze,1.2]],ce=Array.from({length:16},(me,Oe)=>[Math.cos(Oe*Math.PI/8),Math.sin(Oe*Math.PI/8),2]),Me=([me,Oe])=>me*(S.body.x-Z.x)+Oe*(S.body.z-Z.z);He.push(...ce.filter(me=>Me(me)>.2).sort((me,Oe)=>Me(Oe)-Me(me)));for(let[me,Oe,ue]of He){let xe={x:S.body.x+me*ue,y:S.body.y,z:S.body.z+Oe*ue,heading:S.body.heading};if(!e.clear(S.body,S.body,xe))continue;let H={...xe,x:S.body.x+me*R*1.8,z:S.body.z+Oe*R*1.8};H.y=S.pause?S.body.y:n(H.x,H.z),e.propose(S.body,H,(K,Q)=>{S.node.position.set(Q.x,Q.y,Q.z),S.play?.("walk")}),S.state="yield",S.cooldown=f+5;break}continue}if(S.trafficYield&&(S.trafficYield=!1,S.pause?.(!1,S.body),S.state=S.pause?"patrol":S.path.length?"walk":"idle"),!S.meeting){if(S.pause){S.state==="greet"&&f>S.until&&(S.pause(!1),S.state="patrol");continue}if(S.guide&&Math.hypot(t.position.x-S.body.x,t.position.z-S.body.z)>9){S.play("idle");continue}if(S.path.length){let V=S.path[0],ie=V.x-S.body.x,fe=V.z-S.body.z,Ce=Math.hypot(ie,fe),ze=S.guide?1.9:1.65;if(Ce<.18){S.path.shift();continue}let He=Math.min(Ce,ze*R),ce={x:ie/Ce,z:fe/Ce},Me=S.body.x+ce.x*He,me=S.body.z+ce.z*He,Oe=Uu(S.body.heading,Math.atan2(ie,fe),R),ue={x:S.body.x+ce.x*1.8,y:n(Me,me),z:S.body.z+ce.z*1.8,heading:Oe};if(!e.clear(S.body,S.body,ue)){let H=!1;for(let[K,Q]of[[ce.z,-ce.x],[-ce.z,ce.x],[-ce.x,-ce.z]]){let ee={x:S.body.x+K*1.3,y:S.body.y,z:S.body.z+Q*1.3,heading:S.body.heading};if(e.clear(S.body,S.body,ee)){Me=S.body.x+K*He,me=S.body.z+Q*He,Oe=Uu(S.body.heading,Math.atan2(K,Q),R),H=!0;break}}H||(Me=S.body.x,me=S.body.z)}e.clear(S.body,S.body,{x:Me,y:n(Me,me),z:me,heading:Oe},!1)||(Oe=S.body.heading);let xe=Math.hypot(Me-S.body.x,me-S.body.z)>.001;e.propose(S.body,{x:Me,y:n(Me,me),z:me,heading:Oe},(H,K)=>{S.node.position.set(K.x,K.y,K.z),S.node.rotation.y=K.heading,!H||!xe?(S.wait=(S.wait||0)+R,S.play("idle"),S.state="yield"):(S.wait=0,S.state=S.guide?"guide":S.carrying?"carry":"walk",S.play(S.carrying?"carry":"walk")),S.wait>2.5&&(S.wait=0,A(S),S.state=S.guide?"guide":"idle",S.until=f+.5,S.guide&&(S.path=u.route(S.body,S.guide)))})}else if(S.state==="walk"||S.state==="carry"||S.state==="guide"||S.state==="yield"){if(S.guide&&Math.hypot(S.body.x-S.guide.x,S.body.z-S.guide.z)>.5){S.play("idle"),f>S.until&&(S.path=u.route(S.body,S.guide),S.until=f+2.5);continue}S.guide=null,S.state=S.slot?.place==="meeting-charge"?"charge":S.slot?"work":"idle",S.until=f+5+S.index%4,S.play(S.state==="work"?"work":"idle"),S.slot&&r?.(S.slot.place,5),y++}else f>S.until&&(S.role==="courier"&&S.slot&&(S.state==="work"||S.state==="charge")&&(S.carrying=S.slot.place==="parcel-sorter",S.parcels.forEach(V=>{V.visible=S.carrying})),E(S))}}if(!U){d.update(0,!1);return}for(let S of a){if(!S.body.enabled||S.trafficYield||S.meeting||S.guide||S.state==="greet"||f<S.cooldown)continue;let X=a.find(V=>V!==S&&V.body.enabled&&!V.trafficYield&&V.state!=="greet"&&!V.meeting&&!V.guide&&f>=V.cooldown&&Math.abs(S.body.y-V.body.y)<2&&Math.hypot(S.body.x-V.body.x,S.body.z-V.body.z)>2.8&&Math.hypot(S.body.x-V.body.x,S.body.z-V.body.z)<6);if(!X)continue;let Z={a:S,b:X,at:f,sent:!1,replied:!1};h.push(Z);for(let V of[S,X])V.meeting=Z,V.state="exchange",V.pause?.(!0),V.play?.("greet")}for(let S=h.length-1;S>=0;S--){let X=h[S],Z=f-X.at;if(Z>4.8||!X.a.body.enabled||!X.b.body.enabled){b(X),h.splice(S,1);continue}for(let[V,ie]of[[X.a,X.b],[X.b,X.a]])e.propose(V.body,{...V.body,heading:Uu(V.body.heading,Math.atan2(ie.body.x-V.body.x,ie.body.z-V.body.z),R)},(fe,Ce)=>{V.node.rotation.y=Ce.heading});Z>.6&&!X.sent&&(d.send(X.a.body,X.b.body),X.sent=!0),Z>2.4&&!X.replied&&(d.send(X.b.body,X.a.body),X.replied=!0)}d.update(R,!0);let F=a.find(S=>S.id===g);F&&F.state!==w&&(w=F.state,i?.({...T(F),refresh:!0}))}return{add:_,patrol:D,update:W,suspend:P,addPlace(R,U){if(!l.has(R.id)){l.set(R.id,R);for(let F of U.navigation.workpoints||[])c.push({id:R.id+":"+F.id,place:R.id,role:R.role,x:R.x+F.position[0],y:n(R.x,R.z),z:R.z+F.position[2],owner:null})}},nearby(){let R=[];for(let U of a)if(U.body.enabled){let F=Math.hypot(t.position.x-U.body.x,t.position.z-U.body.z);F<5&&Math.abs(t.position.y-U.body.y)<8&&R.push({kind:"resident",id:U.id,distance:F,x:U.body.x,z:U.body.z})}for(let U of l.values()){let F=Math.hypot(t.position.x-U.x,t.position.z-U.z);F<7&&R.push({kind:"demonstrate",id:U.id,distance:F,x:U.x,z:U.z})}return R},inspect(R){let U=a.find(F=>F.id===R);return I(U),T(U)},action(R,U){let F=a.find(S=>S.id===g);if(!F)return null;if(F.meeting){let S=F.meeting,X=h.indexOf(S);b(S),X>=0&&h.splice(X,1)}if(R==="greet"&&(A(F),F.guide=null,F.pause?.(!0),F.state="greet",F.until=f+3,F.cooldown=f+5,F.play?.("greet")),R==="guide"&&!F.pause&&U){A(F);for(let S of[0,1.5,3]){for(let X=0;X<(S?8:1)&&!F.path.length;X++){let Z=U.x+Math.cos(X*Math.PI/4)*S,V=U.z+Math.sin(X*Math.PI/4)*S;F.path=u.route(F.body,{x:Z,z:V,y:n(Z,V)})}if(F.path.length)break}F.guide=F.path.length?F.path[F.path.length-1]:null,F.state=F.guide?"guide":"idle"}return R==="cancel"&&(A(F),F.guide=null,F.pause?.(!1),F.state=F.pause?"patrol":"idle",F.until=f+1),I(F),T(F)},demonstrate(R,U=!1){l.has(R)&&r?.(R,U?0:6)},setTier(R){m=R},setReplay(R){v!==R&&(v=R,P())},event(R){if(v||!o()||!["started","succeeded","failed","sanitized","progress"].includes(R.state)||!Number.isFinite(R.at)||Date.now()-R.at>2500||R.at>Date.now())return;let U=ls.find(F=>F.district===(R.to==="agent"?R.from:R.to));U&&l.has(U.id)&&r?.(U.id,2,R.state)},stats:()=>({completed:y,meetings:h.length,reservations:c.filter(R=>R.owner).length,signals:d.stats(),residents:a.filter(R=>R.body.enabled).map(R=>({...T(R),x:R.body.x,z:R.body.z})),places:[...l.keys()]}),dispose(){p=!0,P(),a.forEach(R=>e.remove(R.id)),a.length=0,l.clear(),c.length=0,d.dispose()}}}function Lp(s){let e=new dt;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,o="high",a=w=>(t.add(w),w),l=w=>(n.add(w),w),c=a(new Rn(1,.025,6,64)),h=a(new _r(1,48)),u=a(new ht),d=new Float32Array(288),f=new Float32Array(96);for(let w=0;w<96;w++)d[w*3]=Math.cos(w*2.399)*Math.sqrt(w/96),d[w*3+2]=Math.sin(w*2.399)*Math.sqrt(w/96),f[w]=w/96;u.setAttribute("position",new gt(d,3)),u.setAttribute("phase",new gt(f,1));for(let w of ls){let x=new dt;x.name=w.id,x.position.set(w.x,.12,w.z),x.visible=!1,e.add(x);let A=l(new Mt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:kt,toneMapped:!1})),T=[];for(let D=0;D<3;D++){let E=new Ke(c,l(A.clone()));E.rotation.x=-Math.PI/2,x.add(E),T.push(E)}let I=new Ke(h,l(A.clone()));I.rotation.x=-Math.PI/2,I.scale.setScalar(2.5),I.position.y=.2,x.add(I);let _=new ln(u,l(new pt({transparent:!0,depthWrite:!1,blending:kt,uniforms:{time:{value:0},strength:{value:0},color:{value:new be(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));x.add(_),i.set(w.id,{group:x,waves:T,base:I,motes:_,district:w.district,started:0,until:0,source:"ambient",model:null})}let g=new Ke(a(new nn(3.9,2.3)),l(new pt({transparent:!0,depthWrite:!1,side:At,blending:kt,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));g.position.set(0,1.4,.8),i.get("repair-bay").group.add(g);let v=i.get("kinetic-fountain"),m=l(new pt({transparent:!0,depthWrite:!1,side:At,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let w=0;w<8;w++){let x=w*Math.PI/4,A=new z(Math.cos(x)*2.3,.5,Math.sin(x)*2.3),T=new z(Math.cos(x+.5)*.7,.55,Math.sin(x+.5)*.7),I=new Zn(A,new z(Math.cos(x)*1.5,3.7,Math.sin(x)*1.5),T);v.group.add(new Ke(a(new go(I,24,.045,5,!1)),m))}let p=l(new pt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),y=new Ke(a(new _r(2.58,64)),p);return y.rotation.x=-Math.PI/2,y.position.y=.3,v.group.add(y),{attach(w,x){let A=i.get(w);A&&(A.model=x,A.group.visible=!0)},demonstrate(w,x=6,A=null){let T=i.get(w);T&&(T.until<=r&&(T.started=r),T.until=r+x,T.source=A?"live":"ambient",T.status=A,T.model?.play("operate"))},clear(){i.forEach(w=>{w.until=0,w.source="ambient",w.status=null})},setTier(w){o=w},update(w,x){x&&(r+=w),m.uniforms.time.value=p.uniforms.time.value=g.material.uniforms.time.value=r;for(let[A,T]of i){let I=T.until>r,_=A==="kinetic-fountain",D=I?_t.smoothstep(r-T.started,0,.35)*_t.smoothstep(T.until-r,0,.65):0,E=_?Math.max(.25,D):D;I||(T.source="ambient",T.status=null);let b=T.status==="failed"?16737881:T.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[T.district]:7529695;if(T.waves.forEach((P,W)=>{P.visible=I;let R=(r*.6+W/3)%1;P.position.y=.4+R*2,P.scale.setScalar(.4+R*2.6),P.material.opacity=(1-R)*.22*D,P.material.color.setHex(b)}),A==="repair-bay"&&(g.visible=I,g.material.uniforms.strength.value=D),A==="relay-mast"&&I){let P=T.model?.node.getObjectByName("antenna");P&&(P.rotation.y=_t.damp(P.rotation.y,Math.atan2(-T.group.position.x,-12-T.group.position.z),3,x?w:0))}T.base.material.opacity=.08*D,T.motes.visible=o!=="low"&&E>0,T.motes.material.uniforms.time.value=r,T.motes.material.uniforms.strength.value=E,T.model?.mixer&&(T.model.mixer.timeScale=x&&(I||_)?.65:0)}},stats:()=>({places:[...i].filter(([,w])=>w.model).map(([w,x])=>({id:w,operating:x.until>r,source:x.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(w=>w.dispose()),n.forEach(w=>w.dispose()),i.clear()}}}function Dp(s,e){let t=new dt;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,o=[],a=[],l=[],c=new Map,h=new Map,u=new Map,d=new Map,f=new Set,g=[],v=new Set,m=[],p=new Map,y=new AbortController,w=[],x=0,A=!1,T=!1,I=e.tier||"high",_=null,D=0,E=0,b=null,P=null,W=0,R=new Set,U=null,F=0,S=new z,X=new z,Z=new z,V=e.camera,ie={ready:!1,lift:null,cab:null,value:0,motion:null,idle:0},fe=[],Ce=new mn(0,0,0,"YXZ"),ze=O=>Math.atan2(Math.sin(O),Math.cos(O)),He=O=>O<=0?0:O>=1?1:O*O*(3-2*O),ce=e.traffic||Sc(),Me=Lp(s),me=new Map;function Oe(O,q){let Y=as(O,q);for(let N of ls)for(let L of _?.assets.find(k=>k.id===N.id)?.navigation.surfaces||[]){let[k,G,le,se]=L.rect;O>=N.x+k&&O<=N.x+le&&q>=N.z+G&&q<=N.z+se&&(Y=Math.max(Y,as(N.x,N.z)+L.height))}return Y}let ue=Ip(s,{traffic:ce,camera:V,floor:Oe,onSelect:e.onSociety,onDemonstrate:Me.demonstrate,active:e.active}),xe=0,H=0;try{R=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(O=>Hn.some(q=>q.id===O)))}catch{}let K=Tu(Iu),Q=K.getLength(),ee=Hn.map(O=>{let q=1/0,Y=0;for(let N=0;N<3e3;N++){K.getPointAt(N/3e3,S);let L=Math.hypot(S.x-O.x,S.z-O.z);L<q&&(q=L,Y=N/3e3)}return{...O,u:Y}}),ae=(O,q)=>e.onSound?.(O,q?.x||0,q?.y||0,q?.z||0,!!U),ve=new xn({color:11834208,metalness:.8,roughness:.35});r.add(ve);for(let O of[-1,1]){let q=[],Y=[];for(let k=0;k<=512;k++){K.getPointAt(k/512,S),K.getTangentAt(k/512,X);for(let G of[-.045,.045]){let le=O*.85+G;q.push(S.x-X.z*le,.21,S.z+X.x*le)}if(k<512){let G=k*2;Y.push(G,G+1,G+2,G+1,G+3,G+2)}}let L=new ht;L.setAttribute("position",new et(q,3)),L.setIndex(Y),L.computeVertexNormals(),i.add(L),t.add(new Ke(L,ve))}async function Ue(O,q=I==="low"?2:I==="medium"?1:0){let Y=O+":"+q;return n.has(Y)||n.set(Y,(async()=>{let N=_.assets.find(ge=>ge.id===O),L=N?.lods.find(ge=>ge.level===q);if(!L||!/^[\w-]+\.lod[0-2]\.glb$/.test(L.file))throw Error("Invalid world asset");let k=await fetch(e.assetURL(L.file),{signal:y.signal});if(!k.ok)throw Error("World asset unavailable");let G=await k.arrayBuffer();if(T)throw Error("Disposed");D+=G.byteLength;let le=await new os().parseAsync(G,""),se=le.animations.length>0||/^(tram|service-cart|robot-|door|lift|sky-lift|telescope)/.test(O);if(le.scene.traverse(ge=>{if(ge.isMesh){i.add(ge.geometry);for(let Ae of Array.isArray(ge.material)?ge.material:[ge.material])r.add(Ae);ge.castShadow=!0,ge.receiveShadow=!0,e.surfaces?.prepare(ge,se)}}),T)throw i.forEach(ge=>ge.dispose()),r.forEach(ge=>ge.dispose()),Error("Disposed");return f.add(O),le})()),n.get(Y)}async function Se(O,q,Y,N=0,L=0,k=t,G=[1,1,1],le){let se=await Ue(O,le);if(T)return null;let ge=se.scene.clone(!0);ge.position.set(q,N,Y),ge.rotation.y=L,ge.scale.set(...G),k.add(ge),ge.traverse(rt=>{rt.isMesh&&(rt.userData.worldAsset=O,rt.userData.worldPart=rt.name,g.push(rt))});let Ae=_.assets.find(rt=>rt.id===O),at="furnishing:"+xe++;for(let[rt,Nt]of(Ae.navigation.colliders||[]).entries())ce.solid(at+":"+rt,{owner:at,x:q,z:Y,heading:L,min:[Nt[0]*G[0],N+Nt[1]*G[1],Nt[2]*G[2]],max:[Nt[3]*G[0],N+Nt[4]*G[1],Nt[5]*G[2]]});let Qe=se.animations.length?new Ro(ge):null;Qe&&o.push(Qe);let Lt=null;return{node:ge,mixer:Qe,owner:at,bounds:Ae.motion_bounds||Ae.lods[0].bounds,clips:se.animations,play(rt,Nt=!1){if(!Qe)return;let Ot=se.animations.find(vi=>vi.name===rt);if(!Ot)return;let Dt=Qe.clipAction(Ot);return Lt===Dt||(Dt.reset(),Nt&&(Dt.setLoop(nc,1),Dt.clampWhenFinished=!0),Dt.play(),Lt&&Dt.crossFadeFrom(Lt,.25,!1),Lt=Dt),Dt}}}function je(O){O.updateMatrixWorld(!0);let q=new Map;O.traverse(N=>{if(!N.isMesh)return;let L=N.geometry.uuid+":"+(Array.isArray(N.material)?N.material.map(k=>k.uuid).join(","):N.material.uuid);q.has(L)||q.set(L,{n:N,matrices:[]}),q.get(L).matrices.push(N.matrixWorld.clone())});let Y=new dt;t.add(Y);for(let{n:N,matrices:L}of q.values()){let k=new gn(N.geometry,N.material,L.length);L.forEach((G,le)=>k.setMatrixAt(le,G)),k.castShadow=!0,k.receiveShadow=!0,k.userData={...N.userData},g.push(k),Y.add(k)}return O.traverse(N=>{let L=g.indexOf(N);L>=0&&g.splice(L,1)}),O.removeFromParent(),u.set(Y.uuid,Y),Y}function Ge(O){if(d.has(O.id))return d.get(O.id);let q=(async()=>{let Y=new dt;t.add(Y);let N=[];for(let L of Tp())N.push(Se("floor",O.x+L.x,O.z+L.z,ut.room,0,Y,[L.sx,1,L.sz]));for(let L=-4;L<=4;L+=4)for(let k=-4;k<=4;k+=4)N.push(Se("ceiling",O.x+L,O.z+k,8.15+ut.room,0,Y));for(let L=-4;L<=4;L+=4)N.push(Se("wall",O.x+L,O.z+6,ut.room,0,Y));for(let L=-4;L<=4;L+=4)for(let k of[-1,1])for(let G of[0,4])N.push(Se("window",O.x+k*6,O.z+L,ut.room+G,Math.PI/2,Y));for(let L=-4;L<=4;L+=4)for(let k of[-1,1])N.push(Se("window",O.x+L,O.z+k*6,ut.gallery,0,Y));for(let L of[-4,4])N.push(Se("wall",O.x+L,O.doorZ,ut.room,0,Y));for(let[L,k]of[[-4,1],[.2,1.1]])N.push(Se("floor",O.x+L,O.z+3,ut.gallery,0,Y,[k,1,1.5])),N.push(Se("railing",O.x+L,O.z,ut.gallery,0,Y,[k,1,1]));for(let L of[.7,5.3])N.push(Se("railing",O.x+2.4,O.z+L,ut.gallery,Math.PI/2,Y,[.35,1,1]));for(let L of[-5.6,1.9])N.push(Se("wall",O.x+L,O.z+.15,ut.room,0,Y,[.05,1,.6]));if(await Promise.all(N),!T)return je(Y)})();return d.set(O.id,q),q}async function st(O){if(!(c.has(O.id)||T)){c.set(O.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),F++;try{await Ge(O);let q=new dt;t.add(q);let Y=[];for(let L of[-3,0])Y.push(Se(O.id==="missions"?"cargo":O.id==="memory"?"archive-shelf":"console",O.x+L,O.z+3,ut.room,Math.PI,q));if(Y.push(Se("bench",O.x-4,O.z-1,ut.room,Math.PI/2,q)),Y.push(Se("bench",O.x-4,O.z+4.8,ut.gallery,0,q)),Y.push(Se("console",O.x,O.z+4.8,ut.gallery,Math.PI,q)),await Promise.all(Y),T)return;je(q);let N=c.get(O.id);N.hologram=await Se("hologram",O.x-1,O.z,ut.room),N.hologram?.play("operate"),O.id==="memory"&&Je(),N.lift=await Se("lift",O.liftX,O.liftZ,ut.room),N.liftAction=N.lift?.play("operate",!0),N.liftAction&&(N.liftAction.paused=!0),await B(O.id,O.x-2.2,O.z+.9,ut.gallery,Math.PI,{skip:null,limit:.9,pitch:[-.14,.18],eye:0}),N.ready=!0}catch(q){T||e.onError?.(q)}finally{F--}}}async function j(){try{let O=await fetch(e.assetURL("manifest.json"),{signal:y.signal});if(!O.ok)throw Error("World manifest unavailable");_=await O.json();let q=new dt;t.add(q);let Y=[];for(let L of Hn)Y.push(Se("station",L.platformX,L.platformZ,ut.pavement,L.angle,q));for(let L of hn){Y.push(Ge(L));let k=await Se("door",L.x,L.doorZ,ut.room,Math.PI);if(T)return;let G=k.play("open",!0);G.paused=!0,h.set(L.id,{...k,action:G,value:0,open:!1})}for(let L=-76;L<=76;L+=8)Y.push(Se("quay",L,77,ut.quay,0,q));for(let[L,k]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])Y.push(Se("garden",L,k,0,0,q)),Y.push(Se("bench",L+3,k,0,Math.PI/2,q));for(let[L,k]of[[-30,23],[30,23],[-28,72]])Y.push(Se("arcade",L,k,0,0,q));Y.push(Se("bridge",-78.5,40,2,0,q,[1.75,1,1]),Se("ramp",-80,49,0,Math.PI,q),Se("stairs",-76.8,49,0,Math.PI,q),Se("ramp",-78.5,31,0,0,q)),Y.push(Se("pad",_i.x,_i.z,0,0,q)),Y.push(Se("sky-deck",St.x,St.z,St.floor,0,q));for(let L of St.benches)Y.push(Se("bench",L.x,L.z,St.floor,L.angle,q));for(let[L,k]of[[-60,-56],[-60,-46],[51,49]])Y.push(Se("charger",L,k,0,0,q));if(await Promise.all(Y),T||(je(q),ie.lift=await Se("sky-lift",St.lift.x,St.lift.z,St.lift.base),T))return;ie.cab=ie.lift?.node.getObjectByName("cab")||null,ie.ready=!!ie.cab;for(let L of St.telescopes)await B(L.id,L.x,L.z,St.floor,L.angle,{skip:"agent",limit:1.45,pitch:[-1.1,.3],eye:1.25});for(let[L,k]of[[-57,-61],[-57,-52]])(await Se("cooler",L,k))?.play("operate");for(let L of ls){let k=await Se(L.id,L.x,L.z,as(L.x,L.z),0,t,[1,1,1],2);if(T)return;k&&(k.level=2,me.set(L.id,k),Me.attach(L.id,k),ue.addPlace(L,_.assets.find(G=>G.id===L.id)),k.play("operate"))}let N=await Se("service-cart",55,49,ut.ground,Math.PI/2);N&&N.play("open",!0);for(let L=0;L<19;L++){let k=["courier","technician","archivist"][L%3],G=await Se("robot-"+k,0,0);if(!G)return;G.node.scale.setScalar(1.15),a.push(G),ue.add(G,L,G.bounds),G.play("idle")}for(let L=0;L<2;L++){let k=await Se("tram",0,0);if(!k)return;let G=k.play("open",!0);G.paused=!0;let le=ee[L?3:0].u;K.getPointAt(le,S),K.getTangentAt(le,X);let se=ce.register("tram-"+L,Ui(k.bounds),{x:S.x,y:ut.road+.03,z:S.z,heading:Math.atan2(X.x,X.z)},3);l.push({...k,body:se,u:le,speed:0,dwell:5,stop:L?3:0,door:G})}for(let L=0;L<3;L++){let k=await Se("cargo",43,70);k&&(ce.removeOwner(k.owner),k.node.visible=!1,m.push({...k,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(O){T||e.onError?.(O)}}j();async function tt(O,q,Y){if(!(q.level===Y||q.requested===Y)){q.requested=Y;try{let N=await Ue(O,Y);if(T||q.requested!==Y)return;let L=new Map;N.scene.traverse(k=>{k.isMesh&&L.set(k.name,k)}),q.node.traverse(k=>{let G=L.get(k.name);k.isMesh&&G&&(k.geometry=G.geometry,k.material=G.material)}),q.level=Y,q.requested=null,e.onReady?.()}catch(N){q.requested=null,T||e.onError?.(N)}}}function Je(){let O=c.get("memory");if(!O?.hologram||!O.text&&!w.length)return;if(!O.text){let k=document.createElement("canvas");k.width=1024,k.height=512;let G=new As(k);G.colorSpace=Ut,v.add(G);let le=new nn(4.8,2.4),se=new Mt({map:G,transparent:!0,depthWrite:!1,side:At,toneMapped:!1});i.add(le),r.add(se);let ge=new Ke(le,se),Ae=hn.find(at=>at.id==="memory");ge.position.set(Ae.x-1,3.6+ut.room,Ae.z+.3),ge.rotation.y=Math.PI,t.add(ge),O.text={canvas:k,texture:G,mesh:ge}}let{canvas:q,texture:Y,mesh:N}=O.text,L=q.getContext("2d");L.clearRect(0,0,1024,512),N.visible=w.length>0,L.fillStyle="rgba(5,24,32,.87)",L.fillRect(0,0,1024,512),L.fillStyle="#b9f4ef",L.font="28px sans-serif",w.slice(0,4).forEach((k,G)=>{let le=Array.from(k).slice(0,96);L.fillText(le.slice(0,48).join(""),30,55+G*118),L.fillText(le.slice(48).join(""),30,94+G*118)}),Y.needsUpdate=!0}async function B(O,q,Y,N,L,k){let G=await Se("telescope",q,Y,N,L);!G||T||fe.push({id:O,model:G,tube:G.node.getObjectByName("tube"),x:q,z:Y,floor:N,angle:L,...k,ex:q+Math.sin(L)*k.eye,ez:Y+Math.cos(L)*k.eye})}function M(O){let q=fe.find(le=>le.id===O);if(!q)return;let Y=ze(q.angle+Math.PI),N=q.floor+2.05,L=Y,k=-.08,G=1/0;for(let le of e.districts||[]){if(le.id===q.skip)continue;let se=le.x-q.ex,ge=le.z-q.ez,Ae=Math.hypot(se,ge),at=Math.atan2(-se,-ge),Qe=Math.abs(ze(at-Y));Ae>12&&Qe<q.limit&&Qe<G&&(G=Qe,L=at,k=_t.clamp(Math.atan2(le.height*.45-N,Ae),...q.pitch))}b={kind:"scope",id:O,scope:q,yaw0:Y,fov:14,sent:"",target:null,back:{position:V.position.clone(),quaternion:V.quaternion.clone(),fov:V.fov}},q.model.node.visible=!1,V.position.set(q.ex,N,q.ez),V.quaternion.setFromEuler(Ce.set(k,L,0,"YXZ")),e.reduced()&&(V.fov=b.fov,V.updateProjectionMatrix()),e.onRide?.("scope"),ae("door",V.position)}function te(O,q,Y){let N=Y.radius*.6,L=O.x-Y.x,k=O.z-Y.z,G=q.x*q.x+q.z*q.z;if(G<1e-9)return 1/0;let le=2*(L*q.x+k*q.z),se=le*le-4*G*(L*L+k*k-N*N);if(se<0)return 1/0;let ge=(-le-Math.sqrt(se))/(2*G);if(ge<0&&(ge=(-le+Math.sqrt(se))/(2*G)),ge<0)return 1/0;let Ae=O.y+q.y*ge;return Ae>=0&&Ae<=Y.height?ge:1/0}function he(O){let q=b.scope;Ce.setFromQuaternion(V.quaternion,"YXZ"),Ce.y=b.yaw0+_t.clamp(ze(Ce.y-b.yaw0),-q.limit,q.limit),Ce.x=_t.clamp(Ce.x,...q.pitch),Ce.z=0,V.quaternion.setFromEuler(Ce),V.position.set(q.ex,q.floor+2.05,q.ez);let Y=e.reduced()?b.fov:_t.damp(V.fov,b.fov,9,Math.min(O,.1));Math.abs(Y-b.fov)<.02&&(Y=b.fov),Y!==V.fov&&(V.fov=Y,V.updateProjectionMatrix()),V.getWorldDirection(S);let N=null,L=1/0;for(let se of e.districts||[]){if(se.id===q.skip)continue;let ge=te(V.position,S,se);ge>12&&ge<L&&(L=ge,N=se.id)}b.target=N;let k=b.back.fov/V.fov,G=(Math.atan2(S.x,-S.z)*180/Math.PI+360)%360,le=[N,N?Math.round(L):0,k.toFixed(1),Math.round(G)].join();le!==b.sent&&(b.sent=le,e.onScope?.({id:b.id,target:N,distance:N?Math.round(L):null,zoom:+k.toFixed(1),bearing:Math.round(G)%360}))}function de(O,q,Y){ie.motion={from:ie.value,to:O,t:0,duration:e.reduced()?0:q},ie.idle=0,Y&&ae("lift",ie.lift.node.position)}function Te(){let O=St.lift;V.position.set(O.x,O.base+O.cab+ie.value+2.4,O.z)}function Le(O){de(O,15,!0),b={kind:"sky",to:O},Te(),V.quaternion.setFromEuler(Ce.set(-.42,Math.PI,0,"YXZ")),e.onRide?.("sky")}function pe(O,q){let Y=St.lift;if(!ie.motion&&b?.kind!=="sky"&&q>0&&Math.hypot(V.position.x-Y.x,V.position.z-Y.z)>30&&(ie.idle+=q,ie.idle>28&&de(ie.value>1?0:Y.travel,15,!1)),ie.motion){let N=ie.motion;N.t+=Math.min(O,.1);let L=N.duration?N.t/N.duration:1;ie.value=N.from+(N.to-N.from)*He(L),L>=1&&(ie.value=N.to,ie.motion=null)}ie.cab.position.y=ie.value,b?.kind==="sky"&&(Te(),ie.motion||(b=null,e.onRide?.(null),ae("lift",V.position)))}function _e(){if(b){if(b.kind==="tram"&&l[b.index].dwell<=1){b.exitRequested=!0;return}De();return}if(P){if(P.kind==="telescope"){M(P.id);return}if(P.kind==="skycall"){de(V.position.y>40?St.lift.travel:0,7,!0);return}if(P.kind==="skyup"||P.kind==="skydown"){Le(P.kind==="skyup"?St.lift.travel:0);return}if(P.kind==="resident"){ue.inspect(P.id);return}if(P.kind==="demonstrate"){ue.demonstrate(P.id,e.reduced()),e.onSociety?.({id:P.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(P.kind==="door"){let O=h.get(P.id);if(O){O.open=!O.open,ae("door",O.node.position);let q=hn.find(Y=>Y.id===P.id);st(q)}}if(P.kind==="discover"){R.add(P.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...R]))}catch{}e.onDiscover?.(P.id),ae("discover",V.position)}if(P.kind==="tram"&&!e.reduced()&&(b={kind:"waiting",station:P.id},e.onRide?.("waiting")),P.kind==="terminal"&&e.onTerminal?.(P.id),P.kind==="drone"&&!e.reduced()&&(b={kind:"drone",elapsed:0,origin:V.position.clone()},e.onRide?.("drone"),ae("tram",V.position)),P.kind==="lift"){let O=c.get(P.id);if(O){let q=V.position.y>5?4:0;if(Math.abs(O.liftValue-q)>.02){O.liftTarget=q;return}O.liftTarget=q?0:4,b={kind:"lift",id:P.id},e.onRide?.("lift"),ae("lift",V.position)}}}}function De(){if(b){if(b.kind==="lift"){let O=c.get(b.id),q=hn.find(Y=>Y.id===b.id);O.liftTarget=O.liftValue<2?0:4,V.position.set(q.x+1.5,2.4+ut.room+O.liftTarget,q.liftZ)}if(b.kind==="tram"){let O=Hn.find(q=>q.id===b.station)||Hn[0];V.position.set(O.platformX,2.4+ut.pavement+.3,O.platformZ)}if(b.kind==="drone"&&V.position.copy(b.origin),b.kind==="sky"&&(ie.motion=null,ie.value=b.to,ie.cab.position.y=ie.value,Te()),b.kind==="scope"){let O=b.scope;Ce.setFromQuaternion(V.quaternion,"YXZ"),O.tube?.rotation.set(-Ce.x,ze(Ce.y+Math.PI-O.angle),0,"YXZ");let q=Ce.y;O.model.node.visible=!0,V.position.copy(b.back.position),Ce.setFromQuaternion(b.back.quaternion,"YXZ"),Ce.y=q,V.quaternion.setFromEuler(Ce),V.fov=b.back.fov,V.updateProjectionMatrix(),e.onScope?.(null)}b=null,e.onRide?.(null)}}function We(){if(b)return[{kind:b.kind==="scope"?"unscope":"exit",id:b.kind,distance:0}];V.getWorldDirection(S);let O=ue.nearby().filter(Y=>(Y.x-V.position.x)*S.x+(Y.z-V.position.z)*S.z>Y.distance*.3),q=(Y,N,L,k,G)=>{let le=Math.hypot(V.position.x-L,V.position.z-k);le<G&&O.push({kind:Y,id:N,distance:le})};for(let Y of hn)V.position.y<5&&q("door",Y.id,Y.x,Y.doorZ,3),c.get(Y.id)?.ready&&(q("lift",Y.id,Y.liftX,Y.liftZ,2.3),V.position.y<5&&q("terminal",Y.id,Y.x-2,Y.z+3,2.5));if(V.position.y<5)for(let Y of Hn)q("tram",Y.id,Y.platformX,Y.platformZ,4),R.has(Y.id)||q("discover",Y.id,Y.platformX+(Y.angle===0?5:0),Y.platformZ+(Y.angle===0?0:5),3);if(q("drone","drone",_i.x,_i.z,5),ie.ready){let Y=St.lift,N=V.position.y>40,L=Math.hypot(V.position.x-Y.x,V.position.z-Y.z),k=N?Y.travel:0;L<(N?3.6:2.4)&&(N||V.position.y<10)&&O.push({kind:ie.motion||Math.abs(ie.value-k)>.05?"skycall":N?"skydown":"skyup",id:"sky",distance:L})}for(let Y of fe)Math.abs(V.position.y-(Y.floor+2.4))<1.5&&q("telescope",Y.id,Y.x,Y.z,1.7);return O.sort((Y,N)=>Y.distance-N.distance)}function Pe(O,q,Y){if(T||!_)return;let N=q?Math.min(O,.05):0;if(E+=N,U=Bs(V.position.x,V.position.z)?.id||null,H+=O,H>.5){H=0;for(let[L,k]of me){let G=V.position.distanceTo(k.node.position),le=I==="low"||G>55?2:I==="medium"||G>32?1:0;tt(L,k,le)}}e.traffic||ce.begin();for(let L of hn)Math.hypot(V.position.x-L.x,V.position.z-L.z)<32&&st(L);o.forEach(L=>{L.getRoot().visible&&L.update(N)});for(let L of h.values()){let k=_t.damp(L.value,L.open?1:0,5,Math.min(O,.1));Math.abs(k-(L.open?1:0))<.001&&(k=L.open?1:0);let G=le=>[-1,1].map(se=>({owner:L.owner,x:L.node.position.x,z:L.node.position.z,min:[se*.8+se*1.6*le-.8,ut.room,-.13],max:[se*.8+se*1.6*le+.8,ut.room+3.6,.13]}));(L.open||G(k).every(le=>ce.solidClear(le)))&&(L.value=k),G(L.value).forEach((le,se)=>ce.solid(L.owner+":panel:"+se,le)),L.action.time=L.value*L.action.getClip().duration,L.mixer.update(0)}ie.ready&&pe(O,N),b?.kind==="scope"&&he(O);for(let[L,k]of c)if(k.liftAction&&(k.liftValue=_t.damp(k.liftValue,k.liftTarget,1.8,Math.min(O,.1)),Math.abs(k.liftValue-k.liftTarget)<.02&&(k.liftValue=k.liftTarget),k.liftAction.time=k.liftValue/4*k.liftAction.getClip().duration,k.lift.mixer.update(0),b?.kind==="lift"&&b.id===L)){let G=hn.find(le=>le.id===L);V.position.set(G.liftX,2.4+ut.room+k.liftValue,G.liftZ),k.liftValue===k.liftTarget&&(b=null,e.onRide?.(null))}if(ue.update(N,q),Me.update(N,q),l.forEach((L,k)=>{if(L.node.visible=!!L.body&&k<(I==="low"?1:2),L.body&&(L.node.visible&&!L.body.enabled&&!ce.clear({...L.body,enabled:!0},L.body,L.body)&&(L.node.visible=!1),L.body.enabled=L.node.visible),!L.node.visible)return;let G={u:L.u,dwell:L.dwell,stop:L.stop},le=!1;if(L.dwell>0)L.dwell-=N,L.speed=0;else{let se=rt=>(rt.u-L.u+1)%1*Q,ge=ee.reduce((rt,Nt)=>se(Nt)>1e-5&&se(Nt)<se(rt)?Nt:rt,ee.find(rt=>se(rt)>1e-5)||ee[0]),Ae=Math.min(11,Math.sqrt(se(ge)*8));for(let rt=1;rt<=3;rt++){let Nt=(L.u+(1.2+L.speed*.7)*rt/3/Q)%1;K.getPointAt(Nt,S),K.getTangentAt(Nt,X);let Ot={x:S.x,y:ut.road+.03,z:S.z,heading:Math.atan2(X.x,X.z)};if(!ce.clear(L.body,Ot,Ot)){Ae=0;break}}L.speed+=_t.clamp(Ae-L.speed,-N*8,N*3.5);let at=N*L.speed/Q,Qe=L.u;L.u=(L.u+at)%1;let Lt=ee.findIndex(rt=>(rt.u-Qe+1)%1>0&&(rt.u-Qe+1)%1<=at+1e-5);Lt>=0&&(L.stop=Lt,L.u=ee[Lt].u,L.dwell=6,L.speed=0,le=!0)}if(K.getPointAt(L.u,L.node.position),L.node.position.y=ut.road+.03,K.getTangentAt(L.u,X),L.node.rotation.y=Math.atan2(X.x,X.z),L.body&&(L.body.enabled=L.node.visible,ce.propose(L.body,{...L.node.position,heading:L.node.rotation.y},(se,ge)=>{se||(Object.assign(L,G),L.speed=0),L.node.position.set(ge.x,ge.y,ge.z),L.node.rotation.y=ge.heading,se&&le&&ae("tram",L.node.position)})),L.door.time=(L.dwell>1?1:0)*L.door.getClip().duration,L.mixer.update(0),b?.kind==="waiting"&&ee[L.stop].id===b.station&&L.dwell>1&&(b={kind:"tram",index:k,station:b.station,offset:0},e.onRide?.("tram")),b?.kind==="tram"&&b.index===k){if(L.dwell>1&&(b.station=ee[L.stop].id,b.exitRequested)){De();return}V.position.copy(L.node.position).addScaledVector(X,b.offset),V.position.y+=.9+1.4,V.lookAt(L.node.position.x+X.x*16,V.position.y,L.node.position.z+X.z*16)}}),b?.kind==="drone"){b.elapsed+=Math.min(O,.1);let L=b.elapsed/40*Math.PI*2;V.position.set(Math.cos(L)*100,45+Math.sin(L*2)*12,Math.sin(L)*95-10),V.lookAt(0,22,-12),b.elapsed>=40&&De()}if(P=Y==="street"&&We()[0]||null,Y==="street"&&!b){let L=V.position.distanceTo(Z);L<3&&(W+=L),W>1.8&&(ae(U?"step_inside":"step",V.position),W=0)}Z.copy(V.position),e.onEnvironment?.(!!U);for(let[L,k]of m.entries()){if(k.elapsed+=N,k.node.visible=q&&k.elapsed<8,!k.node.visible){k.body&&ce.remove(k.body.id),k.body=null;continue}let G=k.direction>0?k.elapsed/8:1-k.elapsed/8,le={x:40+G*3,y:ut.room,z:70,heading:0};if(k.body||(k.body=ce.register("freight-"+L,Ui(k.bounds),le,0)),!k.body){k.elapsed=9,k.node.visible=!1;continue}ce.propose(k.body,le,(se,ge)=>{se||(k.elapsed-=N),k.node.position.set(ge.x,ge.y,ge.z)})}e.traffic||ce.solve(N),e.onInteraction?.(P,R.size,U)}async function Fe(O){if(I=O,!_)return;let q=O==="low"?2:O==="medium"?1:0;ue.setTier(O),Me.setTier(O);try{let Y=new Map(await Promise.all([...f].filter(N=>!me.has(N)).map(async N=>{let L=await Ue(N,q),k=new Map;return L.scene.traverse(G=>{G.isMesh&&k.set(G.name,G)}),[N,k]})));if(T||I!==O)return;for(let N of g){let L=Y.get(N.userData.worldAsset)?.get(N.userData.worldPart);L&&(N.geometry=L.geometry,N.material=L.material)}e.onReady?.()}catch(Y){T||e.onError?.(Y)}}return{update:Pe,interact:_e,endRide:De,setTier:Fe,society:ue,suspend(){ue.suspend(),Me.clear(),A=!1;for(let O of m)O.elapsed=9,O.node.visible=!1,O.body&&ce.remove(O.body.id),O.body=null},syncRide(){if(b?.kind==="tram"){let O=l[b.index];K.getTangentAt(O.u,X),V.position.copy(O.node.position).addScaledVector(X,b.offset),V.position.y+=2.3,V.lookAt(O.node.position.x+X.x*16,V.position.y,O.node.position.z+X.z*16)}},socialAction(O,q){let Y=Hn.find(N=>N.id===q);return ue.action(e.reduced()&&O==="guide"?"cancel":O,Y?{x:Y.platformX,z:Y.platformZ}:null)},setMemory(O){w=(Array.isArray(O)?O:[]).filter(q=>typeof q=="string").slice(0,8),Je()},setWorld(O,q=!1){q?(ue.setReplay(!0),Me.clear()):ue.setReplay(!1);let Y=O?.entities?.filter(L=>L.kind==="mission")||[],N=new Set;for(let L of Y){N.add(L.id);let k=p.get(L.id);if(!q&&e.active()&&A&&k!=null&&k!==L.state&&["running","completed","failed","cancelled"].includes(L.state)&&Date.now()-L.at<3e4){let G=m.find(le=>le.elapsed>=8);G&&(G.elapsed=0,G.direction=L.state==="running"?1:-1,x++)}p.set(L.id,L.state)}for(let L of p.keys())N.has(L)||p.delete(L);if(A=!q,q)for(let L of m)L.elapsed=9,L.node.visible=!1},walkRide(O,q,Y=0){if(!b)return!1;if(b.kind==="tram"&&(b.offset=_t.clamp(b.offset+O*q*3,-2.5,2.5)),b.kind==="scope"){Ce.setFromQuaternion(V.quaternion,"YXZ");let N=q*.9*V.fov/b.back.fov;Ce.x+=O*N,Ce.y-=Y*N,V.quaternion.setFromEuler(Ce)}return!0},zoom(O){return b?.kind!=="scope"?!1:(b.fov=_t.clamp(b.fov*Math.exp(O*.0012),5,26),!0)},scoping:()=>b?.kind==="scope",move(O,q,Y){if(b)return!1;if(Y.y>40)return Sp(O,q,!ie.motion&&ie.value===St.lift.travel);let N=Bs(Y.x,Y.z);return N&&Y.y>5?Math.abs(q-N.z-4.8)<.85&&(Math.abs(O-N.x)<1.1||Math.abs(O-N.x+4)<1.75)?!1:Lu(N,O,q,c.get(N.id)?.liftValue===4):N&&Y.y<5&&([-5.6,1.9].some(L=>Math.abs(O-N.x-L)<.25&&Math.abs(q-N.z-.15)<.25)||Ep(N,O,q)&&!(O<N.liftX+1.25&&Math.abs(q-N.liftZ)<1.25&&c.get(N.id)?.liftValue===0)||Math.hypot(O-(N.x-1),q-N.z)<1.55||[-3,0].some(L=>Math.abs(O-N.x-L)<1.1&&Math.abs(q-N.z-3)<.8))?!1:Rp(O,q,Y,L=>h.get(L)?.value>.85&&!!c.get(L)?.ready,e.districts)},floor(O,q){if(V.position.y>40)return St.floor;let Y=Bs(O,q);return Y&&V.position.y>5&&Lu(Y,O,q,c.get(Y.id)?.liftValue===4)?ut.gallery:Math.max(Oe(O,q),Ap(O,q,e.districts))},visit(O){De(),b=null;let q=ls.find(N=>N.id===O);if(q){let N=_?.assets.find(L=>L.id===O)?.navigation.interaction[0]?.position||[0,1.5,4];V.position.set(q.x,2.4+as(q.x,q.z),q.z+N[2]+1),V.lookAt(q.x,2,q.z);return}if(O==="drone"){V.position.set(_i.x,2.7,_i.z+3),V.lookAt(_i.x,2,_i.z);return}if(O==="skydeck"){let N=57*Math.PI/180,L=St.x+Math.sin(N)*7.2,k=St.z-Math.cos(N)*7.2;V.position.set(L,St.floor+2.4,k),V.lookAt(L,St.floor+.4,k-10);return}let Y=Hn.find(N=>N.id===O);if(Y){let N=R.has(O)?0:5,L=Y.platformX+(Y.angle===0?N:0),k=Y.platformZ+(Y.angle===0?0:N);V.position.set(L,2.4+as(L,k),k),V.lookAt(Y.platformX,2,Y.platformZ===k?Y.platformZ+1:Y.platformZ)}},destination(O){let q=hn.find(Y=>Y.id===O);if(q){De(),b=null;let Y=q.doorZ+q.front*4;V.position.set(q.x,2.4+as(q.x,Y),Y),V.lookAt(q.x,2.4+as(q.x,Y),q.z)}},isRiding:()=>!!b,interaction:()=>P,rideBody:()=>b?.kind==="tram"?"tram-"+b.index:b?.kind==="scope"?b.scope.model.owner:b?.kind==="sky"&&ie.lift?.owner||null,stats:()=>({society:ue.stats(),machinery:Me.stats(),details:[...me].map(([O,q])=>({id:O,level:q.level})),loaded:[...f],bytes:D,freightEvents:x,residents:a.filter(O=>O.node.visible).length,trams:l.filter(O=>O.node.visible).length,rooms:[...c].filter(([,O])=>O.ready).map(([O])=>O),inside:U,ride:b?.kind||null,station:b?.station||null,discovered:[...R],loading:F,interactions:P?{kind:P.kind,id:P.id}:null,sky:{ready:ie.ready,value:+ie.value.toFixed(2),moving:!!ie.motion},telescopes:fe.length,scope:b?.kind==="scope"?{id:b.id,target:b.target,fov:+V.fov.toFixed(1)}:null}),dispose(){T||(T=!0,y.abort(),ue.dispose(),Me.dispose(),e.traffic||ce.dispose(),w=[],p.clear(),t.removeFromParent(),o.forEach(O=>{O.stopAllAction(),O.uncacheRoot(O.getRoot())}),t.traverse(O=>{O.isInstancedMesh&&O.dispose()}),i.forEach(O=>O.dispose()),r.forEach(O=>O.dispose()),v.forEach(O=>O.dispose()),n.clear())}}}var Cy=s=>.5+(s-6)/12*3.62,Np=new z;function Up(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:o}){let a="local",l="clear",c="high",h=!1,u=-100,d=!1,f=n.intensity,g=14,v=-1,m=0,p={value:0},y=new ht,w=[],x=317,A=()=>(x=Math.imul(x,1664525)+1013904223>>>0)/4294967296;for(let R=0;R<1200;R++)w.push((A()-.5)*170,A()*60,(A()-.5)*174-7);y.setAttribute("position",new et(w,3));let T=new pt({transparent:!0,depthWrite:!1,uniforms:{time:p,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),I=new ln(y,T);I.frustumCulled=!1,s.add(I);let _=new be(9417679),D=new be(528926),E=new be(2762296);function b(){let R=Tc(a),U=R.amount,F=Cy(R.hour);e.intensity=.8+U*2.6,e.color.set(R.evening>.1?16757370:13164287),e.position.set(Math.cos(R.hour/24*Math.PI*2)*120,40+U*115,85),Np.set(Math.cos(F)*120,40-R.evening*18+U*115,Math.sin(F)*120),t.intensity=.7+R.evening*1.6,f=n.intensity=.45+U*.45,s.fog.color.copy(D).lerp(_,U).lerp(E,R.evening*(1-U)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-U*.0016,i.setLighting?.(U,R.evening,Np),i.setWeather?.(l),r?.setLighting(U,R.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),I.visible=l==="rain"&&!h,y.setDrawRange(0,c==="low"?200:1200)}function P(R,U){if(l!=="rain"||h||!U){v>=0&&(v=-1,W(0));return}if(g-=R,v<0&&g<=0&&(v=0,m++,g=9+A()*16,o?.(.6+A()*3.2)),v<0)return;v+=R;let F=v<.09?1:v<.16?.25:v<.24?.75:Math.max(0,1-(v-.24)/.35)*.5;W(F),v>.6&&(v=-1,W(0))}function W(R){n.intensity=f+R*2.4,T.uniforms.flash.value=R,i.setFlash?.(R)}return b(),{set(R={}){["local","day","evening","night"].includes(R.time)&&(a=R.time),["clear","rain","fog"].includes(R.weather)&&(l=R.weather),b()},setTier(R){c=R,b()},setIndoor(R){h!==R&&(h=R,b())},update(R,U,F){if(!d)return I.visible=l==="rain"&&!h&&F,F&&(p.value+=Math.min(R,.1)),P(Math.min(R,.1),F),U-u>30?(u=U,b(),!0):!1},mood(){let R=Tc(a);return{day:R.amount,evening:R.evening,weather:l,indoor:h}},stats:()=>({time:a,weather:l,indoor:h,daylight:Tc(a).amount,strikes:m}),dispose(){d=!0,I.removeFromParent(),y.dispose(),T.dispose()}}}function Fp(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(Du[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([a,l,c,h],u)=>s.solid("district:"+i.id+":"+u,{x:i.x,z:i.z,min:[a,0,l],max:[c,8,h]})),Nu[i.id]?Nu[i.id].forEach(([a,l,c],h)=>s.solid("district:"+i.id+":upper:"+h,{x:i.x,z:i.z,min:[-a,l,-a],max:[a,c,a]})):s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let o=e.get(i.asset)?.lods[0].bounds;if(!o)return;let a=o.min.map((c,h)=>c*i.scale[h]),l=o.max.map((c,h)=>c*i.scale[h]);a[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),a[1]=Math.max(a[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:a,max:l})})}var ta={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516],waterline:!0}},Op=new Set(["city.ivory","city.warm"]),Py={low:0,medium:1,high:2,ultra:2},zs=s=>Number(s).toFixed(4),Iy=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,Ly=`
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
#endif`,Dy=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,Ny=`
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
#endif`,Uy=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,Fy=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,Oy=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,By=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,zy=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function ky(s){let e=s?.getAttribute("position");if(!e||s.getAttribute("swSeed"))return;let t=e.count,n=new Int32Array(t),i=s.getIndex();for(let u=0;u<t;u++)n[u]=u;let r=u=>{for(;n[u]!==u;)n[u]=n[n[u]],u=n[u];return u},o=u=>i?i.getX(u):u,a=i?i.count:t;for(let u=0;u+2<a;u+=3){let d=r(o(u));n[r(o(u+1))]=d,n[r(o(u+2))]=d}let l=new Map,c=new Map;for(let u=0;u<t;u++){let d=r(u),f=c.get(d)||[0,0,0];l.set(d,(l.get(d)||0)+1),f[0]+=e.getX(u),f[1]+=e.getY(u),f[2]+=e.getZ(u),c.set(d,f)}let h=new Float32Array(t);for(let u=0;u<t;u++){let d=r(u),f=l.get(d),g=c.get(d);if(f>6){h[u]=2;continue}let v=Math.sin(g[0]/f*12.9898+g[1]/f*78.233+g[2]/f*37.719)*43758.5453;h[u]=v-Math.floor(v)}s.setAttribute("swSeed",new gt(h,1))}function Bp(s,e={}){let t=new AbortController,n=new Map,i=[],r=new Zi(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=Bn,r.needsUpdate=!0;let o={},a={},l={};for(let P of Object.values(ta))for(let W of[P.top,P.side])o[W]||={value:r};for(let P of Object.keys(ta))l[P]={value:new Re(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(P=>new xt(...P));for(;c.length<3;)c.push(new xt(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new be(528926)},swWindow:{value:new xt(.8,1,.06,1)},swFade:{value:new Re(35,140)}},u=2,d=null,f=!1,g=0,v=0,m=0,p=0,y=!1,w=s.capabilities.getMaxAnisotropy(),x=()=>u>1?Math.min(8,w):Math.min(4,w);function A(P,W){let{kind:R,windows:U,space:F}=n.get(W),S=ta[R];Object.assign(P.uniforms,h);let X=["#define SW_LEVEL "+(S?u:0)];F==="world"&&X.push("#define SW_WORLD"),U&&X.push("#define SW_WINDOWS"),S&&(P.uniforms.swTop=o[S.top],P.uniforms.swSide=o[S.side],P.uniforms.swScale=l[R],X.push("#define SW_SURFACE","#define SW_ALBEDO "+zs(S.albedo),"#define SW_ROUGH "+zs(S.rough),"#define SW_BUMP "+zs(S.bump),"#define SW_WETK "+zs(F==="world"?S.wet:S.wet*.5),"#define SW_METAL_TOP "+zs(S.metalTop??1),"#define SW_PUDDLE_DRY "+zs(S.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+zs(S.puddle?.[1]??-2)),S.waterline&&X.push("#define SW_WATERLINE"));let Z=X.join(`
`)+`
`;P.vertexShader=Z+P.vertexShader.replace("#include <common>",`#include <common>
`+Iy).replace("#include <begin_vertex>","#include <begin_vertex>"+Ly),P.fragmentShader=Z+P.fragmentShader.replace("#include <common>",`#include <common>
`+Dy).replace("#include <map_fragment>","#include <map_fragment>"+Ny).replace("#include <color_fragment>","#include <color_fragment>"+Uy).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+Fy).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+Oy).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+By).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+zy)}function T(P,W="world",R=P?.name){if(!P?.isMeshStandardMaterial||n.has(P))return;let U=ta[R]?R:"",F=Op.has(P.name);!U&&!F||(n.set(P,{kind:U,windows:F,space:W}),P.addEventListener("dispose",()=>n.delete(P)),P.onBeforeCompile=S=>A(S,P),P.customProgramCacheKey=()=>["sw",U,F,W,U?u:0].join("|"),P.needsUpdate=!0)}function I(P,W=!1){if(P?.isMesh)for(let R of Array.isArray(P.material)?P.material:[P.material])T(R,W?"object":"world"),Op.has(R?.name)&&ky(P.geometry)}let _=P=>e.url?.(P),D=null;async function E(P){let W=await fetch(_(P.file),{signal:t.signal});if(!W.ok)throw Error("Surface texture unavailable");let R=await W.blob();if(f)return;let U=await createImageBitmap(R,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(f){U.close?.();return}let F=new Xt(U);F.wrapS=F.wrapT=ri,F.colorSpace=Bn,F.flipY=!1,F.anisotropy=x(),F.needsUpdate=!0,i.push(F),o[P.id].value=F,m++,g+=R.size,e.onProgress?.(g,v)}function b(){return d||f||!e.url||(d=(async()=>{if(!D){let R=await fetch(_("manifest.json"),{signal:t.signal});if(!R.ok)throw Error("Surface manifest unavailable");D=await R.json();let U=(D.textures||[]).filter(F=>o[F.id]&&/^[a-z-]+\.webp$/.test(F.file)&&F.tile_metres>0);D.entries=U,v=U.reduce((F,S)=>F+(S.bytes||0),0);for(let F of U)a[F.id]=F.tile_metres;for(let[F,S]of Object.entries(ta))l[F].value.set(1/((a[S.top]||4)/(S.topScale||1)),1/(a[S.side]||4))}let P=D.entries.filter(R=>o[R.id].value===r),W=await Promise.allSettled(P.map(E));p+=W.filter(R=>R.status==="rejected").length})().catch(()=>{p++}).finally(()=>{d=null})),d}return{apply:T,prepare:I,setTier(P){let W=Py[P]??2;if(W!==u){u=W;for(let[R,U]of n)U.kind&&(R.needsUpdate=!0)}for(let R of i)R.anisotropy!==x()&&(R.anisotropy=x(),R.needsUpdate=!0);u>0&&b()},setLighting(P,W,R=0,U=null){U&&h.swSky.value.copy(U).multiplyScalar(1.25);let F=1-P,S=_t.smoothstep(F,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*S+.12*R),.22+F*.98+Math.min(1,W)*.1,.05,.55+.45*F)},setWeather(P){y=P==="rain",h.swRain.value=y?1:0},update(P,W){let R=y?1:0,U=h.swWet.value;if(!W){h.swWet.value=R;return}let F=Math.min(.1,Math.max(0,P));h.swTime.value+=F,h.swWet.value=U+(R-U)*Math.min(1,F/(R>U?14:70))},progress:()=>({bytes:g,expected:v}),stats:()=>({detail:u,textures:m,failures:p,bytes:g,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){f=!0,t.abort();for(let P of i)P.dispose(),P.source.data?.close?.();i.length=0,r.dispose()}}}function Hy(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let o=0;o<s.numberOfChannels;o++){let a=s.getChannelData(o);for(let l=0;l<a.length;l+=e){let c=0,h=Math.min(l+e,a.length);for(let u=l;u<h;u++){let d=a[u];if(!Number.isFinite(d))throw Error("Invalid voice");n=Math.max(n,Math.abs(d)),c+=d*d}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(o=>o>Math.max(.003,n*.003)).sort((o,a)=>o-a),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function zp(s,e){let t=!1,n=!1,i=0,r=null,o=null,a=[],l=null,c=null,h=null,u=250,d=0,f=0,g=0,v="idle",m=null,p=()=>.04+.46/(1+(u/110)**2);function y(){let _=s.currentTime;l?.gain.setTargetAtTime(p(),_,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),_,.18),h?.pan.setTargetAtTime(d,_,.18)}function w(){if(o){o.onended=null;try{o.stop()}catch{}o=null}a.forEach(_=>_.disconnect()),a=[],l=c=h=null}function x(){f++,clearTimeout(i),i=0,r?.abort(),r=null,w(),v="idle"}function A(_=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,I()},_))}function T(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let _=42;for(let D=0;D<2;D++){let E=m.getChannelData(D),b=Math.round(s.sampleRate*.015),P=0;for(let R=0;R<E.length;R++){_=Math.imul(_,1664525)+1013904223>>>0;let U=(R-b)/(E.length-b);E[R]=R<b?0:(_/4294967296*2-1)*Math.pow(1-U,2.25),P+=E[R]*E[R]}let W=.65/Math.sqrt(P||1);for(let R=0;R<E.length;R++)E[R]*=W}return m}async function I(){if(!t||n||r||o||s.state!=="running"){A();return}let _=f,D=new AbortController;r=D,v="loading";let E=setTimeout(()=>D.abort(),32e3);try{let b=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:D.signal});if(b.status===204){v="idle";return}if(!b.ok||!b.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(b.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let P=await b.arrayBuffer();if(P.byteLength>8*1024*1024)throw Error("Voice too large");if(_!==f||!t||n)return;let W=await s.decodeAudioData(P);if(_!==f||!t||n)return;if(!Number.isFinite(W.duration)||W.duration<=0||W.duration>25||W.numberOfChannels>2)throw Error("Invalid voice");let R=Hy(W),U=ce=>(a.push(ce),ce);o=U(s.createBufferSource()),o.buffer=W;let F=U(s.createGain()),S=U(s.createBiquadFilter());F.gain.value=R,S.type="highpass",S.frequency.value=90,c=U(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let X=U(s.createGain()),Z=U(s.createGain()),V=U(s.createGain()),ie=U(s.createGain());X.gain.value=.76,Z.gain.value=.22,V.gain.value=.28;let fe=U(s.createDelay(.2)),Ce=U(s.createConvolver());fe.delayTime.value=.085,Ce.normalize=!1,Ce.buffer=T();let ze=U(s.createWaveShaper()),He=new Float32Array(1024);for(let ce=0;ce<He.length;ce++)He[ce]=Math.max(-.75,Math.min(.75,ce*2/(He.length-1)-1));ze.curve=He,l=U(s.createGain()),l.gain.value=p(),h=U(s.createStereoPanner()),h.pan.value=d,o.connect(F).connect(S).connect(c),c.connect(X).connect(ie),c.connect(fe).connect(Z).connect(ie),c.connect(Ce).connect(V).connect(ie),ie.connect(ze).connect(l).connect(h).connect(e),y(),v="speaking",g++,o.onended=()=>{_!==f||!t||n||(v="tail",i=setTimeout(()=>{w(),v="idle",A()},900))},o.start()}catch{_===f&&!n&&(v=D.signal.aborted?"idle":"unavailable")}finally{clearTimeout(E),r===D&&(r=null),_===f&&!n&&!o&&A(v==="unavailable"?6e4:void 0)}}return{setActive(_){t===_||n||(t=_,t?A(3e3+Math.random()*2e3):x())},setListener(_,D,E,b,P){if(!Number.isFinite(_)||!Number.isFinite(D)||!Number.isFinite(E)||!Number.isFinite(b)||!Number.isFinite(P))return;let W=-_,R=-12-E,U=Math.max(0,Math.hypot(W,R)-13);u=Math.hypot(U,Math.max(0,D-87,-D));let F=Math.hypot(W,R),S=Math.hypot(b,P);d=F&&S?Math.max(-.8,Math.min(.8,(R*b-W*P)/F/S))*.8:0,y()},stats(){return{state:v,phrases:g,distance:u,gain:p(),pan:d,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,x(),m=null)}}}function kp(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,o=!1,a="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let p=0;p<h.length;p++)c=Math.imul(c,1664525)+1013904223|0,h[p]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),d=s.createBiquadFilter(),f=s.createGain();u.buffer=l,u.loop=!0,d.type="lowpass",d.frequency.value=450,f.gain.value=0,u.connect(d).connect(f).connect(t),u.start();let g=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((p,y)=>{let w=y===3?s.createBufferSource():s.createOscillator(),x=s.createGain(),A=s.createStereoPanner(),T=s.createBiquadFilter();return y===3?(w.buffer=l,w.loop=!0):(w.type="sine",w.frequency.value=p.f),T.type="lowpass",T.frequency.value=p.f*2,x.gain.value=0,w.connect(T).connect(x).connect(A).connect(t),w.start(),{...p,source:w,level:x,pan:A,low:T}});function v(){f.gain.setTargetAtTime(r?(a==="rain"?.07:.015)*(o?.12:1):0,s.currentTime,.1),d.frequency.setTargetAtTime(o?280:a==="rain"?2100:600,s.currentTime,.2)}function m(){for(let p of[...n]){try{p.source.stop()}catch{}p.release()}}return{setActive(p){if(r=p,v(),!p){m();for(let y of g)y.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(p,y){o=p,y&&(a=y),v()},listener(p,y,w,x,A){Object.assign(i,{x:p,z:w,fx:x,fz:A});for(let T of g){let I=Math.hypot(p-T.x,w-T.z);T.level.gain.setTargetAtTime(r?T.g/(1+I*I*.025)*(o?.25:1):0,s.currentTime,.1),T.pan.pan.value=Math.max(-1,Math.min(1,((T.x-p)*-A+(T.z-w)*x)/Math.max(1,I)))}},play(p,y=0,w=0,x=0){if(!r||n.size>=16)return;let A=Math.hypot(y-i.x,x-i.z);if(A>90)return;let T={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08]}[p];if(!T)return;let[I,_,D]=T,E=p.startsWith("step")||p==="door"?s.createBufferSource():s.createOscillator();"buffer"in E?E.buffer=l:(E.type="sine",E.frequency.value=_);let b=s.createBiquadFilter(),P=s.createGain(),W=s.createStereoPanner();b.type="lowpass",b.frequency.value=o?_*.7:_,P.gain.setValueAtTime(0,s.currentTime),P.gain.linearRampToValueAtTime(D/(1+A*.08),s.currentTime+.015),P.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+I),W.pan.value=Math.max(-1,Math.min(1,((y-i.x)*-i.fz+(x-i.z)*i.fx)/Math.max(A,1))),E.connect(b).connect(P).connect(W).connect(e);let R={source:E,release(){if(n.delete(R))for(let U of[E,b,P,W])U.disconnect()}};n.add(R),E.onended=R.release,E.start(),E.stop(s.currentTime+I+.02)},stats:()=>({voices:n.size,weather:a,inside:o}),dispose(){r=!1,m(),u.stop();for(let p of g){p.source.stop();for(let y of[p.source,p.level,p.pan,p.low])y.disconnect()}for(let p of[u,d,f])p.disconnect()}}}var Fu=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],Hp=[440,523.25,587.33,659.25,783.99,880,1046.5],Ou=12;function Vy(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let o=0;o<2;o++){let a=i.getChannelData(o);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,a[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function Gy(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,o=!1,a=0,l=null,c=[],h=[],u=new Float32Array(256),d=new Set,f=.18,g=null,v=null,m=null,p=null,y=!1,w="clear",x=[],A=null,T=null,I=null,_=null,D=null,E=null,b=0,P=0,W=0,R=0,U={busy:!1,day:0,evening:0,weather:"clear"},F={ambience:1,effects:1,voice:1};try{o=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function S(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let H=B=>(c.push(B),B),K=B=>(h.push(B),H(B));t=H(e.createGain()),t.gain.value=0,n=H(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),v=H(e.createGain()),v.gain.value=F.ambience,v.connect(t),m=H(e.createGain()),m.gain.value=F.effects,m.connect(t),p=H(e.createGain()),p.gain.value=F.voice,p.connect(t),D=H(e.createConvolver()),D.buffer=Vy(e,3.4,2.6);let Q=H(e.createGain());Q.gain.value=.55,D.connect(Q).connect(v),l=zp(e,p),g=kp(e,m,v),g.environment(y,w);for(let[B,M]of[[55,.07],[82.4069,.022],[110,.01]]){let te=K(e.createOscillator()),he=H(e.createGain());te.frequency.value=B,he.gain.value=M,te.connect(he).connect(v),te.start()}A=H(e.createBiquadFilter()),A.type="lowpass",A.frequency.value=650,A.Q.value=.6,T=H(e.createGain()),T.gain.value=.042;let ee=H(e.createGain());ee.gain.value=.9,A.connect(T).connect(v),T.connect(ee).connect(D),x=Fu[0].map(B=>[-6,6].map(M=>{let te=K(e.createOscillator()),he=H(e.createGain());return te.type="sawtooth",te.frequency.value=B,te.detune.value=M,he.gain.value=.22,te.connect(he).connect(A),te.start(),te}));let ae=K(e.createOscillator()),ve=H(e.createGain());ae.frequency.value=.05,ve.gain.value=180,ae.connect(ve).connect(A.frequency),ae.start();let Ue=e.createBuffer(1,e.sampleRate*4,e.sampleRate),Se=Ue.getChannelData(0),je=0;for(let B=0;B<Se.length;B++)je=(je+(Math.random()*2-1)*.018)/1.018,Se[B]=je;let Ge=Se[Se.length-1]-Se[0];for(let B=0;B<Se.length;B++)Se[B]-=Ge*B/(Se.length-1);let st=K(e.createBufferSource());I=H(e.createBiquadFilter()),_=H(e.createGain()),st.buffer=Ue,st.loop=!0,I.type="lowpass",I.frequency.value=680,I.Q.value=.25,_.gain.value=.32,st.connect(I).connect(_).connect(v),st.start();let j=K(e.createOscillator()),tt=H(e.createGain());j.frequency.value=.075,tt.gain.value=160,j.connect(tt).connect(I.frequency),j.start(),E=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let Je=E.getChannelData(0);for(let B=0;B<Je.length;B++)Je[B]=Math.random()*2-1;ue()}let X=()=>!!e&&o&&r&&!i&&f>0;function Z(H){let K={release(){if(d.delete(K))for(let Q of H){try{Q.stop?.()}catch{}try{Q.disconnect()}catch{}}}};return d.add(K),H[0].onended=K.release,R++,K}function V(H,K,Q){let ee=e.createStereoPanner(),ae=e.createGain();return ee.pan.value=K,ae.gain.value=Q,H.connect(ee).connect(v),ee.connect(ae).connect(D),[ee,ae]}function ie(){let H=e.currentTime,K=Hp[Math.floor(Math.random()*Hp.length)],Q=e.createOscillator(),ee=e.createOscillator(),ae=e.createGain(),ve=e.createGain();Q.frequency.value=K,ee.frequency.value=K*2.76,ae.gain.value=.22,ve.gain.setValueAtTime(0,H),ve.gain.linearRampToValueAtTime(U.busy?.02:.014,H+.008),ve.gain.exponentialRampToValueAtTime(1e-4,H+2.8),Q.connect(ve),ee.connect(ae).connect(ve),Z([Q,ee,ae,ve,...V(ve,Math.random()*1.4-.7,1.4)]),Q.start(H),ee.start(H),Q.stop(H+2.9),ee.stop(H+2.9)}function fe(){let H=e.currentTime,K=e.createBufferSource(),Q=e.createBiquadFilter(),ee=e.createGain(),ae=Math.random()<.5?-1:1;K.buffer=E,K.loop=!0,Q.type="bandpass",Q.Q.value=1.4,Q.frequency.setValueAtTime(260,H),Q.frequency.exponentialRampToValueAtTime(1100,H+1.6),Q.frequency.exponentialRampToValueAtTime(380,H+3.4),ee.gain.setValueAtTime(0,H),ee.gain.linearRampToValueAtTime(.03,H+1.6),ee.gain.exponentialRampToValueAtTime(1e-4,H+3.6),K.connect(Q).connect(ee);let[ve,Ue]=V(ee,-.9*ae,.5);ve.pan.setValueAtTime(-.9*ae,H),ve.pan.linearRampToValueAtTime(.9*ae,H+3.4),Z([K,Q,ee,ve,Ue]),K.start(H),K.stop(H+3.7)}function Ce(){let H=e.currentTime,K=w==="rain"?.36:.32;I.frequency.cancelScheduledValues(H),_.gain.cancelScheduledValues(H),I.frequency.setTargetAtTime(1200+Math.random()*700,H,.8),I.frequency.setTargetAtTime(680,H+2.6,1.4),_.gain.setTargetAtTime(K*1.7,H,.7),_.gain.setTargetAtTime(K,H+2.6,1.5),R++}function ze(){let H=e.currentTime,K=e.createBiquadFilter(),Q=e.createGain();K.type="lowpass",K.frequency.value=420,Q.gain.setValueAtTime(0,H),Q.gain.linearRampToValueAtTime(.028,H+.9),Q.gain.setValueAtTime(.028,H+3.2),Q.gain.exponentialRampToValueAtTime(1e-4,H+5.5);let ee=[69.3,103.83].map(ae=>{let ve=e.createOscillator();return ve.type="sawtooth",ve.frequency.value=ae,ve.detune.value=Math.random()*8-4,ve.connect(K),ve});K.connect(Q),Z([...ee,K,Q,...V(Q,-.55,1.6)]),ee.forEach(ae=>{ae.start(H),ae.stop(H+5.6)})}function He(){let H=e.currentTime,K=2+Math.floor(Math.random()*3),Q=Math.random()*1.2-.6;for(let ee=0;ee<K&&d.size<Ou;ee++){let ae=H+ee*(.12+Math.random()*.1),ve=e.createOscillator(),Ue=e.createGain(),Se=2600+Math.random()*900;ve.frequency.setValueAtTime(Se,ae),ve.frequency.exponentialRampToValueAtTime(Se*1.5,ae+.07),Ue.gain.setValueAtTime(0,ae),Ue.gain.linearRampToValueAtTime(.006,ae+.015),Ue.gain.exponentialRampToValueAtTime(1e-4,ae+.13),ve.connect(Ue),Z([ve,Ue,...V(Ue,Q,.6)]),ve.start(ae),ve.stop(ae+.15)}}function ce(){if(d.size>=Ou)return;let H=Math.random(),K=1-U.day;H<.34?ie():H<.62?fe():H<.8?Ce():K>.5&&U.weather!=="rain"&&H<.88?ze():U.day>.4&&U.weather==="clear"?He():ie()}function Me(){clearTimeout(b),b=0,X()&&(b=setTimeout(()=>{b=0,X()&&(ce(),Me())},(U.busy?2200:4200)+Math.random()*6e3))}function me(){clearTimeout(P),P=0,X()&&(P=setTimeout(()=>{P=0,X()&&(W=(W+1)%Fu.length,x.forEach((H,K)=>H.forEach(Q=>Q.frequency.setTargetAtTime(Fu[W][K],e.currentTime,1.8))),me())},16e3))}function Oe(){clearTimeout(b),clearTimeout(P),b=P=0;for(let H of[...d])H.release()}function ue(){if(!e)return;let H=e.currentTime;A.frequency.setTargetAtTime(U.busy?1500:650+U.day*450+U.evening*150,H,2.5),T.gain.setTargetAtTime((U.busy?.055:.042)*(U.weather==="rain"?.75:1),H,2)}function xe(H=!1){if(clearTimeout(a),!i){if(H&&o)try{S()}catch{o=!1,s(!1);return}e&&(l?.setActive(o&&r&&f>0&&F.voice>0),g?.setActive(o&&r&&f>0),o&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(f,e.currentTime,.25),b||Me(),P||me()):(Oe(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),a=setTimeout(()=>{!i&&(!o||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>o,toggle(){o=!o;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(o))}catch{}xe(!0),s(o)},unlock(){xe(!0)},setActive(H){r!==H&&(r=H,xe())},setVolume(H){Number.isFinite(H)&&(f=Math.max(0,Math.min(.35,H)),xe())},setListener(H,K,Q,ee,ae){l?.setListener(H,K,Q,ee,ae),g?.listener(H,K,Q,ee,ae)},setChannel(H,K){if(!(H in F)||!Number.isFinite(K))return;F[H]=Math.max(0,Math.min(1,K));let Q={ambience:v,effects:m,voice:p}[H];Q&&Q.gain.setTargetAtTime(F[H],e.currentTime,.05),xe()},effect(H,K,Q,ee){g?.play(H,K,Q,ee)},setEnvironment(H,K){y=!!H,["clear","rain","fog"].includes(K)&&(w=K),g?.environment(y,w)},setMood(H={}){Object.assign(U,{busy:!!H.busy,day:Number.isFinite(H.day)?H.day:U.day,evening:Number.isFinite(H.evening)?H.evening:U.evening,weather:["clear","rain","fog"].includes(H.weather)?H.weather:U.weather}),ue()},thunder(H=1){if(!X()||y||d.size>=Ou)return;let K=Math.max(0,Math.min(5,H)),Q=e.currentTime+K,ee=Math.min(1,K/4),ae=e.createBufferSource(),ve=e.createBiquadFilter(),Ue=e.createGain();ae.buffer=E,ae.loop=!0,ve.type="lowpass",ve.Q.value=.7,ve.frequency.setValueAtTime(900-ee*500,Q),ve.frequency.exponentialRampToValueAtTime(70,Q+3.5),Ue.gain.setValueAtTime(0,Q),Ue.gain.linearRampToValueAtTime(.09*(1-ee*.5),Q+.06+ee*.25),Ue.gain.exponentialRampToValueAtTime(.025,Q+1.3),Ue.gain.exponentialRampToValueAtTime(1e-4,Q+5.5),ae.connect(ve).connect(Ue),Z([ae,ve,Ue,...V(Ue,Math.random()*.8-.4,.8)]),ae.start(Q),ae.stop(Q+5.6)},stats(){let H=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let K of u)H+=K*K}return{voice:l?.stats(),effects:g?.stats(),channels:{...F},enabled:o,active:r,state:e?.state||"uninitialized",volume:f,rms:Math.sqrt(H/u.length),soundscape:{chord:W,events:R,transient:d.size,scheduled:!!b,mood:{...U}}}},dispose(){i||(i=!0,Oe(),g?.dispose(),l?.dispose(),clearTimeout(a),h.forEach(H=>{try{H.stop()}catch{}}),c.forEach(H=>H.disconnect()),e&&e.close().catch(()=>{}))}}}var pn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],Ac={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},Bu=new z(122,106,183),zu=class extends In{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new zt(1,1,{type:Jt,samples:n}),this.quad=new xi(new pt({uniforms:Qn.clone(Ni.uniforms),vertexShader:Ni.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},kr=new z(0,22,-12),ks=[];{let s=(r,o,a,l=0,c=0,h=[1,1,1])=>ks.push({asset:r,x:o,z:a,y:l,angle:c,scale:h}),{xs:e,zs:t}=bn;for(let r of t){for(let a of e)s("street-crossing",a,r);let o=[bn.minX,...e,bn.maxX];for(let a=0;a<o.length-1;a++){let l=o[a]+(a?6:0),c=o[a+1]-(a<o.length-2?6:0);s("street-tile",(l+c)/2,r,0,0,[(c-l)/16,1,1])}}for(let r of e){let o=[bn.minZ,...t,bn.maxZ];for(let a=0;a<o.length-1;a++){let l=o[a]+(a?6:0),c=o[a+1]-(a<o.length-2?6:0);s("street-tile",r,(l+c)/2,0,Math.PI/2,[(c-l)/16,1,1])}}for(let r of t)for(let o of[-55,-31,31,55])s("street-lamp",o,r-4.5,.45),s("planter",o+4,r-4.5,.45);for(let{x:r,z:o,scale:a}of Ec)s("data-tower-a",r,o,0,0,[1,a,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,.5),s("server-rack",-49,-36.7,.5);let n=7919,i=()=>(n=Math.imul(n,1664525)+1013904223>>>0)/4294967296;for(let r=0;r<4;r++)for(let o=0;o<12;o++){let a=i(),l=i(),c=i(),h=i(),u=i(),d=i();if(h<.12)continue;let f=(o-5.5)*22+(a-.5)*12,g=-143-r*29+(l-.5)*14,v=1-Math.min(1,Math.abs(f)/150),m=(.5+c*c+.45*v)*(r?1:.85),p=.7+.34*u;s(a>.5?"data-tower-a":"data-tower-b",f,g,-3,d<.3?Math.PI/2:0,[p,m,p])}for(let r=0;r<16;r++){let o=(r-7.5)*34+(i()-.5)*16,a=-268-i()*46;s(i()<.5?"data-tower-a":"data-tower-b",o,a,-3,0,[1.1,.55+i()*1.1,1.1])}}var Wy=[...np(ks),...hn.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function ow(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",o=r==="auto"?"high":r,a=0,l=null,c=0,h=0,u=0,d=0,f=0,g=0,v=0,m=null,p=e.reducedMotion,y=1,w=1,x=[],A=!1,T=null,I=null,_=new AbortController,D=new Set,E=new Map,b=new Map,P=new Set,W=new Set,R=new Set,U=[],F=new cc({antialias:!0,alpha:!1});F.toneMapping=Ls,F.toneMappingExposure=.98,F.shadowMap.type=pl,F.shadowMap.autoUpdate=!1,F.info.autoReset=!1;let S=F.domElement;S.className="sysworld-gl",S.tabIndex=0,S.setAttribute("aria-label",e.label),s.append(S);let X=new Es;X.background=new be(528926),X.fog=new ro(528926,.004);let Z=new Zt(43,1,.3,1800);Z.position.copy(Bu);let V=Sc(),ie=V.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},Z.position,100),fe=new mc(Z,S);fe.target.copy(kr),fe.enableDamping=!0,fe.dampingFactor=.085,fe.minDistance=12,fe.maxDistance=410,fe.maxPolarAngle=Math.PI*.485,fe.update();let Ce=0,ze=()=>{let $=He?.progress();e.onProgress?.(f+($?.bytes||0),Ce+($?.expected||0))},He=Bp(F,{url:$=>e.resourceURL("/3d/system-world/textures/v1/"+$),onProgress:()=>ze(),shelters:hn.map($=>[$.x-$.width/2,$.z-$.depth/2,$.x+$.width/2,$.z+$.depth/2])}),ce=new Lr(F),Me=new gc,me=ce.fromScene(Me,.05);X.environment=me.texture,X.environmentIntensity=.48,Me.dispose(),ce.dispose();let Oe=new bo(12639487,1056813,.8);X.add(Oe);let ue=new $i(13164287,3.2);ue.position.set(-70,145,85),ue.castShadow=!0,Object.assign(ue.shadow.camera,{left:-120,right:120,top:120,bottom:-120,near:1,far:360}),ue.shadow.normalBias=.09,ue.shadow.bias=-15e-5,X.add(ue);let xe=new $i(16758130,2.4);xe.position.set(90,80,-120),X.add(xe);let H=new Kn(6479871,0,75,2);H.position.set(0,34,-12),X.add(H);let K=new zt(1,1,{type:Jt}),Q=new _c(F,K);Q.addPass(new zu(X,Z,Math.min(4,F.capabilities.maxSamples)));let ee=new Or(new Re(1,1),.28,.55,1.25);Q.addPass(ee),Q.addPass(new vc);let ae=new dt,ve=new dt,Ue=new dt;X.add(ae),ae.add(ve,Ue);function Se($,Be){return P.add($),W.add(Be),new Ke($,Be)}let je=Se(new ci(170,3,174),new xn({color:1911345,metalness:.05,roughness:.82}));je.position.set(0,-1.5,-7),je.receiveShadow=!0,ae.add(je),He.apply(je.material,"world","ground");let Ge=_p(X,{sunDirection:ue.position,lamps:ks.filter($=>$.asset==="street-lamp")});Q.addPass(Ge.post);let st=pn.find($=>$.id==="memory"),j=cp(X,st,{roof:21,label:e.memoryLabel}),tt=yp(X,{traffic:V,surfaces:He}),Je=Up(X,{sun:ue,rim:xe,hemisphere:Oe,atmosphere:Ge,surfaces:He,onThunder:$=>e.onThunder?.($)}),B="",M=null,te={value:0},he=new Cs(1,1.045,96);P.add(he);let de=($,Be)=>{let Ye=new pt({transparent:!0,depthWrite:!1,side:At,uniforms:{color:{value:new be(9365742)},time:te,opacity:{value:$},dashes:{value:Be}},vertexShader:"varying vec2 vP;void main(){vP=position.xy;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"uniform vec3 color;uniform float time,opacity,dashes;varying vec2 vP;void main(){float a=atan(vP.y,vP.x)/6.2831853;float dash=dashes>0.5?step(0.3,fract(a*dashes-time*0.35)):1.0;gl_FragColor=vec4(color,opacity*dash*(0.8+0.2*sin(time*2.2)));}"});return W.add(Ye),Ye},Te=new Ke(he,de(.9,18)),Le=new Ke(he,de(.38,0));for(let $ of[Te,Le])$.rotation.x=-Math.PI/2,$.visible=!1,ae.add($);let pe={error:16742504,running:7730385,stale:8096667,idle:8175587},_e={},De=()=>Te.material.uniforms.color.value.setHex(pe[_e[m]]??9365742),We=new Mt({color:6742230,toneMapped:!1}),Pe=new gn(new hi(.65,10,6),We,pn.length);P.add(Pe.geometry),W.add(We),ae.add(Pe);let Fe=new bt,O=new z,q=new Co,Y=new Re;pn.forEach(($,Be)=>{Fe.position.set($.x,$.height+2,$.z),Fe.updateMatrix(),Pe.setMatrixAt(Be,Fe.matrix),Pe.setColorAt(Be,new be(8425630))}),e.signal?.addEventListener("abort",us,{once:!0});let N;try{if(e.signal?.aborted)throw Error("Disposed");let $=await fetch(e.assetURL("manifest.json"),{signal:_.signal});if(!$.ok)throw Error("City manifest unavailable");N=await $.json()}catch($){throw us(),$}let L=new Map(N.assets.map($=>[$.id,$]));Fp(V,L,pn,ks),M=Dp(X,{camera:Z,districts:pn,tier:o,traffic:V,reduced:()=>p,active:()=>n&&!A&&i!=="map"&&!p&&!e.replaying?.(),assetURL:$=>e.resourceURL("/3d/system-world/v2/"+$),surfaces:He,onInteraction:e.onInteraction,onScope:$=>e.onScope?.($),onDiscover:e.onDiscover,onSound:e.onSound,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:$=>{Je.setIndoor($),e.onEnvironment?.($)},onError:e.onError,onReady:()=>{F.shadowMap.needsUpdate=!0}});async function k($,Be){let Ye=$+":"+Be;return E.has(Ye)||E.set(Ye,(async()=>{let C=L.get($)?.lods.find(ne=>ne.level===Be);if(!C||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(C.file))throw Error("Invalid city asset");let J=new AbortController;D.add(J);let oe=setTimeout(()=>J.abort(),15e3);try{let ne=await fetch(e.assetURL(C.file),{signal:J.signal});if(!ne.ok)throw Error("City asset unavailable");let re=await ne.arrayBuffer();if(t)throw Error("Disposed");let Ee=await new os().parseAsync(re,"");if(t)throw Ee.scene.traverse(we=>{we.isMesh&&(we.geometry.dispose(),we.material.dispose())}),Error("Disposed");return f+=re.byteLength,ze(),Ee.scene.traverse(we=>{if(!we.isMesh)return;P.add(we.geometry),we.castShadow=!0,we.receiveShadow=!0;let Ne=(Array.isArray(we.material)?we.material:[we.material]).map(Ie=>{if(b.has(Ie.name)){let Xe=Ie;Ie=b.get(Ie.name),Xe.dispose()}else b.set(Ie.name,Ie),W.add(Ie);return Ie});we.material=Array.isArray(we.material)?Ne:Ne[0],He.prepare(we)}),Ee.scene}finally{clearTimeout(oe),D.delete(J)}})().catch(C=>{throw E.delete(Ye),C})),E.get(Ye)}function G($){$.traverse(Be=>{Be.isInstancedMesh&&Be.dispose()}),$.clear()}async function le(){let $=++a,Be=Ac[o],Ye=Be.lod,C=[...new Set([...ks.map(oe=>oe.asset),...pn.map(oe=>oe.asset),"service-drone"])],J=(oe,ne)=>L.get(oe)?.lods.find(re=>re.level===ne)?.bytes||0;Ce=C.reduce((oe,ne)=>oe+(E.has(ne+":"+Ye)?0:J(ne,Ye)),f)+(Ye===2?0:["data-tower-a","data-tower-b"].reduce((oe,ne)=>oe+(E.has(ne+":2")?0:J(ne,2)),0));try{let oe=new Map(await Promise.all(C.map(async ye=>[ye,await k(ye,Ye)]))),ne=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async ye=>[ye,await k(ye,2)])));if(t||$!==a)return;G(ve),G(Ue),x=[];let re=new Map;for(let ye of ks){let Ne=(ye.z<-100?ne:oe).get(ye.asset);Ne.updateMatrixWorld(!0);let Ie=new it().compose(new z(ye.x,ye.y,ye.z),new Gt().setFromAxisAngle(new z(0,1,0),ye.angle),new z(...ye.scale));Ne.traverse(Xe=>{if(!Xe.isMesh)return;let nt=Xe.uuid;re.has(nt)||re.set(nt,{node:Xe,matrices:[]}),re.get(nt).matrices.push(new it().multiplyMatrices(Ie,Xe.matrixWorld))})}for(let{node:ye,matrices:Ne}of re.values()){let Ie=new gn(ye.geometry,ye.material,Ne.length);Ne.forEach((Xe,nt)=>Ie.setMatrixAt(nt,Xe)),Ie.castShadow=!0,Ie.receiveShadow=!0,ve.add(Ie)}for(let ye of pn){let Ne=oe.get(ye.asset).clone(!0);Ne.position.set(ye.x,0,ye.z),Ne.userData.district=ye.id,Ue.add(Ne),x.push(Ne)}I?.attachLandmarks(Ue),tt.setTemplate(oe.get("service-drone"));let Ee=new Kt,we=new Map;for(let ye of["data-tower-a","data-tower-b"])we.set(ye,Ee.setFromObject(ne.get(ye)).max.y);Ge.setAviation(ks.filter(ye=>we.has(ye.asset)&&(ye.z<-100||ye.scale[1]>1.1)).map(ye=>({x:ye.x,y:ye.y+we.get(ye.asset)*ye.scale[1]+.6,z:ye.z}))),F.shadowMap.needsUpdate=!0,e.onReady?.()}catch(oe){!t&&$===a&&e.onError?.(oe)}}function se(){if(t)return;y=Math.max(1,s.clientWidth),w=Math.max(1,s.clientHeight);let $=Math.min(devicePixelRatio||1,Ac[o].dpr,Math.sqrt(3840*2160/(y*w)));F.setPixelRatio($),F.setSize(y,w,!1),Q.setPixelRatio($),Q.setSize(y,w),Z.aspect=y/w,Z.updateProjectionMatrix(),j.setPointScale(w*$),Ge.setPointScale(w*$)}function ge($){return r=$ in Ac||$==="auto"?$:"auto",o=r==="auto"?"high":r,Ae(),le()}function Ae(){let $=Ac[o];F.shadowMap.enabled=$.shadow>0,$.shadow&&ue.shadow.mapSize.x!==$.shadow&&(ue.shadow.mapSize.set($.shadow,$.shadow),ue.shadow.map?.dispose(),ue.shadow.map=null),F.shadowMap.needsUpdate=!0,ee.enabled=$.bloom,He.setTier(o),Ge.setTier(o),M.setTier(o),Je.setTier(o),tt.setTier(o),se(),e.onQuality?.(r,o)}function at($,Be){l=null;let Ye=V.findFree(ie,$);if(!Ye)return;if($=new z(Ye.x,Ye.y,Ye.z),p){Z.position.copy($),Object.assign(ie,Ye),fe.target.copy(Be),fe.update(),l=null;return}let C=(re,Ee)=>V.clear(ie,{...re,heading:0},{...Ee,heading:0},!1),J=Z.position.clone(),oe=Math.max(J.y,$.y,110),ne=[J,$];if(!C(J,$)){let re=[J],Ee=new z($.x,oe,$.z),we=[-1],ye=[0],Ne=-1;if(!C(Ee,$))return;for(let Ie of[3,8,16])for(let Xe=0;Xe<8;Xe++)re.push(new z(J.x+Math.cos(Xe*Math.PI/4)*Ie,J.y,J.z+Math.sin(Xe*Math.PI/4)*Ie));for(let Ie of hn)if(Math.hypot(J.x-Ie.x,J.z-Ie.z)<24)for(let Xe of[Ie.z,Ie.doorZ-Ie.front*2,Ie.doorZ+Ie.front*3])re.push(new z(Ie.x,J.y,Xe));for(;ye.length;){let Ie=ye.shift(),Xe=re[Ie],nt=new z(Xe.x,oe,Xe.z);if(C(Xe,nt)&&C(nt,Ee)){Ne=Ie;break}for(let qe=1;qe<re.length;qe++)we[qe]===void 0&&Xe.distanceTo(re[qe])<=20&&C(Xe,re[qe])&&(we[qe]=Ie,ye.push(qe))}if(Ne<0)return;ne=[];for(let Ie=Ne;Ie>=0;Ie=we[Ie])ne.unshift(re[Ie]);ne.push(new z(re[Ne].x,oe,re[Ne].z),Ee,$)}l={start:Z.position.clone(),targetStart:fe.target.clone(),end:$.clone(),target:Be.clone(),time:0,waypoints:ne}}function Qe(){V.relocate(ie,Z.position),Z.position.set(ie.x,ie.y,ie.z)}function Lt($){let Be=pn.find(Ye=>Ye.id===$);Be&&(m=$,Te.position.set(Be.x,.55,Be.z),Te.scale.setScalar(Be.radius*1.3),Te.visible=!0,De(),Le.visible=!!Fi&&Fi!==m,i==="street"?(e.onCut?.(),Z.position.set(Be.x,2.4,Be.z+Be.radius+7),Z.lookAt(Be.x,Be.height*.4,Be.z),Qe()):at(new z(Be.x+Be.radius*2.7,Be.height*.7+18,Be.z+Be.radius*4),new z(Be.x,Be.height*.4,Be.z)))}function rt($){M.endRide(),M.society.suspend(),R.clear(),l=null,i=$,fe.enabled=i==="orbit"||i==="tour",ie.minY=i==="street"?-2.3:-.4,ie.ignore=ie.follow=null,ie.circles[0].r=ie.reach=i==="street"?.24:.38,document.pointerLockElement===S&&document.exitPointerLock(),i==="street"?(e.onCut?.(),Hr(),Z.position.set(18,2.4,57),Z.lookAt(0,26,-12),Qe()):i!=="map"&&(Qe(),at(Bu.clone().multiplyScalar(Z.aspect<1?1.3:1),kr)),g=0,v=0,e.onMode?.(i)}let Nt=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};fe.addEventListener("start",Nt);function Ot($,Be,Ye,C){$.addEventListener(Be,Ye,C),U.push(()=>$.removeEventListener(Be,Ye,C))}let Dt=null,vi=!1,cs=new mn(0,0,0,"YXZ");function na($,Be){let Ye=.0025*Z.fov/43;cs.setFromQuaternion(Z.quaternion),cs.y-=$*Ye,cs.x=_t.clamp(cs.x-Be*Ye,-1.35,1.35),Z.quaternion.setFromEuler(cs)}Ot(S,"pointerdown",$=>{S.focus({preventScroll:!0}),Nt(),Dt=[$.clientX,$.clientY],vi=!1,i==="street"&&S.setPointerCapture($.pointerId)});let Fi=null,ti=-1e9;function Hs($){if($===Fi)return;Fi=$,S.style.cursor=$?"pointer":"";let Be=pn.find(Ye=>Ye.id===$);Le.visible=!!Be&&$!==m,Be&&(Le.position.set(Be.x,.56,Be.z),Le.scale.setScalar(Be.radius*1.3)),e.onHover?.($)}function Hr(){Hs(null)}function ia($){if(i==="street"||i==="map"||Dt||$.timeStamp-ti<80)return;ti=$.timeStamp;let Be=S.getBoundingClientRect();Y.set(($.clientX-Be.left)/Be.width*2-1,1-($.clientY-Be.top)/Be.height*2),q.setFromCamera(Y,Z);let Ye=q.intersectObjects(x,!0)[0]?.object;for(;Ye&&!Ye.userData.district;)Ye=Ye.parent;Hs(Ye?.userData.district||null)}Ot(S,"pointerleave",Hr),Ot(S,"pointermove",$=>{i==="street"&&(document.pointerLockElement===S||Dt)&&na($.movementX,$.movementY),ia($),Dt&&Math.hypot($.clientX-Dt[0],$.clientY-Dt[1])>5&&(vi=!0)}),Ot(S,"pointerup",$=>{if(Dt&&!vi&&i!=="street"){let Be=S.getBoundingClientRect();Y.set(($.clientX-Be.left)/Be.width*2-1,1-($.clientY-Be.top)/Be.height*2),q.setFromCamera(Y,Z);let Ye=q.intersectObjects(x,!0)[0];if(Ye){let C=Ye.object;for(;C&&!C.userData.district;)C=C.parent;C&&e.onSelect?.(C.userData.district)}}Dt=null}),Ot(S,"pointercancel",()=>{Dt=null,R.clear()}),Ot(S,"keydown",$=>{if($.key==="Escape"){M.scoping()?M.endRide():rt("orbit"),$.preventDefault();return}if(i==="street"&&["+","=","-"].includes($.key)&&M.zoom($.key==="-"?240:-240)){$.preventDefault();return}if(i==="street"&&$.code==="KeyE"&&!$.repeat){M.interact(),$.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes($.code)&&(R.add($.code),$.preventDefault(),$.stopPropagation())}),Ot(S,"keyup",$=>R.delete($.code)),Ot(S,"wheel",$=>{i==="street"&&M.zoom($.deltaY)&&$.preventDefault()},{passive:!1}),Ot(S,"blur",()=>R.clear()),Ot(window,"blur",()=>{R.clear(),Dt=null}),Ot(S,"webglcontextlost",$=>{$.preventDefault(),A=!0,R.clear(),e.onContextLost?.()}),T=new ResizeObserver(se),T.observe(s);function Vs($){let Be=(R.has("ShiftLeft")?25:11)*$,Ye=Number(R.has("KeyW")||R.has("ArrowUp"))-Number(R.has("KeyS")||R.has("ArrowDown")),C=Number(R.has("KeyD")||R.has("ArrowRight"))-Number(R.has("KeyA")||R.has("ArrowLeft"));if(!Ye&&!C||M.walkRide(Ye,$,C))return;let J=Be/Math.hypot(Ye,C);Z.getWorldDirection(O),O.y=0,O.normalize();let oe=(O.x*Ye-O.z*C)*J,ne=(O.z*Ye+O.x*C)*J,re=(Ee,we)=>M.move(Ee,we,Z.position)&&V.clear(ie,ie,{x:Ee,y:2.4+M.floor(Ee,we),z:we,heading:0});re(Z.position.x+oe,Z.position.z)&&(Z.position.x+=oe),re(Z.position.x,Z.position.z+ne)&&(Z.position.z+=ne),M.isRiding()||(Z.position.y=2.4+M.floor(Z.position.x,Z.position.z))}let Vr=0,hs=sp($=>{V.begin(),Vr+=p?0:$,i==="street"&&Vs($),M.update($,!p,i);let Be=ie.follow;ie.follow=M.rideBody(),ie.ignore=ie.follow||Be,I?.update($,!p),tt.update($,Vr,!p),V.propose(ie,{...Z.position,heading:0},(Ye,C)=>Z.position.set(C.x,C.y,C.z)),V.solve($),M.syncRide?.(),ie.follow&&Object.assign(ie,{x:Z.position.x,y:Z.position.y,z:Z.position.z}),ie.ignore=ie.follow});function sa($,Be){if(t||!n||A||i==="map")return;let Ye=null,C=0,J=null;if(i!=="street"){if(i==="tour"&&(g-=$,g<=0)){let Ee=pn[v++%pn.length].id;Lt(Ee),e.onTourFocus?.(Ee),g=7}if(l){Ye=l,C=l.time,l.time+=$;let Ee=Math.min(1,l.time/(l.duration||(l.waypoints.length===2?1.1:3.2))),we=Ee*Ee*(3-2*Ee),ye=Math.min(l.waypoints.length-1-1e-6,we*(l.waypoints.length-1)),Ne=Math.floor(ye);Z.position.lerpVectors(l.waypoints[Ne],l.waypoints[Ne+1],ye-Ne),fe.target.lerpVectors(l.targetStart,l.target,we),Z.lookAt(fe.target),J=Z.position.clone(),Ee===1&&(l=null)}else fe.update()}hs(Math.min(.1,Math.max(0,$)))||(V.begin(),V.propose(ie,{...Z.position,heading:0},(Ee,we)=>Z.position.set(we.x,we.y,we.z)),V.solve(0)),Ye&&(Z.position.distanceToSquared(J)>1e-6?(Ye.wait=(Ye.wait||0)+$,Ye.time=C,l=Ye.wait<6?Ye:null):Ye.wait=0),Z.getWorldDirection(O),e.onListener?.(Z.position.x,Z.position.y,Z.position.z,O.x,O.z);let oe=!!e.busy?.();Je.update($,Be,!p)&&(F.shadowMap.needsUpdate=!0),H.intensity=p?0:oe?180+Math.sin(Be*2)*35:0,Ge.setBusy(oe),Ge.setCinematic(i==="tour"),Ge.update($,Be,Z,!p),He.update($,!p),p||(te.value+=Math.min(.1,Math.max(0,$)));let ne=Je.mood(),re=[oe,ne.day.toFixed(1),ne.evening.toFixed(1),ne.weather].join();if(ee.strength=.2+.15*(1-ne.day),F.toneMappingExposure=.95+.05*(1-ne.day),re!==B&&(B=re,e.onMood?.({busy:oe,day:ne.day,evening:ne.evening,weather:ne.weather})),j.update($,Z,!p),F.info.reset(),Q.render(),c++,r==="auto"&&$>0&&$<.2&&Be-d>12&&(h+=$,u++,u>=180)){let Ee=h/u,we=["low","medium","high"],ye=we.indexOf(o),Ne=Ee>.028&&ye>0?we[ye-1]:Ee<.017&&ye<2?we[ye+1]:o;u=0,h=0,Ne!==o&&(o=Ne,d=Be,Ae(),le())}}function ra($,Be){I?.setData($,Be),pn.forEach((Ye,C)=>{let J=$.find(oe=>oe.id===Ye.id);_e[Ye.id]=!J||J.stale?"stale":J.state==="error"?"error":J.state==="running"?"running":"idle",Pe.setColorAt(C,new be(pe[_e[Ye.id]]))}),Pe.instanceColor.needsUpdate=!0,De()}function us(){t||(t=!0,a++,_.abort(),D.forEach($=>$.abort()),R.clear(),document.pointerLockElement===S&&document.exitPointerLock(),e.signal?.removeEventListener("abort",us),U.forEach($=>$()),T?.disconnect(),fe.dispose(),I?.dispose(),M?.dispose(),Je.dispose(),j.dispose(),tt.dispose(),V.dispose(),Ge.dispose(),He.dispose(),G(ve),G(Ue),Pe.dispose(),P.forEach($=>$.dispose()),W.forEach($=>$.dispose()),Q.passes.forEach($=>$.dispose?.()),Q.dispose(),me.dispose(),ue.shadow.dispose(),F.dispose(),F.forceContextLoss(),S.remove(),E.clear(),b.clear())}I=op(X,pn,{traffic:V,society:M.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!A&&i!=="map"&&!e.replaying?.(),obstacles:Wy}),j.setReducedMotion(!!p);try{Ae(),await le()}catch($){throw us(),$}return{districts:pn,canvas:S,update:sa,focus($){Nt(),Lt($)},setMode:rt,setQuality:ge,setData:ra,dispose:us,interact(){M.interact()},socialAction:M.socialAction,visit($){rt("street"),M.visit($),Qe()},enter($){rt("street"),M.destination($),Qe()},setEnvironment($){Je.set($),F.shadowMap.needsUpdate=!0},setVisible($){n=$,$||(hs(0),M.suspend(),I?.update(0,!1),R.clear(),Dt=null,document.pointerLockElement===S&&document.exitPointerLock())},setReducedMotion($){p=$,j.setReducedMotion(!!$),$&&(l=null,M.endRide(),M.society.suspend()),$&&i==="tour"&&rt("orbit")},setHologram($,Be){j.setTexts($,Be),M.setMemory($)},setWorld($,Be){M.setWorld($,Be)},moveKey($,Be){Be?R.add($):R.delete($)},lockPointer(){if(i==="street")return S.requestPointerLock()},project($){let Be=pn.find(C=>C.id===$);if(!Be)return null;O.set(Be.x,Be.height+4,Be.z);let Ye=Z.position.distanceTo(O);return O.project(Z),{x:(O.x+1)*y/2,y:(1-O.y)*w/2,visible:O.z<1&&O.z>-1,distance:Ye}},capture(){return t||A?Promise.resolve(null):(Q.render(),new Promise($=>S.toBlob($,"image/png")))},intro(){if(p||i!=="orbit"||t)return;let $=Bu.clone().multiplyScalar(Z.aspect<1?1.3:1);Z.position.copy($).sub(kr).applyAxisAngle(new z(0,1,0),-.55).multiplyScalar(1.22).add(kr),Z.position.y+=34,Z.lookAt(kr),Qe(),at($,kr),l&&(l.duration=3.6)},stats(){return{flying:!!l,traffic:V.stats(),experience:M.stats(),weather:Je.stats(),life:I?.stats(),hologram:j.stats(),atmosphere:Ge.stats(),surfaces:He.stats(),drones:tt.stats(),frames:c,tier:o,mode:i,focusedDistrict:m,hovered:Fi,loadedBytes:f+M.stats().bytes+He.stats().bytes,cachedModels:E.size,calls:F.info.render.calls,triangles:F.info.render.triangles,geometries:F.info.memory.geometries,position:Z.position.toArray(),renderer:F.getContext().getParameter(F.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||F.getContext().RENDERER),disposed:t}}}}export{ow as createCity,Gy as createCityAmbience,pn as districts,Wy as obstacles,ks as placements};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
