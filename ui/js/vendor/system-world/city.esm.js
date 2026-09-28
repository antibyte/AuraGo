var is={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},ss={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},Md=0,ch=1,bd=2;var Ro=1,al=2,Er=3,In=0,tn=1,Nt=2,Wn=0,bs=1,Xt=2,hh=3,uh=4,Sd=5;var Ji=100,Td=101,Ed=102,wd=103,Ad=104,Rd=200,Cd=201,Pd=202,Id=203,wa=204,Aa=205,Ld=206,Dd=207,Nd=208,Ud=209,Fd=210,Od=211,Bd=212,zd=213,kd=214,Ra=0,Ca=1,Pa=2,Ss=3,Ia=4,La=5,Da=6,Na=7,ll=0,Hd=1,Vd=2,ti=0,Co=1,Po=2,Io=3,Ns=4,Lo=5,Do=6,No=7,Jc="attached",Gd="detached",dh=300,rs=301,Us=302,cl=303,hl=304,Uo=306,$i=1e3,Hn=1001,rr=1002,Kt=1003,ul=1004;var Fs=1005;var Vt=1006,wr=1007;var ni=1008;var Sn=1009,fh=1010,ph=1011,Ar=1012,dl=1013,ii=1014,Nn=1015,nn=1016,fl=1017,pl=1018,Rr=1020,mh=35902,gh=35899,xh=1021,_h=1022,Un=1023,hi=1026,os=1027,ml=1028,gl=1029,as=1030,xl=1031;var _l=1033,Fo=33776,Oo=33777,Bo=33778,zo=33779,vl=35840,yl=35841,Ml=35842,bl=35843,Sl=36196,Tl=37492,El=37496,wl=37488,Al=37489,ko=37490,Rl=37491,Cl=37808,Pl=37809,Il=37810,Ll=37811,Dl=37812,Nl=37813,Ul=37814,Fl=37815,Ol=37816,Bl=37817,zl=37818,kl=37819,Hl=37820,Vl=37821,Gl=36492,Wl=36494,Xl=36495,ql=36283,Yl=36284,Ho=36285,Zl=36286,Kl=2200,Wd=2201,Xd=2202,Ts=2300,Es=2301,Ea=2302,$c=2303,vs=2400,ys=2401,$r=2402,jl=2500,qd=2501,vh=0,Vo=1,Cr=2,Yd=3200;var Go=0,Zd=1,Bi="",Ht="srgb",mn="srgb-linear",Qr="linear",yt="srgb";var _s=7680;var Qc=519,Kd=512,jd=513,Jd=514,Jl=515,$d=516,Qd=517,$l=518,ef=519,Ua=35044;var yh="300 es",Jn=2e3,or=2001;function Lp(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Dp(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function ar(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function tf(){let s=ar("canvas");return s.style.display="block",s}var Du={},lr=null;function eo(...s){let e="THREE."+s.shift();lr?lr("log",e,...s):console.log(e,...s)}function nf(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function Xe(...s){s=nf(s);let e="THREE."+s.shift();if(lr)lr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function Qe(...s){s=nf(s);let e="THREE."+s.shift();if(lr)lr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function Ms(...s){let e=s.join(" ");e in Du||(Du[e]=!0,Xe(...s))}function sf(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var rf={[Ra]:Ca,[Pa]:Da,[Ia]:Na,[Ss]:La,[Ca]:Ra,[Da]:Pa,[Na]:Ia,[La]:Ss},Vn=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,o=i.length;r<o;r++)i[r].call(this,e);e.target=null}}},hn=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],Nu=1234567,Zr=Math.PI/180,ws=180/Math.PI;function $n(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(hn[s&255]+hn[s>>8&255]+hn[s>>16&255]+hn[s>>24&255]+"-"+hn[e&255]+hn[e>>8&255]+"-"+hn[e>>16&15|64]+hn[e>>24&255]+"-"+hn[t&63|128]+hn[t>>8&255]+"-"+hn[t>>16&255]+hn[t>>24&255]+hn[n&255]+hn[n>>8&255]+hn[n>>16&255]+hn[n>>24&255]).toLowerCase()}function ot(s,e,t){return Math.max(e,Math.min(t,s))}function Mh(s,e){return(s%e+e)%e}function Np(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function Up(s,e,t){return s!==e?(t-s)/(e-s):0}function Kr(s,e,t){return(1-t)*s+t*e}function Fp(s,e,t,n){return Kr(s,e,1-Math.exp(-t*n))}function Op(s,e=1){return e-Math.abs(Mh(s,e*2)-e)}function Bp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function zp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function kp(s,e){return s+Math.floor(Math.random()*(e-s+1))}function Hp(s,e){return s+Math.random()*(e-s)}function Vp(s){return s*(.5-Math.random())}function Gp(s){s!==void 0&&(Nu=s);let e=Nu+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function Wp(s){return s*Zr}function Xp(s){return s*ws}function qp(s){return(s&s-1)===0&&s!==0}function Yp(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function Zp(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function Kp(s,e,t,n,i){let r=Math.cos,o=Math.sin,a=r(t/2),l=o(t/2),c=r((e+n)/2),h=o((e+n)/2),u=r((e-n)/2),d=o((e-n)/2),f=r((n-e)/2),g=o((n-e)/2);switch(i){case"XYX":s.set(a*h,l*u,l*d,a*c);break;case"YZY":s.set(l*d,a*h,l*u,a*c);break;case"ZXZ":s.set(l*u,l*d,a*h,a*c);break;case"XZX":s.set(a*h,l*g,l*f,a*c);break;case"YXY":s.set(l*f,a*h,l*g,a*c);break;case"ZYZ":s.set(l*g,l*f,a*h,a*c);break;default:Xe("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function jn(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function Et(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var Ct={DEG2RAD:Zr,RAD2DEG:ws,generateUUID:$n,clamp:ot,euclideanModulo:Mh,mapLinear:Np,inverseLerp:Up,lerp:Kr,damp:Fp,pingpong:Op,smoothstep:Bp,smootherstep:zp,randInt:kp,randFloat:Hp,randFloatSpread:Vp,seededRandom:Gp,degToRad:Wp,radToDeg:Xp,isPowerOfTwo:qp,ceilPowerOfTwo:Yp,floorPowerOfTwo:Zp,setQuaternionFromProperEuler:Kp,normalize:Et,denormalize:jn},ve=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=ot(this.x,e.x,t.x),this.y=ot(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=ot(this.x,e,t),this.y=ot(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ot(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(ot(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,o=this.y-e.y;return this.x=r*n-o*i+e.x,this.y=r*i+o*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},Zt=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,o,a){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],d=r[o+0],f=r[o+1],g=r[o+2],M=r[o+3];if(u!==M||l!==d||c!==f||h!==g){let m=l*d+c*f+h*g+u*M;m<0&&(d=-d,f=-f,g=-g,M=-M,m=-m);let p=1-a;if(m<.9995){let v=Math.acos(m),S=Math.sin(v);p=Math.sin(p*v)/S,a=Math.sin(a*v)/S,l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+M*a}else{l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+M*a;let v=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=v,c*=v,h*=v,u*=v}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,o){let a=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[o],d=r[o+1],f=r[o+2],g=r[o+3];return e[t]=a*g+h*u+l*f-c*d,e[t+1]=l*g+h*d+c*u-a*f,e[t+2]=c*g+h*f+a*d-l*u,e[t+3]=h*g-a*u-l*d-c*f,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,o=e._order,a=Math.cos,l=Math.sin,c=a(n/2),h=a(i/2),u=a(r/2),d=l(n/2),f=l(i/2),g=l(r/2);switch(o){case"XYZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"YXZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"ZXY":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"ZYX":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"YZX":this._x=d*h*u+c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u-d*f*g;break;case"XZY":this._x=d*h*u-c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u+d*f*g;break;default:Xe("Quaternion: .setFromEuler() encountered an unknown order: "+o)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],o=t[1],a=t[5],l=t[9],c=t[2],h=t[6],u=t[10],d=n+a+u;if(d>0){let f=.5/Math.sqrt(d+1);this._w=.25/f,this._x=(h-l)*f,this._y=(r-c)*f,this._z=(o-i)*f}else if(n>a&&n>u){let f=2*Math.sqrt(1+n-a-u);this._w=(h-l)/f,this._x=.25*f,this._y=(i+o)/f,this._z=(r+c)/f}else if(a>u){let f=2*Math.sqrt(1+a-n-u);this._w=(r-c)/f,this._x=(i+o)/f,this._y=.25*f,this._z=(l+h)/f}else{let f=2*Math.sqrt(1+u-n-a);this._w=(o-i)/f,this._x=(r+c)/f,this._y=(l+h)/f,this._z=.25*f}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(ot(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+o*a+i*c-r*l,this._y=i*h+o*l+r*a-n*c,this._z=r*h+o*c+n*l-i*a,this._w=o*h-n*a-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=this.dot(e);a<0&&(n=-n,i=-i,r=-r,o=-o,a=-a);let l=1-t;if(a<.9995){let c=Math.acos(a),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},z=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(Uu.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(Uu.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,o=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*o,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*o,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*o,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,o=e.y,a=e.z,l=e.w,c=2*(o*i-a*n),h=2*(a*t-r*i),u=2*(r*n-o*t);return this.x=t+l*c+o*u-a*h,this.y=n+l*h+a*c-r*u,this.z=i+l*u+r*h-o*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=ot(this.x,e.x,t.x),this.y=ot(this.y,e.y,t.y),this.z=ot(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=ot(this.x,e,t),this.y=ot(this.y,e,t),this.z=ot(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ot(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,o=t.x,a=t.y,l=t.z;return this.x=i*l-r*a,this.y=r*o-n*l,this.z=n*a-i*o,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return bc.copy(this).projectOnVector(e),this.sub(bc)}reflect(e){return this.sub(bc.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(ot(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},bc=new z,Uu=new Zt,tt=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,o,a,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c)}set(e,t,n,i,r,o,a,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=a,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=o,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[3],l=n[6],c=n[1],h=n[4],u=n[7],d=n[2],f=n[5],g=n[8],M=i[0],m=i[3],p=i[6],v=i[1],S=i[4],_=i[7],w=i[2],T=i[5],L=i[8];return r[0]=o*M+a*v+l*w,r[3]=o*m+a*S+l*T,r[6]=o*p+a*_+l*L,r[1]=c*M+h*v+u*w,r[4]=c*m+h*S+u*T,r[7]=c*p+h*_+u*L,r[2]=d*M+f*v+g*w,r[5]=d*m+f*S+g*T,r[8]=d*p+f*_+g*L,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8];return t*o*h-t*a*c-n*r*h+n*a*l+i*r*c-i*o*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=h*o-a*c,d=a*l-h*r,f=c*r-o*l,g=t*u+n*d+i*f;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let M=1/g;return e[0]=u*M,e[1]=(i*c-h*n)*M,e[2]=(a*n-i*o)*M,e[3]=d*M,e[4]=(h*t-i*l)*M,e[5]=(i*r-a*t)*M,e[6]=f*M,e[7]=(n*l-c*t)*M,e[8]=(o*t-n*r)*M,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,o,a){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*o+c*a)+o+e,-i*c,i*l,-i*(-c*o+l*a)+a+t,0,0,1),this}scale(e,t){return Ms("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Sc.makeScale(e,t)),this}rotate(e){return Ms("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Sc.makeRotation(-e)),this}translate(e,t){return Ms("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Sc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Sc=new tt,Fu=new tt().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),Ou=new tt().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function jp(){let s={enabled:!0,workingColorSpace:mn,spaces:{},convert:function(i,r,o){return this.enabled===!1||r===o||!r||!o||(this.spaces[r].transfer===yt&&(i.r=Pi(i.r),i.g=Pi(i.g),i.b=Pi(i.b)),this.spaces[r].primaries!==this.spaces[o].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[o].fromXYZ)),this.spaces[o].transfer===yt&&(i.r=sr(i.r),i.g=sr(i.g),i.b=sr(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===Bi?Qr:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,o){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[o].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return Ms("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return Ms("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[mn]:{primaries:e,whitePoint:n,transfer:Qr,toXYZ:Fu,fromXYZ:Ou,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:Ht},outputColorSpaceConfig:{drawingBufferColorSpace:Ht}},[Ht]:{primaries:e,whitePoint:n,transfer:yt,toXYZ:Fu,fromXYZ:Ou,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:Ht}}}),s}var rt=jp();function Pi(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function sr(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var Ws,Fa=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{Ws===void 0&&(Ws=ar("canvas")),Ws.width=e.width,Ws.height=e.height;let i=Ws.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=Ws}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=ar("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let o=0;o<r.length;o++)r[o]=Pi(r[o]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(Pi(t[n]/255)*255):t[n]=Pi(t[n]);return{data:t,width:e.width,height:e.height}}else return Xe("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},Jp=0,cr=class{constructor(e=null){this.isSource=!0,Object.defineProperty(this,"id",{value:Jp++}),this.uuid=$n(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let o=0,a=i.length;o<a;o++)i[o].isDataTexture?r.push(Tc(i[o].image)):r.push(Tc(i[o]))}else r=Tc(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function Tc(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Fa.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(Xe("Texture: Unable to serialize Texture."),{})}var $p=0,Ec=new z,Qt=class s extends Vn{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=Hn,i=Hn,r=Vt,o=ni,a=Un,l=Sn,c=s.DEFAULT_ANISOTROPY,h=Bi){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:$p++}),this.uuid=$n(),this.name="",this.source=new cr(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=o,this.anisotropy=c,this.format=a,this.internalFormat=null,this.type=l,this.offset=new ve(0,0),this.repeat=new ve(1,1),this.center=new ve(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new tt,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Ec).x}get height(){return this.source.getSize(Ec).y}get depth(){return this.source.getSize(Ec).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){Xe(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Xe(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==dh)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case $i:e.x=e.x-Math.floor(e.x);break;case Hn:e.x=e.x<0?0:1;break;case rr:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case $i:e.y=e.y-Math.floor(e.y);break;case Hn:e.y=e.y<0?0:1;break;case rr:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};Qt.DEFAULT_IMAGE=null;Qt.DEFAULT_MAPPING=dh;Qt.DEFAULT_ANISOTROPY=1;var Mt=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,o=e.elements;return this.x=o[0]*t+o[4]*n+o[8]*i+o[12]*r,this.y=o[1]*t+o[5]*n+o[9]*i+o[13]*r,this.z=o[2]*t+o[6]*n+o[10]*i+o[14]*r,this.w=o[3]*t+o[7]*n+o[11]*i+o[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],d=l[1],f=l[5],g=l[9],M=l[2],m=l[6],p=l[10];if(Math.abs(h-d)<.01&&Math.abs(u-M)<.01&&Math.abs(g-m)<.01){if(Math.abs(h+d)<.1&&Math.abs(u+M)<.1&&Math.abs(g+m)<.1&&Math.abs(c+f+p-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let S=(c+1)/2,_=(f+1)/2,w=(p+1)/2,T=(h+d)/4,L=(u+M)/4,y=(g+m)/4;return S>_&&S>w?S<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(S),i=T/n,r=L/n):_>w?_<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(_),n=T/i,r=y/i):w<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(w),n=L/r,i=y/r),this.set(n,i,r,t),this}let v=Math.sqrt((m-g)*(m-g)+(u-M)*(u-M)+(d-h)*(d-h));return Math.abs(v)<.001&&(v=1),this.x=(m-g)/v,this.y=(u-M)/v,this.z=(d-h)/v,this.w=Math.acos((c+f+p-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=ot(this.x,e.x,t.x),this.y=ot(this.y,e.y,t.y),this.z=ot(this.z,e.z,t.z),this.w=ot(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=ot(this.x,e,t),this.y=ot(this.y,e,t),this.z=ot(this.z,e,t),this.w=ot(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(ot(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},Oa=class extends Vn{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:Vt,depthBuffer:!0,stencilBuffer:!1,resolveDepthBuffer:!0,resolveStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new Mt(0,0,e,t),this.scissorTest=!1,this.viewport=new Mt(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new Qt(i),o=n.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0,this.textures[a].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:Vt,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&(this._depthTexture.renderTarget=null),e!==null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new cr(i)}return this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,e.depthTexture!==null&&(this.depthTexture=e.depthTexture.clone()),this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},Wt=class extends Oa{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},to=class extends Qt{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Kt,this.minFilter=Kt,this.wrapR=Hn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var Ba=class extends Qt{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Kt,this.minFilter=Kt,this.wrapR=Hn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var et=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,o,a,l,c,h,u,d,f,g,M,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,M,m)}set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,M,m){let p=this.elements;return p[0]=e,p[4]=t,p[8]=n,p[12]=i,p[1]=r,p[5]=o,p[9]=a,p[13]=l,p[2]=c,p[6]=h,p[10]=u,p[14]=d,p[3]=f,p[7]=g,p[11]=M,p[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/Xs.setFromMatrixColumn(e,0).length(),r=1/Xs.setFromMatrixColumn(e,1).length(),o=1/Xs.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*o,t[9]=n[9]*o,t[10]=n[10]*o,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,o=Math.cos(n),a=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let d=o*h,f=o*u,g=a*h,M=a*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=f+g*c,t[5]=d-M*c,t[9]=-a*l,t[2]=M-d*c,t[6]=g+f*c,t[10]=o*l}else if(e.order==="YXZ"){let d=l*h,f=l*u,g=c*h,M=c*u;t[0]=d+M*a,t[4]=g*a-f,t[8]=o*c,t[1]=o*u,t[5]=o*h,t[9]=-a,t[2]=f*a-g,t[6]=M+d*a,t[10]=o*l}else if(e.order==="ZXY"){let d=l*h,f=l*u,g=c*h,M=c*u;t[0]=d-M*a,t[4]=-o*u,t[8]=g+f*a,t[1]=f+g*a,t[5]=o*h,t[9]=M-d*a,t[2]=-o*c,t[6]=a,t[10]=o*l}else if(e.order==="ZYX"){let d=o*h,f=o*u,g=a*h,M=a*u;t[0]=l*h,t[4]=g*c-f,t[8]=d*c+M,t[1]=l*u,t[5]=M*c+d,t[9]=f*c-g,t[2]=-c,t[6]=a*l,t[10]=o*l}else if(e.order==="YZX"){let d=o*l,f=o*c,g=a*l,M=a*c;t[0]=l*h,t[4]=M-d*u,t[8]=g*u+f,t[1]=u,t[5]=o*h,t[9]=-a*h,t[2]=-c*h,t[6]=f*u+g,t[10]=d-M*u}else if(e.order==="XZY"){let d=o*l,f=o*c,g=a*l,M=a*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=d*u+M,t[5]=o*h,t[9]=f*u-g,t[2]=g*u-f,t[6]=a*h,t[10]=M*u+d}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(Qp,e,em)}lookAt(e,t,n){let i=this.elements;return Cn.subVectors(e,t),Cn.lengthSq()===0&&(Cn.z=1),Cn.normalize(),Wi.crossVectors(n,Cn),Wi.lengthSq()===0&&(Math.abs(n.z)===1?Cn.x+=1e-4:Cn.z+=1e-4,Cn.normalize(),Wi.crossVectors(n,Cn)),Wi.normalize(),Qo.crossVectors(Cn,Wi),i[0]=Wi.x,i[4]=Qo.x,i[8]=Cn.x,i[1]=Wi.y,i[5]=Qo.y,i[9]=Cn.y,i[2]=Wi.z,i[6]=Qo.z,i[10]=Cn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[4],l=n[8],c=n[12],h=n[1],u=n[5],d=n[9],f=n[13],g=n[2],M=n[6],m=n[10],p=n[14],v=n[3],S=n[7],_=n[11],w=n[15],T=i[0],L=i[4],y=i[8],I=i[12],E=i[1],R=i[5],F=i[9],X=i[13],P=i[2],N=i[6],O=i[10],A=i[14],V=i[3],Y=i[7],G=i[11],ne=i[15];return r[0]=o*T+a*E+l*P+c*V,r[4]=o*L+a*R+l*N+c*Y,r[8]=o*y+a*F+l*O+c*G,r[12]=o*I+a*X+l*A+c*ne,r[1]=h*T+u*E+d*P+f*V,r[5]=h*L+u*R+d*N+f*Y,r[9]=h*y+u*F+d*O+f*G,r[13]=h*I+u*X+d*A+f*ne,r[2]=g*T+M*E+m*P+p*V,r[6]=g*L+M*R+m*N+p*Y,r[10]=g*y+M*F+m*O+p*G,r[14]=g*I+M*X+m*A+p*ne,r[3]=v*T+S*E+_*P+w*V,r[7]=v*L+S*R+_*N+w*Y,r[11]=v*y+S*F+_*O+w*G,r[15]=v*I+S*X+_*A+w*ne,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],o=e[1],a=e[5],l=e[9],c=e[13],h=e[2],u=e[6],d=e[10],f=e[14],g=e[3],M=e[7],m=e[11],p=e[15],v=l*f-c*d,S=a*f-c*u,_=a*d-l*u,w=o*f-c*h,T=o*d-l*h,L=o*u-a*h;return t*(M*v-m*S+p*_)-n*(g*v-m*w+p*T)+i*(g*S-M*w+p*L)-r*(g*_-M*T+m*L)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],o=e[5],a=e[9],l=e[2],c=e[6],h=e[10];return t*(o*h-a*c)-n*(r*h-a*l)+i*(r*c-o*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=e[9],d=e[10],f=e[11],g=e[12],M=e[13],m=e[14],p=e[15],v=t*a-n*o,S=t*l-i*o,_=t*c-r*o,w=n*l-i*a,T=n*c-r*a,L=i*c-r*l,y=h*M-u*g,I=h*m-d*g,E=h*p-f*g,R=u*m-d*M,F=u*p-f*M,X=d*p-f*m,P=v*X-S*F+_*R+w*E-T*I+L*y;if(P===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let N=1/P;return e[0]=(a*X-l*F+c*R)*N,e[1]=(i*F-n*X-r*R)*N,e[2]=(M*L-m*T+p*w)*N,e[3]=(d*T-u*L-f*w)*N,e[4]=(l*E-o*X-c*I)*N,e[5]=(t*X-i*E+r*I)*N,e[6]=(m*_-g*L-p*S)*N,e[7]=(h*L-d*_+f*S)*N,e[8]=(o*F-a*E+c*y)*N,e[9]=(n*E-t*F-r*y)*N,e[10]=(g*T-M*_+p*v)*N,e[11]=(u*_-h*T-f*v)*N,e[12]=(a*I-o*R-l*y)*N,e[13]=(t*R-n*I+i*y)*N,e[14]=(M*S-g*w-m*v)*N,e[15]=(h*w-u*S+d*v)*N,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,o=e.x,a=e.y,l=e.z,c=r*o,h=r*a;return this.set(c*o+n,c*a-i*l,c*l+i*a,0,c*a+i*l,h*a+n,h*l-i*o,0,c*l-i*a,h*l+i*o,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,o){return this.set(1,n,r,0,e,1,o,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,o=t._y,a=t._z,l=t._w,c=r+r,h=o+o,u=a+a,d=r*c,f=r*h,g=r*u,M=o*h,m=o*u,p=a*u,v=l*c,S=l*h,_=l*u,w=n.x,T=n.y,L=n.z;return i[0]=(1-(M+p))*w,i[1]=(f+_)*w,i[2]=(g-S)*w,i[3]=0,i[4]=(f-_)*T,i[5]=(1-(d+p))*T,i[6]=(m+v)*T,i[7]=0,i[8]=(g+S)*L,i[9]=(m-v)*L,i[10]=(1-(d+M))*L,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let o=Xs.set(i[0],i[1],i[2]).length(),a=Xs.set(i[4],i[5],i[6]).length(),l=Xs.set(i[8],i[9],i[10]).length();r<0&&(o=-o),Yn.copy(this);let c=1/o,h=1/a,u=1/l;return Yn.elements[0]*=c,Yn.elements[1]*=c,Yn.elements[2]*=c,Yn.elements[4]*=h,Yn.elements[5]*=h,Yn.elements[6]*=h,Yn.elements[8]*=u,Yn.elements[9]*=u,Yn.elements[10]*=u,t.setFromRotationMatrix(Yn),n.x=o,n.y=a,n.z=l,this}makePerspective(e,t,n,i,r,o,a=Jn,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),d=(t+e)/(t-e),f=(n+i)/(n-i),g,M;if(l)g=r/(o-r),M=o*r/(o-r);else if(a===Jn)g=-(o+r)/(o-r),M=-2*o*r/(o-r);else if(a===or)g=-o/(o-r),M=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=u,c[9]=f,c[13]=0,c[2]=0,c[6]=0,c[10]=g,c[14]=M,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,o,a=Jn,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),d=-(t+e)/(t-e),f=-(n+i)/(n-i),g,M;if(l)g=1/(o-r),M=o/(o-r);else if(a===Jn)g=-2/(o-r),M=-(o+r)/(o-r);else if(a===or)g=-1/(o-r),M=-r/(o-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=0,c[12]=d,c[1]=0,c[5]=u,c[9]=0,c[13]=f,c[2]=0,c[6]=0,c[10]=g,c[14]=M,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},Xs=new z,Yn=new et,Qp=new z(0,0,0),em=new z(1,1,1),Wi=new z,Qo=new z,Cn=new z,Bu=new et,zu=new Zt,Gn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],o=i[4],a=i[8],l=i[1],c=i[5],h=i[9],u=i[2],d=i[6],f=i[10];switch(t){case"XYZ":this._y=Math.asin(ot(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-h,f),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(d,c),this._z=0);break;case"YXZ":this._x=Math.asin(-ot(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(a,f),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(ot(d,-1,1)),Math.abs(d)<.9999999?(this._y=Math.atan2(-u,f),this._z=Math.atan2(-o,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-ot(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(d,f),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-o,c));break;case"YZX":this._z=Math.asin(ot(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(a,f));break;case"XZY":this._z=Math.asin(-ot(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(d,c),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-h,f),this._y=0);break;default:Xe("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return Bu.makeRotationFromQuaternion(e),this.setFromRotationMatrix(Bu,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return zu.setFromEuler(this),this.setFromQuaternion(zu,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};Gn.DEFAULT_ORDER="XYZ";var hr=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},tm=0,ku=new z,qs=new Zt,Ti=new et,ea=new z,kr=new z,nm=new z,im=new Zt,Hu=new z(1,0,0),Vu=new z(0,1,0),Gu=new z(0,0,1),Wu={type:"added"},sm={type:"removed"},Ys={type:"childadded",child:null},wc={type:"childremoved",child:null},St=class s extends Vn{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:tm++}),this.uuid=$n(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new z,t=new Gn,n=new Zt,i=new z(1,1,1);function r(){n.setFromEuler(t,!1)}function o(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new et},normalMatrix:{value:new tt}}),this.matrix=new et,this.matrixWorld=new et,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new hr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return qs.setFromAxisAngle(e,t),this.quaternion.multiply(qs),this}rotateOnWorldAxis(e,t){return qs.setFromAxisAngle(e,t),this.quaternion.premultiply(qs),this}rotateX(e){return this.rotateOnAxis(Hu,e)}rotateY(e){return this.rotateOnAxis(Vu,e)}rotateZ(e){return this.rotateOnAxis(Gu,e)}translateOnAxis(e,t){return ku.copy(e).applyQuaternion(this.quaternion),this.position.add(ku.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(Hu,e)}translateY(e){return this.translateOnAxis(Vu,e)}translateZ(e){return this.translateOnAxis(Gu,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Ti.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?ea.copy(e):ea.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),kr.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Ti.lookAt(kr,ea,this.up):Ti.lookAt(ea,kr,this.up),this.quaternion.setFromRotationMatrix(Ti),i&&(Ti.extractRotation(i.matrixWorld),qs.setFromRotationMatrix(Ti),this.quaternion.premultiply(qs.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?(Qe("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(Wu),Ys.child=e,this.dispatchEvent(Ys),Ys.child=null):Qe("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(sm),wc.child=e,this.dispatchEvent(wc),wc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Ti.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Ti.multiply(e.parent.matrixWorld)),e.applyMatrix4(Ti),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(Wu),Ys.child=e,this.dispatchEvent(Ys),Ys.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let o=this.children[n].getObjectByProperty(e,t);if(o!==void 0)return o}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,o=i.length;r<o;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(kr,e,nm),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(kr,im,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let o=0,a=r.length;o<a;o++)r[o].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,this.name!==""&&(i.name=this.name),this.castShadow===!0&&(i.castShadow=!0),this.receiveShadow===!0&&(i.receiveShadow=!0),this.visible===!1&&(i.visible=!1),this.frustumCulled===!1&&(i.frustumCulled=!1),this.renderOrder!==0&&(i.renderOrder=this.renderOrder),this.static!==!1&&(i.static=this.static),Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.matrixAutoUpdate===!1&&(i.matrixAutoUpdate=!1),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(a=>({...a,boundingBox:a.boundingBox?a.boundingBox.toJSON():void 0,boundingSphere:a.boundingSphere?a.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(a=>({...a})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(a,l){return a[l.uuid]===void 0&&(a[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let l=a.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let l=0,c=this.material.length;l<c;l++)a.push(r(e.materials,this.material[l]));i.material=a}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let a=0;a<this.children.length;a++)i.children.push(this.children[a].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let a=0;a<this.animations.length;a++){let l=this.animations[a];i.animations.push(r(e.animations,l))}}if(t){let a=o(e.geometries),l=o(e.materials),c=o(e.textures),h=o(e.images),u=o(e.shapes),d=o(e.skeletons),f=o(e.animations),g=o(e.nodes);a.length>0&&(n.geometries=a),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),d.length>0&&(n.skeletons=d),f.length>0&&(n.animations=f),g.length>0&&(n.nodes=g)}return n.object=i,n;function o(a){let l=[];for(let c in a){let h=a[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}};St.DEFAULT_UP=new z(0,1,0);St.DEFAULT_MATRIX_AUTO_UPDATE=!0;St.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var lt=class extends St{constructor(){super(),this.isGroup=!0,this.type="Group"}},rm={type:"move"},ur=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new lt,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new lt,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new z,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new z),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new lt,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new z,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new z,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,o=null,a=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){o=!0;for(let M of e.hand.values()){let m=t.getJointPose(M,n),p=this._getHandJoint(c,M);m!==null&&(p.matrix.fromArray(m.transform.matrix),p.matrix.decompose(p.position,p.rotation,p.scale),p.matrixWorldNeedsUpdate=!0,p.jointRadius=m.radius),p.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],d=h.position.distanceTo(u.position),f=.02,g=.005;c.inputState.pinching&&d>f+g?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&d<=f-g&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));a!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(a.matrix.fromArray(i.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,i.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(i.linearVelocity)):a.hasLinearVelocity=!1,i.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(i.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(rm)))}return a!==null&&(a.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=o!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new lt;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},of={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Xi={h:0,s:0,l:0},ta={h:0,s:0,l:0};function Ac(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var xe=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=Ht){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,rt.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=rt.workingColorSpace){return this.r=e,this.g=t,this.b=n,rt.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=rt.workingColorSpace){if(e=Mh(e,1),t=ot(t,0,1),n=ot(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,o=2*n-r;this.r=Ac(o,r,e+1/3),this.g=Ac(o,r,e),this.b=Ac(o,r,e-1/3)}return rt.colorSpaceToWorking(this,i),this}setStyle(e,t=Ht){function n(r){r!==void 0&&parseFloat(r)<1&&Xe("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,o=i[1],a=i[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:Xe("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(o===6)return this.setHex(parseInt(r,16),t);Xe("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=Ht){let n=of[e.toLowerCase()];return n!==void 0?this.setHex(n,t):Xe("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=Pi(e.r),this.g=Pi(e.g),this.b=Pi(e.b),this}copyLinearToSRGB(e){return this.r=sr(e.r),this.g=sr(e.g),this.b=sr(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=Ht){return rt.workingToColorSpace(un.copy(this),e),Math.round(ot(un.r*255,0,255))*65536+Math.round(ot(un.g*255,0,255))*256+Math.round(ot(un.b*255,0,255))}getHexString(e=Ht){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=rt.workingColorSpace){rt.workingToColorSpace(un.copy(this),t);let n=un.r,i=un.g,r=un.b,o=Math.max(n,i,r),a=Math.min(n,i,r),l,c,h=(a+o)/2;if(a===o)l=0,c=0;else{let u=o-a;switch(c=h<=.5?u/(o+a):u/(2-o-a),o){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=rt.workingColorSpace){return rt.workingToColorSpace(un.copy(this),t),e.r=un.r,e.g=un.g,e.b=un.b,e}getStyle(e=Ht){rt.workingToColorSpace(un.copy(this),e);let t=un.r,n=un.g,i=un.b;return e!==Ht?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(Xi),this.setHSL(Xi.h+e,Xi.s+t,Xi.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(Xi),e.getHSL(ta);let n=Kr(Xi.h,ta.h,t),i=Kr(Xi.s,ta.s,t),r=Kr(Xi.l,ta.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},un=new xe;xe.NAMES=of;var no=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new xe(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var As=class extends St{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new Gn,this.environmentIntensity=1,this.environmentRotation=new Gn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),this.backgroundBlurriness>0&&(t.object.backgroundBlurriness=this.backgroundBlurriness),this.backgroundIntensity!==1&&(t.object.backgroundIntensity=this.backgroundIntensity),t.object.backgroundRotation=this.backgroundRotation.toArray(),this.environmentIntensity!==1&&(t.object.environmentIntensity=this.environmentIntensity),t.object.environmentRotation=this.environmentRotation.toArray(),t}},Zn=new z,Ei=new z,Rc=new z,wi=new z,Zs=new z,Ks=new z,Xu=new z,Cc=new z,Pc=new z,Ic=new z,Lc=new Mt,Dc=new Mt,Nc=new Mt,ji=class s{constructor(e=new z,t=new z,n=new z){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),Zn.subVectors(e,t),i.cross(Zn);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){Zn.subVectors(i,t),Ei.subVectors(n,t),Rc.subVectors(e,t);let o=Zn.dot(Zn),a=Zn.dot(Ei),l=Zn.dot(Rc),c=Ei.dot(Ei),h=Ei.dot(Rc),u=o*c-a*a;if(u===0)return r.set(0,0,0),null;let d=1/u,f=(c*l-a*h)*d,g=(o*h-a*l)*d;return r.set(1-f-g,g,f)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,wi)===null?!1:wi.x>=0&&wi.y>=0&&wi.x+wi.y<=1}static getInterpolation(e,t,n,i,r,o,a,l){return this.getBarycoord(e,t,n,i,wi)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,wi.x),l.addScaledVector(o,wi.y),l.addScaledVector(a,wi.z),l)}static getInterpolatedAttribute(e,t,n,i,r,o){return Lc.setScalar(0),Dc.setScalar(0),Nc.setScalar(0),Lc.fromBufferAttribute(e,t),Dc.fromBufferAttribute(e,n),Nc.fromBufferAttribute(e,i),o.setScalar(0),o.addScaledVector(Lc,r.x),o.addScaledVector(Dc,r.y),o.addScaledVector(Nc,r.z),o}static isFrontFacing(e,t,n,i){return Zn.subVectors(n,t),Ei.subVectors(e,t),Zn.cross(Ei).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return Zn.subVectors(this.c,this.b),Ei.subVectors(this.a,this.b),Zn.cross(Ei).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,o,a;Zs.subVectors(i,n),Ks.subVectors(r,n),Cc.subVectors(e,n);let l=Zs.dot(Cc),c=Ks.dot(Cc);if(l<=0&&c<=0)return t.copy(n);Pc.subVectors(e,i);let h=Zs.dot(Pc),u=Ks.dot(Pc);if(h>=0&&u<=h)return t.copy(i);let d=l*u-h*c;if(d<=0&&l>=0&&h<=0)return o=l/(l-h),t.copy(n).addScaledVector(Zs,o);Ic.subVectors(e,r);let f=Zs.dot(Ic),g=Ks.dot(Ic);if(g>=0&&f<=g)return t.copy(r);let M=f*c-l*g;if(M<=0&&c>=0&&g<=0)return a=c/(c-g),t.copy(n).addScaledVector(Ks,a);let m=h*g-f*u;if(m<=0&&u-h>=0&&f-g>=0)return Xu.subVectors(r,i),a=(u-h)/(u-h+(f-g)),t.copy(i).addScaledVector(Xu,a);let p=1/(m+M+d);return o=M*p,a=d*p,t.copy(n).addScaledVector(Zs,o).addScaledVector(Ks,a)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},en=class{constructor(e=new z(1/0,1/0,1/0),t=new z(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(Kn.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(Kn.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=Kn.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)e.isMesh===!0?e.getVertexPosition(o,Kn):Kn.fromBufferAttribute(r,o),Kn.applyMatrix4(e.matrixWorld),this.expandByPoint(Kn);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),na.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),na.copy(n.boundingBox)),na.applyMatrix4(e.matrixWorld),this.union(na)}let i=e.children;for(let r=0,o=i.length;r<o;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,Kn),Kn.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(Hr),ia.subVectors(this.max,Hr),js.subVectors(e.a,Hr),Js.subVectors(e.b,Hr),$s.subVectors(e.c,Hr),qi.subVectors(Js,js),Yi.subVectors($s,Js),ps.subVectors(js,$s);let t=[0,-qi.z,qi.y,0,-Yi.z,Yi.y,0,-ps.z,ps.y,qi.z,0,-qi.x,Yi.z,0,-Yi.x,ps.z,0,-ps.x,-qi.y,qi.x,0,-Yi.y,Yi.x,0,-ps.y,ps.x,0];return!Uc(t,js,Js,$s,ia)||(t=[1,0,0,0,1,0,0,0,1],!Uc(t,js,Js,$s,ia))?!1:(sa.crossVectors(qi,Yi),t=[sa.x,sa.y,sa.z],Uc(t,js,Js,$s,ia))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,Kn).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(Kn).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Ai[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Ai[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Ai[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Ai[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Ai[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Ai[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Ai[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Ai[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Ai),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Ai=[new z,new z,new z,new z,new z,new z,new z,new z],Kn=new z,na=new en,js=new z,Js=new z,$s=new z,qi=new z,Yi=new z,ps=new z,Hr=new z,ia=new z,sa=new z,ms=new z;function Uc(s,e,t,n,i){for(let r=0,o=s.length-3;r<=o;r+=3){ms.fromArray(s,r);let a=i.x*Math.abs(ms.x)+i.y*Math.abs(ms.y)+i.z*Math.abs(ms.z),l=e.dot(ms),c=t.dot(ms),h=n.dot(ms);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>a)return!1}return!0}var Jt=new z,ra=new ve,om=0,xt=class extends Vn{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:om++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Ua,this.updateRanges=[],this.gpuType=Nn,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)ra.fromBufferAttribute(this,t),ra.applyMatrix3(e),this.setXY(t,ra.x,ra.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)Jt.fromBufferAttribute(this,t),Jt.applyMatrix3(e),this.setXYZ(t,Jt.x,Jt.y,Jt.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)Jt.fromBufferAttribute(this,t),Jt.applyMatrix4(e),this.setXYZ(t,Jt.x,Jt.y,Jt.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)Jt.fromBufferAttribute(this,t),Jt.applyNormalMatrix(e),this.setXYZ(t,Jt.x,Jt.y,Jt.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)Jt.fromBufferAttribute(this,t),Jt.transformDirection(e),this.setXYZ(t,Jt.x,Jt.y,Jt.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=jn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Et(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=jn(t,this.array)),t}setX(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=jn(t,this.array)),t}setY(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=jn(t,this.array)),t}setZ(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=jn(t,this.array)),t}setW(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array),r=Et(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return this.name!==""&&(e.name=this.name),this.usage!==Ua&&(e.usage=this.usage),e}dispose(){this.dispatchEvent({type:"dispose"})}};var io=class extends xt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var so=class extends xt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var nt=class extends xt{constructor(e,t,n){super(new Float32Array(e),t,n)}},am=new en,Vr=new z,Fc=new z,dn=class{constructor(e=new z,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):am.setFromPoints(e).getCenter(n);let i=0;for(let r=0,o=e.length;r<o;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;Vr.subVectors(e,this.center);let t=Vr.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(Vr,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(Fc.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(Vr.copy(e.center).add(Fc)),this.expandByPoint(Vr.copy(e.center).sub(Fc))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},lm=0,zn=new et,Oc=new St,Qs=new z,Pn=new en,Gr=new en,an=new z,pt=class s extends Vn{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:lm++}),this.uuid=$n(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Lp(e)?so:io)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new tt().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return zn.makeRotationFromQuaternion(e),this.applyMatrix4(zn),this}rotateX(e){return zn.makeRotationX(e),this.applyMatrix4(zn),this}rotateY(e){return zn.makeRotationY(e),this.applyMatrix4(zn),this}rotateZ(e){return zn.makeRotationZ(e),this.applyMatrix4(zn),this}translate(e,t,n){return zn.makeTranslation(e,t,n),this.applyMatrix4(zn),this}scale(e,t,n){return zn.makeScale(e,t,n),this.applyMatrix4(zn),this}lookAt(e){return Oc.lookAt(e),Oc.updateMatrix(),this.applyMatrix4(Oc.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(Qs).negate(),this.translate(Qs.x,Qs.y,Qs.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let o=e[i];n.push(o.x,o.y,o.z||0)}this.setAttribute("position",new nt(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&Xe("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new en);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){Qe("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new z(-1/0,-1/0,-1/0),new z(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];Pn.setFromBufferAttribute(r),this.morphTargetsRelative?(an.addVectors(this.boundingBox.min,Pn.min),this.boundingBox.expandByPoint(an),an.addVectors(this.boundingBox.max,Pn.max),this.boundingBox.expandByPoint(an)):(this.boundingBox.expandByPoint(Pn.min),this.boundingBox.expandByPoint(Pn.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&Qe('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new dn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){Qe("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new z,1/0);return}if(e){let n=this.boundingSphere.center;if(Pn.setFromBufferAttribute(e),t)for(let r=0,o=t.length;r<o;r++){let a=t[r];Gr.setFromBufferAttribute(a),this.morphTargetsRelative?(an.addVectors(Pn.min,Gr.min),Pn.expandByPoint(an),an.addVectors(Pn.max,Gr.max),Pn.expandByPoint(an)):(Pn.expandByPoint(Gr.min),Pn.expandByPoint(Gr.max))}Pn.getCenter(n);let i=0;for(let r=0,o=e.count;r<o;r++)an.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(an));if(t)for(let r=0,o=t.length;r<o;r++){let a=t[r],l=this.morphTargetsRelative;for(let c=0,h=a.count;c<h;c++)an.fromBufferAttribute(a,c),l&&(Qs.fromBufferAttribute(e,c),an.add(Qs)),i=Math.max(i,n.distanceToSquared(an))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&Qe('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){Qe("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,o=this.getAttribute("tangent");(o===void 0||o.count!==n.count)&&(o=new xt(new Float32Array(4*n.count),4),this.setAttribute("tangent",o));let a=[],l=[];for(let y=0;y<n.count;y++)a[y]=new z,l[y]=new z;let c=new z,h=new z,u=new z,d=new ve,f=new ve,g=new ve,M=new z,m=new z;function p(y,I,E){c.fromBufferAttribute(n,y),h.fromBufferAttribute(n,I),u.fromBufferAttribute(n,E),d.fromBufferAttribute(r,y),f.fromBufferAttribute(r,I),g.fromBufferAttribute(r,E),h.sub(c),u.sub(c),f.sub(d),g.sub(d);let R=1/(f.x*g.y-g.x*f.y);isFinite(R)&&(M.copy(h).multiplyScalar(g.y).addScaledVector(u,-f.y).multiplyScalar(R),m.copy(u).multiplyScalar(f.x).addScaledVector(h,-g.x).multiplyScalar(R),a[y].add(M),a[I].add(M),a[E].add(M),l[y].add(m),l[I].add(m),l[E].add(m))}let v=this.groups;v.length===0&&(v=[{start:0,count:e.count}]);for(let y=0,I=v.length;y<I;++y){let E=v[y],R=E.start,F=E.count;for(let X=R,P=R+F;X<P;X+=3)p(e.getX(X+0),e.getX(X+1),e.getX(X+2))}let S=new z,_=new z,w=new z,T=new z;function L(y){w.fromBufferAttribute(i,y),T.copy(w);let I=a[y];S.copy(I),S.sub(w.multiplyScalar(w.dot(I))).normalize(),_.crossVectors(T,I);let R=_.dot(l[y])<0?-1:1;o.setXYZW(y,S.x,S.y,S.z,R)}for(let y=0,I=v.length;y<I;++y){let E=v[y],R=E.start,F=E.count;for(let X=R,P=R+F;X<P;X+=3)L(e.getX(X+0)),L(e.getX(X+1)),L(e.getX(X+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new xt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let d=0,f=n.count;d<f;d++)n.setXYZ(d,0,0,0);let i=new z,r=new z,o=new z,a=new z,l=new z,c=new z,h=new z,u=new z;if(e)for(let d=0,f=e.count;d<f;d+=3){let g=e.getX(d+0),M=e.getX(d+1),m=e.getX(d+2);i.fromBufferAttribute(t,g),r.fromBufferAttribute(t,M),o.fromBufferAttribute(t,m),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),a.fromBufferAttribute(n,g),l.fromBufferAttribute(n,M),c.fromBufferAttribute(n,m),a.add(h),l.add(h),c.add(h),n.setXYZ(g,a.x,a.y,a.z),n.setXYZ(M,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let d=0,f=t.count;d<f;d+=3)i.fromBufferAttribute(t,d+0),r.fromBufferAttribute(t,d+1),o.fromBufferAttribute(t,d+2),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),n.setXYZ(d+0,h.x,h.y,h.z),n.setXYZ(d+1,h.x,h.y,h.z),n.setXYZ(d+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)an.fromBufferAttribute(e,t),an.normalize(),e.setXYZ(t,an.x,an.y,an.z)}toNonIndexed(){function e(a,l){let c=a.array,h=a.itemSize,u=a.normalized,d=new c.constructor(l.length*h),f=0,g=0;for(let M=0,m=l.length;M<m;M++){a.isInterleavedBufferAttribute?f=l[M]*a.data.stride+a.offset:f=l[M]*h;for(let p=0;p<h;p++)d[g++]=c[f++]}return new xt(d,h,u)}if(this.index===null)return Xe("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let a in i){let l=i[a],c=e(l,n);t.setAttribute(a,c)}let r=this.morphAttributes;for(let a in r){let l=[],c=r[a];for(let h=0,u=c.length;h<u;h++){let d=c[h],f=e(d,n);l.push(f)}t.morphAttributes[a]=l}t.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,l=o.length;a<l;a++){let c=o[a];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,this.name!==""&&(e.name=this.name),Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,d=c.length;u<d;u++){let f=c[u];h.push(f.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(e.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(e.data.boundingSphere=a.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let d=0,f=u.length;d<f;d++)h.push(u[d].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let o=e.groups;for(let c=0,h=o.length;c<h;c++){let u=o[c];this.addGroup(u.start,u.count,u.materialIndex)}let a=e.boundingBox;a!==null&&(this.boundingBox=a.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},dr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Ua,this.updateRanges=[],this.version=0,this.uuid=$n()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=$n()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){return e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=$n()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer))),{uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride}}},pn=new z,fr=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyMatrix4(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.applyNormalMatrix(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)pn.fromBufferAttribute(this,t),pn.transformDirection(e),this.setXYZ(t,pn.x,pn.y,pn.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=jn(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Et(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=jn(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=jn(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=jn(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=jn(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array),r=Et(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){eo("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new xt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){eo("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},cm=0,gn=class extends Vn{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:cm++}),this.uuid=$n(),this.name="",this.type="Material",this.blending=bs,this.side=In,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=wa,this.blendDst=Aa,this.blendEquation=Ji,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new xe(0,0,0),this.blendAlpha=0,this.depthFunc=Ss,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=Qc,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=_s,this.stencilZFail=_s,this.stencilZPass=_s,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){Xe(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){Xe(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,this.name!==""&&(n.name=this.name),this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&this.emissiveIntensity!==1&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&this.attenuationDistance!==1/0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.shadowSide!==null&&(n.shadowSide=this.shadowSide),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),this.blending!==bs&&(n.blending=this.blending),this.side!==In&&(n.side=this.side),this.vertexColors===!0&&(n.vertexColors=!0),this.opacity<1&&(n.opacity=this.opacity),this.transparent===!0&&(n.transparent=!0),this.blendSrc!==wa&&(n.blendSrc=this.blendSrc),this.blendDst!==Aa&&(n.blendDst=this.blendDst),this.blendEquation!==Ji&&(n.blendEquation=this.blendEquation),this.blendSrcAlpha!==null&&(n.blendSrcAlpha=this.blendSrcAlpha),this.blendDstAlpha!==null&&(n.blendDstAlpha=this.blendDstAlpha),this.blendEquationAlpha!==null&&(n.blendEquationAlpha=this.blendEquationAlpha),this.blendColor&&this.blendColor.isColor&&(n.blendColor=this.blendColor.getHex()),this.blendAlpha!==0&&(n.blendAlpha=this.blendAlpha),this.depthFunc!==Ss&&(n.depthFunc=this.depthFunc),this.depthTest===!1&&(n.depthTest=this.depthTest),this.depthWrite===!1&&(n.depthWrite=this.depthWrite),this.colorWrite===!1&&(n.colorWrite=this.colorWrite),this.stencilWriteMask!==255&&(n.stencilWriteMask=this.stencilWriteMask),this.stencilFunc!==Qc&&(n.stencilFunc=this.stencilFunc),this.stencilRef!==0&&(n.stencilRef=this.stencilRef),this.stencilFuncMask!==255&&(n.stencilFuncMask=this.stencilFuncMask),this.stencilFail!==_s&&(n.stencilFail=this.stencilFail),this.stencilZFail!==_s&&(n.stencilZFail=this.stencilZFail),this.stencilZPass!==_s&&(n.stencilZPass=this.stencilZPass),this.stencilWrite===!0&&(n.stencilWrite=this.stencilWrite),this.rotation!==void 0&&this.rotation!==0&&(n.rotation=this.rotation),this.polygonOffset===!0&&(n.polygonOffset=!0),this.polygonOffsetFactor!==0&&(n.polygonOffsetFactor=this.polygonOffsetFactor),this.polygonOffsetUnits!==0&&(n.polygonOffsetUnits=this.polygonOffsetUnits),this.linewidth!==void 0&&this.linewidth!==1&&(n.linewidth=this.linewidth),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.dithering===!0&&(n.dithering=!0),this.alphaTest>0&&(n.alphaTest=this.alphaTest),this.alphaHash===!0&&(n.alphaHash=!0),this.alphaToCoverage===!0&&(n.alphaToCoverage=!0),this.premultipliedAlpha===!0&&(n.premultipliedAlpha=!0),this.forceSinglePass===!0&&(n.forceSinglePass=!0),this.allowOverride===!1&&(n.allowOverride=!1),this.wireframe===!0&&(n.wireframe=!0),this.wireframeLinewidth>1&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!=="round"&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!=="round"&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading===!0&&(n.flatShading=!0),this.visible===!1&&(n.visible=!1),this.toneMapped===!1&&(n.toneMapped=!1),this.fog===!1&&(n.fog=!1),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let o=[];for(let a in r){let l=r[a];delete l.metadata,o.push(l)}return o}if(t){let r=i(e.textures),o=i(e.images);r.length>0&&(n.textures=r),o.length>0&&(n.images=o)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new xe().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new ve().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new ve().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var Ri=new z,Bc=new z,oa=new z,Zi=new z,zc=new z,aa=new z,kc=new z,ui=class{constructor(e=new z,t=new z(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,Ri)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=Ri.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(Ri.copy(this.origin).addScaledVector(this.direction,t),Ri.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){Bc.copy(e).add(t).multiplyScalar(.5),oa.copy(t).sub(e).normalize(),Zi.copy(this.origin).sub(Bc);let r=e.distanceTo(t)*.5,o=-this.direction.dot(oa),a=Zi.dot(this.direction),l=-Zi.dot(oa),c=Zi.lengthSq(),h=Math.abs(1-o*o),u,d,f,g;if(h>0)if(u=o*l-a,d=o*a-l,g=r*h,u>=0)if(d>=-g)if(d<=g){let M=1/h;u*=M,d*=M,f=u*(u+o*d+2*a)+d*(o*u+d+2*l)+c}else d=r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d=-r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d<=-g?(u=Math.max(0,-(-o*r+a)),d=u>0?-r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c):d<=g?(u=0,d=Math.min(Math.max(-r,-l),r),f=d*(d+2*l)+c):(u=Math.max(0,-(o*r+a)),d=u>0?r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c);else d=o>0?-r:r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(Bc).addScaledVector(oa,d),f}intersectSphere(e,t){Ri.subVectors(e.center,this.origin);let n=Ri.dot(this.direction),i=Ri.dot(Ri)-n*n,r=e.radius*e.radius;if(i>r)return null;let o=Math.sqrt(r-i),a=n-o,l=n+o;return l<0?null:a<0?this.at(l,t):this.at(a,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,o,a,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,d=this.origin;return c>=0?(n=(e.min.x-d.x)*c,i=(e.max.x-d.x)*c):(n=(e.max.x-d.x)*c,i=(e.min.x-d.x)*c),h>=0?(r=(e.min.y-d.y)*h,o=(e.max.y-d.y)*h):(r=(e.max.y-d.y)*h,o=(e.min.y-d.y)*h),n>o||r>i||((r>n||isNaN(n))&&(n=r),(o<i||isNaN(i))&&(i=o),u>=0?(a=(e.min.z-d.z)*u,l=(e.max.z-d.z)*u):(a=(e.max.z-d.z)*u,l=(e.min.z-d.z)*u),n>l||a>i)||((a>n||n!==n)&&(n=a),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,Ri)!==null}intersectTriangle(e,t,n,i,r){zc.subVectors(t,e),aa.subVectors(n,e),kc.crossVectors(zc,aa);let o=this.direction.dot(kc),a;if(o>0){if(i)return null;a=1}else if(o<0)a=-1,o=-o;else return null;Zi.subVectors(this.origin,e);let l=a*this.direction.dot(aa.crossVectors(Zi,aa));if(l<0)return null;let c=a*this.direction.dot(zc.cross(Zi));if(c<0||l+c>o)return null;let h=-a*Zi.dot(kc);return h<0?null:this.at(h/o,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},bt=class extends gn{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new xe(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Gn,this.combine=ll,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},qu=new et,gs=new ui,la=new dn,Yu=new z,ca=new z,ha=new z,ua=new z,Hc=new z,da=new z,Zu=new z,fa=new z,je=class extends St{constructor(e=new pt,t=new bt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,o=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let a=this.morphTargetInfluences;if(r&&a){da.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=a[l],u=r[l];h!==0&&(Hc.fromBufferAttribute(u,e),o?da.addScaledVector(Hc,h):da.addScaledVector(Hc.sub(t),h))}t.add(da)}return t}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),la.copy(n.boundingSphere),la.applyMatrix4(r),gs.copy(e.ray).recast(e.near),!(la.containsPoint(gs.origin)===!1&&(gs.intersectSphere(la,Yu)===null||gs.origin.distanceToSquared(Yu)>(e.far-e.near)**2))&&(qu.copy(r).invert(),gs.copy(e.ray).applyMatrix4(qu),!(n.boundingBox!==null&&gs.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,gs)))}_computeIntersections(e,t,n){let i,r=this.geometry,o=this.material,a=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,d=r.groups,f=r.drawRange;if(a!==null)if(Array.isArray(o))for(let g=0,M=d.length;g<M;g++){let m=d[g],p=o[m.materialIndex],v=Math.max(m.start,f.start),S=Math.min(a.count,Math.min(m.start+m.count,f.start+f.count));for(let _=v,w=S;_<w;_+=3){let T=a.getX(_),L=a.getX(_+1),y=a.getX(_+2);i=pa(this,p,e,n,c,h,u,T,L,y),i&&(i.faceIndex=Math.floor(_/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),M=Math.min(a.count,f.start+f.count);for(let m=g,p=M;m<p;m+=3){let v=a.getX(m),S=a.getX(m+1),_=a.getX(m+2);i=pa(this,o,e,n,c,h,u,v,S,_),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(o))for(let g=0,M=d.length;g<M;g++){let m=d[g],p=o[m.materialIndex],v=Math.max(m.start,f.start),S=Math.min(l.count,Math.min(m.start+m.count,f.start+f.count));for(let _=v,w=S;_<w;_+=3){let T=_,L=_+1,y=_+2;i=pa(this,p,e,n,c,h,u,T,L,y),i&&(i.faceIndex=Math.floor(_/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),M=Math.min(l.count,f.start+f.count);for(let m=g,p=M;m<p;m+=3){let v=m,S=m+1,_=m+2;i=pa(this,o,e,n,c,h,u,v,S,_),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function hm(s,e,t,n,i,r,o,a){let l;if(e.side===tn?l=n.intersectTriangle(o,r,i,!0,a):l=n.intersectTriangle(i,r,o,e.side===In,a),l===null)return null;fa.copy(a),fa.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(fa);return c<t.near||c>t.far?null:{distance:c,point:fa.clone(),object:s}}function pa(s,e,t,n,i,r,o,a,l,c){s.getVertexPosition(a,ca),s.getVertexPosition(l,ha),s.getVertexPosition(c,ua);let h=hm(s,e,t,n,ca,ha,ua,Zu);if(h){let u=new z;ji.getBarycoord(Zu,ca,ha,ua,u),i&&(h.uv=ji.getInterpolatedAttribute(i,a,l,c,u,new ve)),r&&(h.uv1=ji.getInterpolatedAttribute(r,a,l,c,u,new ve)),o&&(h.normal=ji.getInterpolatedAttribute(o,a,l,c,u,new z),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let d={a,b:l,c,normal:new z,materialIndex:0};ji.getNormal(ca,ha,ua,d.normal),h.face=d,h.barycoord=u}return h}var Wr=new Mt,Ku=new Mt,ju=new Mt,um=new Mt,Ju=new et,ma=new z,Vc=new dn,$u=new et,Gc=new ui,ro=class extends je{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=Jc,this.bindMatrix=new et,this.bindMatrixInverse=new et,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new en),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ma),this.boundingBox.expandByPoint(ma)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new dn),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,ma),this.boundingSphere.expandByPoint(ma)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Vc.copy(this.boundingSphere),Vc.applyMatrix4(i),e.ray.intersectsSphere(Vc)!==!1&&($u.copy(i).invert(),Gc.copy(e.ray).applyMatrix4($u),!(this.boundingBox!==null&&Gc.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,Gc)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new Mt,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===Jc?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===Gd?this.bindMatrixInverse.copy(this.bindMatrix).invert():Xe("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;Ku.fromBufferAttribute(i.attributes.skinIndex,e),ju.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(Wr.copy(t),t.set(0,0,0,0)):(Wr.set(...t,1),t.set(0,0,0)),Wr.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let o=ju.getComponent(r);if(o!==0){let a=Ku.getComponent(r);Ju.multiplyMatrices(n.bones[a].matrixWorld,n.boneInverses[a]),t.addScaledVector(um.copy(Wr).applyMatrix4(Ju),o)}}return t.isVector4&&(t.w=Wr.w),t.applyMatrix4(this.bindMatrixInverse)}},pr=class extends St{constructor(){super(),this.isBone=!0,this.type="Bone"}},mr=class extends Qt{constructor(e=null,t=1,n=1,i,r,o,a,l,c=Kt,h=Kt,u,d){super(null,o,a,l,c,h,i,r,u,d),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},Qu=new et,dm=new et,oo=class s{constructor(e=[],t=[]){this.uuid=$n(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){Xe("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new et)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new et;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,o=e.length;r<o;r++){let a=e[r]?e[r].matrixWorld:dm;Qu.multiplyMatrices(a,t[r]),Qu.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new mr(t,e,e,Un,Nn);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],o=t[r];o===void 0&&(Xe("Skeleton: No bone found with UUID:",r),o=new pr),this.bones.push(o),this.boneInverses.push(new et().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let o=t[i];e.bones.push(o.uuid);let a=n[i];e.boneInverses.push(a.toArray())}return e}},Qi=class extends xt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},er=new et,ed=new et,ga=[],td=new en,fm=new et,Xr=new je,qr=new dn,vn=class extends je{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new Qi(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,fm)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new en),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,er),td.copy(e.boundingBox).applyMatrix4(er),this.boundingBox.union(td)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new dn),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,er),qr.copy(e.boundingSphere).applyMatrix4(er),this.boundingSphere.union(qr)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,o=e*r+1;for(let a=0;a<n.length;a++)n[a]=i[o+a]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(Xr.geometry=this.geometry,Xr.material=this.material,Xr.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),qr.copy(this.boundingSphere),qr.applyMatrix4(n),e.ray.intersectsSphere(qr)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,er),ed.multiplyMatrices(n,er),Xr.matrixWorld=ed,Xr.raycast(e,ga);for(let o=0,a=ga.length;o<a;o++){let l=ga[o];l.instanceId=r,l.object=this,t.push(l)}ga.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new Qi(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new mr(new Float32Array(i*this.count),i,this.count,ml,Nn));let r=this.morphTexture.source.data.data,o=0;for(let c=0;c<n.length;c++)o+=n[c];let a=this.geometry.morphTargetsRelative?1:1-o,l=i*e;return r[l]=a,r.set(n,l+1),this}updateMorphTargets(){}dispose(){this.dispatchEvent({type:"dispose"}),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Wc=new z,pm=new z,mm=new tt,kn=class{constructor(e=new z(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=Wc.subVectors(n,t).cross(pm.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(Wc),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let o=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(o<0||o>1)?null:t.copy(e.start).addScaledVector(i,o)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||mm.getNormalMatrix(e),i=this.coplanarPoint(Wc).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}},xs=new dn,gm=new ve(.5,.5),xa=new z,gr=class{constructor(e=new kn,t=new kn,n=new kn,i=new kn,r=new kn,o=new kn){this.planes=[e,t,n,i,r,o]}set(e,t,n,i,r,o){let a=this.planes;return a[0].copy(e),a[1].copy(t),a[2].copy(n),a[3].copy(i),a[4].copy(r),a[5].copy(o),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=Jn,n=!1){let i=this.planes,r=e.elements,o=r[0],a=r[1],l=r[2],c=r[3],h=r[4],u=r[5],d=r[6],f=r[7],g=r[8],M=r[9],m=r[10],p=r[11],v=r[12],S=r[13],_=r[14],w=r[15];if(i[0].setComponents(c-o,f-h,p-g,w-v).normalize(),i[1].setComponents(c+o,f+h,p+g,w+v).normalize(),i[2].setComponents(c+a,f+u,p+M,w+S).normalize(),i[3].setComponents(c-a,f-u,p-M,w-S).normalize(),n)i[4].setComponents(l,d,m,_).normalize(),i[5].setComponents(c-l,f-d,p-m,w-_).normalize();else if(i[4].setComponents(c-l,f-d,p-m,w-_).normalize(),t===Jn)i[5].setComponents(c+l,f+d,p+m,w+_).normalize();else if(t===or)i[5].setComponents(l,d,m,_).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),xs.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),xs.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(xs)}intersectsSprite(e){xs.center.set(0,0,0);let t=gm.distanceTo(e.center);return xs.radius=.7071067811865476+t,xs.applyMatrix4(e.matrixWorld),this.intersectsSphere(xs)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(xa.x=i.normal.x>0?e.max.x:e.min.x,xa.y=i.normal.y>0?e.max.y:e.min.y,xa.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(xa)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var di=class extends gn{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new xe(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},za=new z,ka=new z,nd=new et,Yr=new ui,_a=new dn,Xc=new z,id=new z,Ii=class extends St{constructor(e=new pt,t=new di){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)za.fromBufferAttribute(t,i-1),ka.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=za.distanceTo(ka);e.setAttribute("lineDistance",new nt(n,1))}else Xe("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),_a.copy(n.boundingSphere),_a.applyMatrix4(i),_a.radius+=r,e.ray.intersectsSphere(_a)===!1)return;nd.copy(i).invert(),Yr.copy(e.ray).applyMatrix4(nd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=this.isLineSegments?2:1,h=n.index,d=n.attributes.position;if(h!==null){let f=Math.max(0,o.start),g=Math.min(h.count,o.start+o.count);for(let M=f,m=g-1;M<m;M+=c){let p=h.getX(M),v=h.getX(M+1),S=va(this,e,Yr,l,p,v,M);S&&t.push(S)}if(this.isLineLoop){let M=h.getX(g-1),m=h.getX(f),p=va(this,e,Yr,l,M,m,g-1);p&&t.push(p)}}else{let f=Math.max(0,o.start),g=Math.min(d.count,o.start+o.count);for(let M=f,m=g-1;M<m;M+=c){let p=va(this,e,Yr,l,M,M+1,M);p&&t.push(p)}if(this.isLineLoop){let M=va(this,e,Yr,l,g-1,f,g-1);M&&t.push(M)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function va(s,e,t,n,i,r,o){let a=s.geometry.attributes.position;if(za.fromBufferAttribute(a,i),ka.fromBufferAttribute(a,r),t.distanceSqToSegment(za,ka,Xc,id)>n)return;Xc.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(Xc);if(!(c<e.near||c>e.far))return{distance:c,point:id.clone().applyMatrix4(s.matrixWorld),index:o,face:null,faceIndex:null,barycoord:null,object:s}}var sd=new z,rd=new z,Rs=class extends Ii{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)sd.fromBufferAttribute(t,i),rd.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+sd.distanceTo(rd);e.setAttribute("lineDistance",new nt(n,1))}else Xe("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},ao=class extends Ii{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},xr=class extends gn{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new xe(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},od=new et,eh=new ui,ya=new dn,Ma=new z,xn=class extends St{constructor(e=new pt,t=new xr){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),ya.copy(n.boundingSphere),ya.applyMatrix4(i),ya.radius+=r,e.ray.intersectsSphere(ya)===!1)return;od.copy(i).invert(),eh.copy(e.ray).applyMatrix4(od);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=n.index,u=n.attributes.position;if(c!==null){let d=Math.max(0,o.start),f=Math.min(c.count,o.start+o.count);for(let g=d,M=f;g<M;g++){let m=c.getX(g);Ma.fromBufferAttribute(u,m),ad(Ma,m,l,i,e,t,this)}}else{let d=Math.max(0,o.start),f=Math.min(u.count,o.start+o.count);for(let g=d,M=f;g<M;g++)Ma.fromBufferAttribute(u,g),ad(Ma,g,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function ad(s,e,t,n,i,r,o){let a=eh.distanceSqToPoint(s);if(a<t){let l=new z;eh.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(a),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:o})}}var lo=class extends Qt{constructor(e=[],t=rs,n,i,r,o,a,l,c,h){super(e,t,n,i,r,o,a,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},Cs=class extends Qt{constructor(e,t,n,i,r,o,a,l,c){super(e,t,n,i,r,o,a,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var Li=class extends Qt{constructor(e,t,n=ii,i,r,o,a=Kt,l=Kt,c,h=hi,u=1){if(h!==hi&&h!==os)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let d={width:e,height:t,depth:u};super(d,i,r,o,a,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new cr(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return this.compareFunction!==null&&(t.compareFunction=this.compareFunction),t}},Ha=class extends Li{constructor(e,t=ii,n=rs,i,r,o=Kt,a=Kt,l,c=hi){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,o,a,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},co=class extends Qt{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},fi=class s extends pt{constructor(e=1,t=1,n=1,i=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:o};let a=this;i=Math.floor(i),r=Math.floor(r),o=Math.floor(o);let l=[],c=[],h=[],u=[],d=0,f=0;g("z","y","x",-1,-1,n,t,e,o,r,0),g("z","y","x",1,-1,n,t,-e,o,r,1),g("x","z","y",1,1,e,n,t,i,o,2),g("x","z","y",1,-1,e,n,-t,i,o,3),g("x","y","z",1,-1,e,t,n,i,r,4),g("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new nt(c,3)),this.setAttribute("normal",new nt(h,3)),this.setAttribute("uv",new nt(u,2));function g(M,m,p,v,S,_,w,T,L,y,I){let E=_/L,R=w/y,F=_/2,X=w/2,P=T/2,N=L+1,O=y+1,A=0,V=0,Y=new z;for(let G=0;G<O;G++){let ne=G*R-X;for(let le=0;le<N;le++){let Ae=le*E-F;Y[M]=Ae*v,Y[m]=ne*S,Y[p]=P,c.push(Y.x,Y.y,Y.z),Y[M]=0,Y[m]=0,Y[p]=T>0?1:-1,h.push(Y.x,Y.y,Y.z),u.push(le/L),u.push(1-G/y),A+=1}}for(let G=0;G<y;G++)for(let ne=0;ne<L;ne++){let le=d+ne+N*G,Ae=d+ne+N*(G+1),Pe=d+(ne+1)+N*(G+1),Re=d+(ne+1)+N*G;l.push(le,Ae,Re),l.push(Ae,Pe,Re),V+=6}a.addGroup(f,V,I),f+=V,d+=A}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var _r=class s extends pt{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],o=[],a=[],l=[],c=new z,h=new ve;o.push(0,0,0),a.push(0,0,1),l.push(.5,.5);for(let u=0,d=3;u<=t;u++,d+=3){let f=n+u/t*i;c.x=e*Math.cos(f),c.y=e*Math.sin(f),o.push(c.x,c.y,c.z),a.push(0,0,1),h.x=(o[d]/e+1)/2,h.y=(o[d+1]/e+1)/2,l.push(h.x,h.y)}for(let u=1;u<=t;u++)r.push(u,u+1,0);this.setIndex(r),this.setAttribute("position",new nt(o,3)),this.setAttribute("normal",new nt(a,3)),this.setAttribute("uv",new nt(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},es=class s extends pt{constructor(e=1,t=1,n=1,i=32,r=1,o=!1,a=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],d=[],f=[],g=0,M=[],m=n/2,p=0;v(),o===!1&&(e>0&&S(!0),t>0&&S(!1)),this.setIndex(h),this.setAttribute("position",new nt(u,3)),this.setAttribute("normal",new nt(d,3)),this.setAttribute("uv",new nt(f,2));function v(){let _=new z,w=new z,T=0,L=(t-e)/n;for(let y=0;y<=r;y++){let I=[],E=y/r,R=E*(t-e)+e;for(let F=0;F<=i;F++){let X=F/i,P=X*l+a,N=Math.sin(P),O=Math.cos(P);w.x=R*N,w.y=-E*n+m,w.z=R*O,u.push(w.x,w.y,w.z),_.set(N,L,O).normalize(),d.push(_.x,_.y,_.z),f.push(X,1-E),I.push(g++)}M.push(I)}for(let y=0;y<i;y++)for(let I=0;I<r;I++){let E=M[I][y],R=M[I+1][y],F=M[I+1][y+1],X=M[I][y+1];(e>0||I!==0)&&(h.push(E,R,X),T+=3),(t>0||I!==r-1)&&(h.push(R,F,X),T+=3)}c.addGroup(p,T,0),p+=T}function S(_){let w=g,T=new ve,L=new z,y=0,I=_===!0?e:t,E=_===!0?1:-1;for(let F=1;F<=i;F++)u.push(0,m*E,0),d.push(0,E,0),f.push(.5,.5),g++;let R=g;for(let F=0;F<=i;F++){let P=F/i*l+a,N=Math.cos(P),O=Math.sin(P);L.x=I*O,L.y=m*E,L.z=I*N,u.push(L.x,L.y,L.z),d.push(0,E,0),T.x=N*.5+.5,T.y=O*.5*E+.5,f.push(T.x,T.y),g++}for(let F=0;F<i;F++){let X=w+F,P=R+F;_===!0?h.push(P,P+1,X):h.push(P+1,P,X),y+=3}c.addGroup(p,y,_===!0?1:2),p+=y}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Ps=class s extends es{constructor(e=1,t=1,n=32,i=1,r=!1,o=0,a=Math.PI*2){super(0,e,t,n,i,r,o,a),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:o,thetaLength:a}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Va=class s extends pt{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],o=[];a(i),c(n),h(),this.setAttribute("position",new nt(r,3)),this.setAttribute("normal",new nt(r.slice(),3)),this.setAttribute("uv",new nt(o,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function a(v){let S=new z,_=new z,w=new z;for(let T=0;T<t.length;T+=3)f(t[T+0],S),f(t[T+1],_),f(t[T+2],w),l(S,_,w,v)}function l(v,S,_,w){let T=w+1,L=[];for(let y=0;y<=T;y++){L[y]=[];let I=v.clone().lerp(_,y/T),E=S.clone().lerp(_,y/T),R=T-y;for(let F=0;F<=R;F++)F===0&&y===T?L[y][F]=I:L[y][F]=I.clone().lerp(E,F/R)}for(let y=0;y<T;y++)for(let I=0;I<2*(T-y)-1;I++){let E=Math.floor(I/2);I%2===0?(d(L[y][E+1]),d(L[y+1][E]),d(L[y][E])):(d(L[y][E+1]),d(L[y+1][E+1]),d(L[y+1][E]))}}function c(v){let S=new z;for(let _=0;_<r.length;_+=3)S.x=r[_+0],S.y=r[_+1],S.z=r[_+2],S.normalize().multiplyScalar(v),r[_+0]=S.x,r[_+1]=S.y,r[_+2]=S.z}function h(){let v=new z;for(let S=0;S<r.length;S+=3){v.x=r[S+0],v.y=r[S+1],v.z=r[S+2];let _=m(v)/2/Math.PI+.5,w=p(v)/Math.PI+.5;o.push(_,1-w)}g(),u()}function u(){for(let v=0;v<o.length;v+=6){let S=o[v+0],_=o[v+2],w=o[v+4],T=Math.max(S,_,w),L=Math.min(S,_,w);T>.9&&L<.1&&(S<.2&&(o[v+0]+=1),_<.2&&(o[v+2]+=1),w<.2&&(o[v+4]+=1))}}function d(v){r.push(v.x,v.y,v.z)}function f(v,S){let _=v*3;S.x=e[_+0],S.y=e[_+1],S.z=e[_+2]}function g(){let v=new z,S=new z,_=new z,w=new z,T=new ve,L=new ve,y=new ve;for(let I=0,E=0;I<r.length;I+=9,E+=6){v.set(r[I+0],r[I+1],r[I+2]),S.set(r[I+3],r[I+4],r[I+5]),_.set(r[I+6],r[I+7],r[I+8]),T.set(o[E+0],o[E+1]),L.set(o[E+2],o[E+3]),y.set(o[E+4],o[E+5]),w.copy(v).add(S).add(_).divideScalar(3);let R=m(w);M(T,E+0,v,R),M(L,E+2,S,R),M(y,E+4,_,R)}}function M(v,S,_,w){w<0&&v.x===1&&(o[S]=v.x-1),_.x===0&&_.z===0&&(o[S]=w/2/Math.PI+.5)}function m(v){return Math.atan2(v.z,-v.x)}function p(v){return Math.atan2(-v.y,Math.sqrt(v.x*v.x+v.z*v.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var Ln=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){Xe("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let o=1;o<=e;o++)n=this.getPoint(o/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,o;t?o=t:o=e*n[r-1];let a=0,l=r-1,c;for(;a<=l;)if(i=Math.floor(a+(l-a)/2),c=n[i]-o,c<0)a=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===o)return i/(r-1);let h=n[i],d=n[i+1]-h,f=(o-h)/d;return(i+f)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let o=this.getPoint(i),a=this.getPoint(r),l=t||(o.isVector2?new ve:new z);return l.copy(a).sub(o).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new z,i=[],r=[],o=[],a=new z,l=new et;for(let f=0;f<=e;f++){let g=f/e;i[f]=this.getTangentAt(g,new z)}r[0]=new z,o[0]=new z;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),d=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),d<=c&&n.set(0,0,1),a.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],a),o[0].crossVectors(i[0],r[0]);for(let f=1;f<=e;f++){if(r[f]=r[f-1].clone(),o[f]=o[f-1].clone(),a.crossVectors(i[f-1],i[f]),a.length()>Number.EPSILON){a.normalize();let g=Math.acos(ot(i[f-1].dot(i[f]),-1,1));r[f].applyMatrix4(l.makeRotationAxis(a,g))}o[f].crossVectors(i[f],r[f])}if(t===!0){let f=Math.acos(ot(r[0].dot(r[e]),-1,1));f/=e,i[0].dot(a.crossVectors(r[0],r[e]))>0&&(f=-f);for(let g=1;g<=e;g++)r[g].applyMatrix4(l.makeRotationAxis(i[g],f*g)),o[g].crossVectors(i[g],r[g])}return{tangents:i,normals:r,binormals:o}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},ho=class extends Ln{constructor(e=0,t=0,n=1,i=1,r=0,o=Math.PI*2,a=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=o,this.aClockwise=a,this.aRotation=l}getPoint(e,t=new ve){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,o=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(o?r=0:r=i),this.aClockwise===!0&&!o&&(r===i?r=-i:r=r-i);let a=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(a),c=this.aY+this.yRadius*Math.sin(a);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),d=l-this.aX,f=c-this.aY;l=d*h-f*u+this.aX,c=d*u+f*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},Ga=class extends ho{constructor(e,t,n,i,r,o){super(e,t,n,n,i,r,o),this.isArcCurve=!0,this.type="ArcCurve"}};function bh(){let s=0,e=0,t=0,n=0;function i(r,o,a,l){s=r,e=a,t=-3*r+3*o-2*a-l,n=2*r-2*o+a+l}return{initCatmullRom:function(r,o,a,l,c){i(o,a,c*(a-r),c*(l-o))},initNonuniformCatmullRom:function(r,o,a,l,c,h,u){let d=(o-r)/c-(a-r)/(c+h)+(a-o)/h,f=(a-o)/h-(l-o)/(h+u)+(l-a)/u;d*=h,f*=h,i(o,a,d,f)},calc:function(r){let o=r*r,a=o*r;return s+e*r+t*o+n*a}}}var ld=new z,cd=new z,qc=new bh,Yc=new bh,Zc=new bh,vr=class extends Ln{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new z){let n=t,i=this.points,r=i.length,o=(r-(this.closed?0:1))*e,a=Math.floor(o),l=o-a;this.closed?a+=a>0?0:(Math.floor(Math.abs(a)/r)+1)*r:l===0&&a===r-1&&(a=r-2,l=1);let c,h;this.closed||a>0?c=i[(a-1)%r]:(cd.subVectors(i[0],i[1]).add(i[0]),c=cd);let u=i[a%r],d=i[(a+1)%r];if(this.closed||a+2<r?h=i[(a+2)%r]:(ld.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=ld),this.curveType==="centripetal"||this.curveType==="chordal"){let f=this.curveType==="chordal"?.5:.25,g=Math.pow(c.distanceToSquared(u),f),M=Math.pow(u.distanceToSquared(d),f),m=Math.pow(d.distanceToSquared(h),f);M<1e-4&&(M=1),g<1e-4&&(g=M),m<1e-4&&(m=M),qc.initNonuniformCatmullRom(c.x,u.x,d.x,h.x,g,M,m),Yc.initNonuniformCatmullRom(c.y,u.y,d.y,h.y,g,M,m),Zc.initNonuniformCatmullRom(c.z,u.z,d.z,h.z,g,M,m)}else this.curveType==="catmullrom"&&(qc.initCatmullRom(c.x,u.x,d.x,h.x,this.tension),Yc.initCatmullRom(c.y,u.y,d.y,h.y,this.tension),Zc.initCatmullRom(c.z,u.z,d.z,h.z,this.tension));return n.set(qc.calc(l),Yc.calc(l),Zc.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new z().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function hd(s,e,t,n,i){let r=(n-e)*.5,o=(i-t)*.5,a=s*s,l=s*a;return(2*t-2*n+r+o)*l+(-3*t+3*n-2*r-o)*a+r*s+t}function xm(s,e){let t=1-s;return t*t*e}function _m(s,e){return 2*(1-s)*s*e}function vm(s,e){return s*s*e}function jr(s,e,t,n){return xm(s,e)+_m(s,t)+vm(s,n)}function ym(s,e){let t=1-s;return t*t*t*e}function Mm(s,e){let t=1-s;return 3*t*t*s*e}function bm(s,e){return 3*(1-s)*s*s*e}function Sm(s,e){return s*s*s*e}function Jr(s,e,t,n,i){return ym(s,e)+Mm(s,t)+bm(s,n)+Sm(s,i)}var Wa=class extends Ln{constructor(e=new ve,t=new ve,n=new ve,i=new ve){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new ve){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(Jr(e,i.x,r.x,o.x,a.x),Jr(e,i.y,r.y,o.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Xa=class extends Ln{constructor(e=new z,t=new z,n=new z,i=new z){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new z){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set(Jr(e,i.x,r.x,o.x,a.x),Jr(e,i.y,r.y,o.y,a.y),Jr(e,i.z,r.z,o.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},qa=class extends Ln{constructor(e=new ve,t=new ve){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new ve){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new ve){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},yr=class extends Ln{constructor(e=new z,t=new z){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new z){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new z){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Ya=class extends Ln{constructor(e=new ve,t=new ve,n=new ve){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new ve){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(jr(e,i.x,r.x,o.x),jr(e,i.y,r.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Qn=class extends Ln{constructor(e=new z,t=new z,n=new z){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new z){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(jr(e,i.x,r.x,o.x),jr(e,i.y,r.y,o.y),jr(e,i.z,r.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Za=class extends Ln{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new ve){let n=t,i=this.points,r=(i.length-1)*e,o=Math.floor(r),a=r-o,l=i[o===0?o:o-1],c=i[o],h=i[o>i.length-2?i.length-1:o+1],u=i[o>i.length-3?i.length-1:o+2];return n.set(hd(a,l.x,c.x,h.x,u.x),hd(a,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new ve().fromArray(i))}return this}},th=Object.freeze({__proto__:null,ArcCurve:Ga,CatmullRomCurve3:vr,CubicBezierCurve:Wa,CubicBezierCurve3:Xa,EllipseCurve:ho,LineCurve:qa,LineCurve3:yr,QuadraticBezierCurve:Ya,QuadraticBezierCurve3:Qn,SplineCurve:Za}),uo=class extends Ln{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new th[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let o=i[r]-n,a=this.curves[r],l=a.getLength(),c=l===0?0:1-o/l;return a.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let o=r[i],a=o.isEllipseCurve?e*2:o.isLineCurve||o.isLineCurve3?1:o.isSplineCurve?e*o.points.length:e,l=o.getPoints(a);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new th[i.type]().fromJSON(i))}return this}};var Mr=class s extends Va{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var ln=class s extends pt{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,o=t/2,a=Math.floor(n),l=Math.floor(i),c=a+1,h=l+1,u=e/a,d=t/l,f=[],g=[],M=[],m=[];for(let p=0;p<h;p++){let v=p*d-o;for(let S=0;S<c;S++){let _=S*u-r;g.push(_,-v,0),M.push(0,0,1),m.push(S/a),m.push(1-p/l)}}for(let p=0;p<l;p++)for(let v=0;v<a;v++){let S=v+c*p,_=v+c*(p+1),w=v+1+c*(p+1),T=v+1+c*p;f.push(S,_,T),f.push(_,w,T)}this.setIndex(f),this.setAttribute("position",new nt(g,3)),this.setAttribute("normal",new nt(M,3)),this.setAttribute("uv",new nt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},Is=class s extends pt{constructor(e=.5,t=1,n=32,i=1,r=0,o=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:o},n=Math.max(3,n),i=Math.max(1,i);let a=[],l=[],c=[],h=[],u=e,d=(t-e)/i,f=new z,g=new ve;for(let M=0;M<=i;M++){for(let m=0;m<=n;m++){let p=r+m/n*o;f.x=u*Math.cos(p),f.y=u*Math.sin(p),l.push(f.x,f.y,f.z),c.push(0,0,1),g.x=(f.x/t+1)/2,g.y=(f.y/t+1)/2,h.push(g.x,g.y)}u+=d}for(let M=0;M<i;M++){let m=M*(n+1);for(let p=0;p<n;p++){let v=p+m,S=v,_=v+n+1,w=v+n+2,T=v+1;a.push(S,_,T),a.push(_,w,T)}}this.setIndex(a),this.setAttribute("position",new nt(l,3)),this.setAttribute("normal",new nt(c,3)),this.setAttribute("uv",new nt(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var pi=class s extends pt{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,o=0,a=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:o,thetaLength:a},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(o+a,Math.PI),c=0,h=[],u=new z,d=new z,f=[],g=[],M=[],m=[];for(let p=0;p<=n;p++){let v=[],S=p/n,_=o+S*a,w=e*Math.cos(_),T=Math.sqrt(e*e-w*w),L=0;p===0&&o===0?L=.5/t:p===n&&l===Math.PI&&(L=-.5/t);for(let y=0;y<=t;y++){let I=y/t,E=i+I*r;u.x=-T*Math.cos(E),u.y=w,u.z=T*Math.sin(E),g.push(u.x,u.y,u.z),d.copy(u).normalize(),M.push(d.x,d.y,d.z),m.push(I+L,1-S),v.push(c++)}h.push(v)}for(let p=0;p<n;p++)for(let v=0;v<t;v++){let S=h[p][v+1],_=h[p][v],w=h[p+1][v],T=h[p+1][v+1];(p!==0||o>0)&&f.push(S,_,T),(p!==n-1||l<Math.PI)&&f.push(_,w,T)}this.setIndex(f),this.setAttribute("position",new nt(g,3)),this.setAttribute("normal",new nt(M,3)),this.setAttribute("uv",new nt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Dn=class s extends pt{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,o=0,a=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:o,thetaLength:a},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],d=new z,f=new z,g=new z;for(let M=0;M<=n;M++){let m=o+M/n*a;for(let p=0;p<=i;p++){let v=p/i*r;f.x=(e+t*Math.cos(m))*Math.cos(v),f.y=(e+t*Math.cos(m))*Math.sin(v),f.z=t*Math.sin(m),c.push(f.x,f.y,f.z),d.x=e*Math.cos(v),d.y=e*Math.sin(v),g.subVectors(f,d).normalize(),h.push(g.x,g.y,g.z),u.push(p/i),u.push(M/n)}}for(let M=1;M<=n;M++)for(let m=1;m<=i;m++){let p=(i+1)*M+m-1,v=(i+1)*(M-1)+m-1,S=(i+1)*(M-1)+m,_=(i+1)*M+m;l.push(p,v,_),l.push(v,S,_)}this.setIndex(l),this.setAttribute("position",new nt(c,3)),this.setAttribute("normal",new nt(h,3)),this.setAttribute("uv",new nt(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc)}};var fo=class s extends pt{constructor(e=new Qn(new z(-1,-1,0),new z(-1,1,0),new z(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let o=e.computeFrenetFrames(t,r);this.tangents=o.tangents,this.normals=o.normals,this.binormals=o.binormals;let a=new z,l=new z,c=new ve,h=new z,u=[],d=[],f=[],g=[];M(),this.setIndex(g),this.setAttribute("position",new nt(u,3)),this.setAttribute("normal",new nt(d,3)),this.setAttribute("uv",new nt(f,2));function M(){for(let S=0;S<t;S++)m(S);m(r===!1?t:0),v(),p()}function m(S){h=e.getPointAt(S/t,h);let _=o.normals[S],w=o.binormals[S];for(let T=0;T<=i;T++){let L=T/i*Math.PI*2,y=Math.sin(L),I=-Math.cos(L);l.x=I*_.x+y*w.x,l.y=I*_.y+y*w.y,l.z=I*_.z+y*w.z,l.normalize(),d.push(l.x,l.y,l.z),a.x=h.x+n*l.x,a.y=h.y+n*l.y,a.z=h.z+n*l.z,u.push(a.x,a.y,a.z)}}function p(){for(let S=1;S<=t;S++)for(let _=1;_<=i;_++){let w=(i+1)*(S-1)+(_-1),T=(i+1)*S+(_-1),L=(i+1)*S+_,y=(i+1)*(S-1)+_;g.push(w,T,y),g.push(T,L,y)}}function v(){for(let S=0;S<=t;S++)for(let _=0;_<=i;_++)c.x=S/t,c.y=_/i,f.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new th[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},po=class extends pt{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new z,r=new z;if(e.index!==null){let o=e.attributes.position,a=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:a.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],d=u.start,f=u.count;for(let g=d,M=d+f;g<M;g+=3)for(let m=0;m<3;m++){let p=a.getX(g+m),v=a.getX(g+(m+1)%3);i.fromBufferAttribute(o,p),r.fromBufferAttribute(o,v),ud(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let o=e.attributes.position;for(let a=0,l=o.count/3;a<l;a++)for(let c=0;c<3;c++){let h=3*a+c,u=3*a+(c+1)%3;i.fromBufferAttribute(o,h),r.fromBufferAttribute(o,u),ud(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new nt(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function ud(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function Os(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(dd(i))i.isRenderTargetTexture?(Xe("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(dd(i[0])){let r=[];for(let o=0,a=i.length;o<a;o++)r[o]=i[o].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function fn(s){let e={};for(let t=0;t<s.length;t++){let n=Os(s[t]);for(let i in n)e[i]=n[i]}return e}function dd(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function Tm(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Sh(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:rt.workingColorSpace}var si={clone:Os,merge:fn},Em=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,wm=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,mt=class extends gn{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Em,this.fragmentShader=wm,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=Os(e.uniforms),this.uniformsGroups=Tm(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let o=this.uniforms[i].value;o&&o.isTexture?t.uniforms[i]={type:"t",value:o.toJSON(e).uuid}:o&&o.isColor?t.uniforms[i]={type:"c",value:o.getHex()}:o&&o.isVector2?t.uniforms[i]={type:"v2",value:o.toArray()}:o&&o.isVector3?t.uniforms[i]={type:"v3",value:o.toArray()}:o&&o.isVector4?t.uniforms[i]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?t.uniforms[i]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?t.uniforms[i]={type:"m4",value:o.toArray()}:t.uniforms[i]={value:o}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new xe().setHex(i.value);break;case"v2":this.uniforms[n].value=new ve().fromArray(i.value);break;case"v3":this.uniforms[n].value=new z().fromArray(i.value);break;case"v4":this.uniforms[n].value=new Mt().fromArray(i.value);break;case"m3":this.uniforms[n].value=new tt().fromArray(i.value);break;case"m4":this.uniforms[n].value=new et().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},br=class extends mt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},yn=class extends gn{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new xe(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new xe(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Go,this.normalScale=new ve(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Gn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Mn=class extends yn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new ve(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return ot(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new xe(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new xe(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new xe(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var mo=class extends gn{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new xe(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new xe(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Go,this.normalScale=new ve(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Gn,this.combine=ll,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Ka=class extends gn{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=Yd,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},ja=class extends gn{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function ba(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function Am(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function fd(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,o=0;o!==n;++r){let a=t[r]*e;for(let l=0;l!==e;++l)i[o++]=s[a+l]}return i}function Rm(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let o=r[n];if(o!==void 0)if(Array.isArray(o))do o=r[n],o!==void 0&&(e.push(r.time),t.push(...o)),r=s[i++];while(r!==void 0);else if(o.toArray!==void 0)do o=r[n],o!==void 0&&(e.push(r.time),o.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do o=r[n],o!==void 0&&(e.push(r.time),t.push(o)),r=s[i++];while(r!==void 0)}var mi=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let o;n:{i:if(!(e<i)){for(let a=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===a)break;if(r=i,i=t[++n],e<i)break t}o=t.length;break n}if(!(e>=r)){let a=t[1];e<a&&(n=2,r=a);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}o=n,n=0;break n}break e}for(;n<o;){let a=n+o>>>1;e<t[a]?o=a:n=a+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let o=0;o!==i;++o)t[o]=n[r+o];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},Ja=class extends mi{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:vs,endingEnd:vs}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,o=e+1,a=i[r],l=i[o];if(a===void 0)switch(this.getSettings_().endingStart){case ys:r=e,a=2*t-n;break;case $r:r=i.length-2,a=t+i[r]-i[r+1];break;default:r=e,a=n}if(l===void 0)switch(this.getSettings_().endingEnd){case ys:o=e,l=2*n-t;break;case $r:o=1,l=n+i[1]-i[0];break;default:o=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-a),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=o*h}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this._offsetPrev,u=this._offsetNext,d=this._weightPrev,f=this._weightNext,g=(n-t)/(i-t),M=g*g,m=M*g,p=-d*m+2*d*M-d*g,v=(1+d)*m+(-1.5-2*d)*M+(-.5+d)*g+1,S=(-1-f)*m+(1.5+f)*M+.5*g,_=f*m-f*M;for(let w=0;w!==a;++w)r[w]=p*o[h+w]+v*o[c+w]+S*o[l+w]+_*o[u+w];return r}},go=class extends mi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=(n-t)/(i-t),u=1-h;for(let d=0;d!==a;++d)r[d]=o[c+d]*u+o[l+d]*h;return r}},$a=class extends mi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},Qa=class extends mi{interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this.inTangents,u=this.outTangents;if(!h||!u){let g=(n-t)/(i-t),M=1-g;for(let m=0;m!==a;++m)r[m]=o[c+m]*M+o[l+m]*g;return r}let d=a*2,f=e-1;for(let g=0;g!==a;++g){let M=o[c+g],m=o[l+g],p=f*d+g*2,v=u[p],S=u[p+1],_=e*d+g*2,w=h[_],T=h[_+1],L=(n-t)/(i-t),y,I,E,R,F;for(let X=0;X<8;X++){y=L*L,I=y*L,E=1-L,R=E*E,F=R*E;let N=F*t+3*R*L*v+3*E*y*w+I*i-n;if(Math.abs(N)<1e-10)break;let O=3*R*(v-t)+6*E*L*(w-v)+3*y*(i-w);if(Math.abs(O)<1e-10)break;L=L-N/O,L=Math.max(0,Math.min(1,L))}r[g]=F*M+3*R*L*S+3*E*y*T+I*m}return r}},bn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=ba(t,this.TimeBufferType),this.values=ba(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:ba(e.times,Array),values:ba(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i)}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new $a(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new go(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new Ja(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new Qa(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case Ts:t=this.InterpolantFactoryMethodDiscrete;break;case Es:t=this.InterpolantFactoryMethodLinear;break;case Ea:t=this.InterpolantFactoryMethodSmooth;break;case $c:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return Xe("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return Ts;case this.InterpolantFactoryMethodLinear:return Es;case this.InterpolantFactoryMethodSmooth:return Ea;case this.InterpolantFactoryMethodBezier:return $c}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e}return this}trim(e,t){let n=this.times,i=n.length,r=0,o=i-1;for(;r!==i&&n[r]<e;)++r;for(;o!==-1&&n[o]>t;)--o;if(++o,r!==0||o!==i){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=n.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&(Qe("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&(Qe("KeyframeTrack: Track is empty.",this),e=!1);let o=null;for(let a=0;a!==r;a++){let l=n[a];if(typeof l=="number"&&isNaN(l)){Qe("KeyframeTrack: Time is not a valid number.",this,a,l),e=!1;break}if(o!==null&&o>l){Qe("KeyframeTrack: Out of order keys.",this,a,l,o),e=!1;break}o=l}if(i!==void 0&&Dp(i))for(let a=0,l=i.length;a!==l;++a){let c=i[a];if(isNaN(c)){Qe("KeyframeTrack: Value is not a valid number.",this,a,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===Ea,r=e.length-1,o=1;for(let a=1;a<r;++a){let l=!1,c=e[a],h=e[a+1];if(c!==h&&(a!==1||c!==e[0]))if(i)l=!0;else{let u=a*n,d=u-n,f=u+n;for(let g=0;g!==n;++g){let M=t[u+g];if(M!==t[d+g]||M!==t[f+g]){l=!0;break}}}if(l){if(a!==o){e[o]=e[a];let u=a*n,d=o*n;for(let f=0;f!==n;++f)t[d+f]=t[u+f]}++o}}if(r>0){e[o]=e[r];for(let a=r*n,l=o*n,c=0;c!==n;++c)t[l+c]=t[a+c];++o}return o!==e.length?(this.times=e.slice(0,o),this.values=t.slice(0,o*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,i}};bn.prototype.ValueTypeName="";bn.prototype.TimeBufferType=Float32Array;bn.prototype.ValueBufferType=Float32Array;bn.prototype.DefaultInterpolation=Es;var Di=class extends bn{constructor(e,t,n){super(e,t,n)}};Di.prototype.ValueTypeName="bool";Di.prototype.ValueBufferType=Array;Di.prototype.DefaultInterpolation=Ts;Di.prototype.InterpolantFactoryMethodLinear=void 0;Di.prototype.InterpolantFactoryMethodSmooth=void 0;var xo=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}};xo.prototype.ValueTypeName="color";var Ni=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}};Ni.prototype.ValueTypeName="number";var el=class extends mi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=(n-t)/(i-t),c=e*a;for(let h=c+a;c!==h;c+=4)Zt.slerpFlat(r,0,o,c-a,o,c,l);return r}},Ui=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new el(this.times,this.values,this.getValueSize(),e)}};Ui.prototype.ValueTypeName="quaternion";Ui.prototype.InterpolantFactoryMethodSmooth=void 0;var Fi=class extends bn{constructor(e,t,n){super(e,t,n)}};Fi.prototype.ValueTypeName="string";Fi.prototype.ValueBufferType=Array;Fi.prototype.DefaultInterpolation=Ts;Fi.prototype.InterpolantFactoryMethodLinear=void 0;Fi.prototype.InterpolantFactoryMethodSmooth=void 0;var ts=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}};ts.prototype.ValueTypeName="vector";var Ls=class{constructor(e="",t=-1,n=[],i=jl){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=$n(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let o=0,a=n.length;o!==a;++o)t.push(Pm(n[o]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,o=n.length;r!==o;++r)t.push(bn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,o=[];for(let a=0;a<r;a++){let l=[],c=[];l.push((a+r-1)%r,a,(a+1)%r),c.push(0,1,0);let h=Am(l);l=fd(l,1,h),c=fd(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),o.push(new Ni(".morphTargetInfluences["+t[a].name+"]",l,c).scale(1/n))}return new this(e,-1,o)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let a=0,l=e.length;a<l;a++){let c=e[a],h=c.name.match(r);if(h&&h.length>1){let u=h[1],d=i[u];d||(i[u]=d=[]),d.push(c)}}let o=[];for(let a in i)o.push(this.CreateFromMorphTargetSequence(a,i[a],t,n));return o}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function Cm(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Ni;case"vector":case"vector2":case"vector3":case"vector4":return ts;case"color":return xo;case"quaternion":return Ui;case"bool":case"boolean":return Di;case"string":return Fi}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function Pm(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=Cm(s.type);if(s.times===void 0){let t=[],n=[];Rm(s.keys,t,n,"value"),s.times=t,s.values=n}return e.parse!==void 0?e.parse(s):new e(s.name,s.times,s.values,s.interpolation)}var ci={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(pd(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!pd(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function pd(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var tl=class{constructor(e,t,n){let i=this,r=!1,o=0,a=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){a++,r===!1&&i.onStart!==void 0&&i.onStart(h,o,a),r=!0},this.itemEnd=function(h){o++,i.onProgress!==void 0&&i.onProgress(h,o,a),o===a&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,d=c.length;u<d;u+=2){let f=c[u],g=c[u+1];if(f.global&&(f.lastIndex=0),f.test(h))return g}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},af=new tl,gi=class{constructor(e){this.manager=e!==void 0?e:af,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};gi.DEFAULT_MATERIAL_NAME="__DEFAULT";var Ci={},nh=class extends Error{constructor(e,t){super(e),this.response=t}},Sr=class extends gi{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=ci.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Ci[e]!==void 0){Ci[e].push({onLoad:t,onProgress:n,onError:i});return}Ci[e]=[],Ci[e].push({onLoad:t,onProgress:n,onError:i});let o=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),a=this.mimeType,l=this.responseType;fetch(o).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&Xe("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Ci[e],u=c.body.getReader(),d=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),f=d?parseInt(d):0,g=f!==0,M=0,m=new ReadableStream({start(p){v();function v(){u.read().then(({done:S,value:_})=>{if(S)p.close();else{M+=_.byteLength;let w=new ProgressEvent("progress",{lengthComputable:g,loaded:M,total:f});for(let T=0,L=h.length;T<L;T++){let y=h[T];y.onProgress&&y.onProgress(w)}p.enqueue(_),v()}},S=>{p.error(S)})}}});return new Response(m)}else throw new nh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,a));case"json":return c.json();default:if(a==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(a),d=u&&u[1]?u[1].toLowerCase():void 0,f=new TextDecoder(d);return c.arrayBuffer().then(g=>f.decode(g))}}}).then(c=>{ci.add(`file:${e}`,c);let h=Ci[e];delete Ci[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onLoad&&f.onLoad(c)}}).catch(c=>{let h=Ci[e];if(h===void 0)throw this.manager.itemError(e),c;delete Ci[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onError&&f.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var tr=new WeakMap,nl=class extends gi{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=ci.get(`image:${e}`);if(o!==void 0){if(o.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);else{let u=tr.get(o);u===void 0&&(u=[],tr.set(o,u)),u.push({onLoad:t,onError:i})}return o}let a=ar("img");function l(){h(),t&&t(this);let u=tr.get(this)||[];for(let d=0;d<u.length;d++){let f=u[d];f.onLoad&&f.onLoad(this)}tr.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),ci.remove(`image:${e}`);let d=tr.get(this)||[];for(let f=0;f<d.length;f++){let g=d[f];g.onError&&g.onError(u)}tr.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){a.removeEventListener("load",l,!1),a.removeEventListener("error",c,!1)}return a.addEventListener("load",l,!1),a.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(a.crossOrigin=this.crossOrigin),ci.add(`image:${e}`,a),r.manager.itemStart(e),a.src=e,a}};var _o=class extends gi{constructor(e){super(e)}load(e,t,n,i){let r=new Qt,o=new nl(this.manager);return o.setCrossOrigin(this.crossOrigin),o.setPath(this.path),o.load(e,function(a){r.image=a,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Ds=class extends St{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new xe(e),this.intensity=t}dispose(){this.dispatchEvent({type:"dispose"})}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},vo=class extends Ds{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(St.DEFAULT_UP),this.updateMatrix(),this.groundColor=new xe(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},Kc=new et,md=new z,gd=new z,yo=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new ve(512,512),this.mapType=Sn,this.map=null,this.mapPass=null,this.matrix=new et,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new gr,this._frameExtents=new ve(1,1),this._viewportCount=1,this._viewports=[new Mt(0,0,1,1)]}getViewportCount(){return this._viewportCount}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera,n=this.matrix;md.setFromMatrixPosition(e.matrixWorld),t.position.copy(md),gd.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(gd),t.updateMatrixWorld(),Kc.multiplyMatrices(t.projectionMatrix,t.matrixWorldInverse),this._frustum.setFromProjectionMatrix(Kc,t.coordinateSystem,t.reversedDepth),t.coordinateSystem===or||t.reversedDepth?n.set(.5,0,0,.5,0,.5,0,.5,0,0,1,0,0,0,0,1):n.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),n.multiply(Kc)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return this.intensity!==1&&(e.intensity=this.intensity),this.bias!==0&&(e.bias=this.bias),this.normalBias!==0&&(e.normalBias=this.normalBias),this.radius!==1&&(e.radius=this.radius),(this.mapSize.x!==512||this.mapSize.y!==512)&&(e.mapSize=this.mapSize.toArray()),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Sa=new z,Ta=new Zt,li=new z,Mo=class extends St{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new et,this.projectionMatrix=new et,this.projectionMatrixInverse=new et,this.coordinateSystem=Jn,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Sa,Ta,li),li.x===1&&li.y===1&&li.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Sa,Ta,li.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Sa,Ta,li),li.x===1&&li.y===1&&li.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Sa,Ta,li.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},Ki=new z,xd=new ve,_d=new ve,$t=class extends Mo{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=ws*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(Zr*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return ws*2*Math.atan(Math.tan(Zr*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){Ki.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(Ki.x,Ki.y).multiplyScalar(-e/Ki.z),Ki.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(Ki.x,Ki.y).multiplyScalar(-e/Ki.z)}getViewSize(e,t){return this.getViewBounds(e,xd,_d),t.subVectors(_d,xd)}setViewOffset(e,t,n,i,r,o){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(Zr*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,o=this.view;if(this.view!==null&&this.view.enabled){let l=o.fullWidth,c=o.fullHeight;r+=o.offsetX*i/l,t-=o.offsetY*n/c,i*=o.width/l,n*=o.height/c}let a=this.filmOffset;a!==0&&(r+=e*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},ih=class extends yo{constructor(){super(new $t(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=ws*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this}},bo=class extends Ds{constructor(e,t,n=0,i=Math.PI/3,r=0,o=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(St.DEFAULT_UP),this.updateMatrix(),this.target=new St,this.distance=n,this.angle=i,this.penumbra=r,this.decay=o,this.map=null,this.shadow=new ih}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},sh=class extends yo{constructor(){super(new $t(90,1,.5,500)),this.isPointLightShadow=!0}},ei=class extends Ds{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new sh}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},xi=class extends Mo{constructor(e=-1,t=1,n=1,i=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,o=n+e,a=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,o=r+c*this.view.width,a-=h*this.view.offsetY,l=a-h*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},rh=class extends yo{constructor(){super(new xi(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},ns=class extends Ds{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(St.DEFAULT_UP),this.updateMatrix(),this.target=new St,this.shadow=new rh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var Oi=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var jc=new WeakMap,So=class extends gi{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&Xe("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&Xe("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=ci.get(`image-bitmap:${e}`);if(o!==void 0){if(r.manager.itemStart(e),o.then){o.then(c=>{jc.has(o)===!0?(i&&i(jc.get(o)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);return}let a={};a.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",a.headers=this.requestHeader,a.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,a).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign(r.options,{colorSpaceConversion:"none"}))}).then(function(c){ci.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e)}).catch(function(c){i&&i(c),jc.set(l,c),ci.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});ci.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var nr=-90,ir=1,il=class extends St{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new $t(nr,ir,e,t);i.layers=this.layers,this.add(i);let r=new $t(nr,ir,e,t);r.layers=this.layers,this.add(r);let o=new $t(nr,ir,e,t);o.layers=this.layers,this.add(o);let a=new $t(nr,ir,e,t);a.layers=this.layers,this.add(a);let l=new $t(nr,ir,e,t);l.layers=this.layers,this.add(l);let c=new $t(nr,ir,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,o,a,l]=t;for(let c of t)this.remove(c);if(e===Jn)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===or)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,l,c,h]=this.children,u=e.getRenderTarget(),d=e.getActiveCubeFace(),f=e.getActiveMipmapLevel(),g=e.xr.enabled;e.xr.enabled=!1;let M=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=M,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,d,f),e.xr.enabled=g,n.texture.needsPMREMUpdate=!0}},sl=class extends $t{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},To=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=Im.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function Im(){this._document.hidden===!1&&this.reset()}var rl=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,o;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,o=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,o=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,o=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=o,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,o=this.cumulativeWeight;if(o===0){for(let a=0;a!==i;++a)n[r+a]=n[a];o=t}else{o+=t;let a=t/o;this._mixBufferRegion(n,r,0,a,i)}this.cumulativeWeight=o}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,o=this.cumulativeWeightAdditive,a=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}o>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){a.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,o=i;r!==o;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let o=0;o!==r;++o)e[t+o]=e[n+o]}_slerp(e,t,n,i){Zt.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let o=this._workIndex*r;Zt.multiplyQuaternionsFlat(e,o,e,t,e,n),Zt.slerpFlat(e,t,e,t,e,o,i)}_lerp(e,t,n,i,r){let o=1-i;for(let a=0;a!==r;++a){let l=t+a;e[l]=e[l]*o+e[n+a]*i}}_lerpAdditive(e,t,n,i,r){for(let o=0;o!==r;++o){let a=t+o;e[a]=e[a]+e[n+o]*i}}},Th="\\[\\]\\.:\\/",Lm=new RegExp("["+Th+"]","g"),Eh="[^"+Th+"]",Dm="[^"+Th.replace("\\.","")+"]",Nm=/((?:WC+[\/:])*)/.source.replace("WC",Eh),Um=/(WCOD+)?/.source.replace("WCOD",Dm),Fm=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Eh),Om=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Eh),Bm=new RegExp("^"+Nm+Um+Fm+Om+"$"),zm=["material","materials","bones","map"],oh=class{constructor(e,t,n){let i=n||Rt.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},Rt=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(Lm,"")}static parseTrackName(e){let t=Bm.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);zm.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===t||a.uuid===t)return a;let l=n(a.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){Xe("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){Qe("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){Qe("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){Qe("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){Qe("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){Qe("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){Qe("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){Qe("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let o=e[i];if(o===void 0){let c=t.nodeName;Qe("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let a=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?a=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){Qe("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){Qe("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(l=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};Rt.Composite=oh;Rt.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};Rt.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};Rt.prototype.GetterByBindingType=[Rt.prototype._getValue_direct,Rt.prototype._getValue_array,Rt.prototype._getValue_arrayElement,Rt.prototype._getValue_toArray];Rt.prototype.SetterByBindingTypeAndVersioning=[[Rt.prototype._setValue_direct,Rt.prototype._setValue_direct_setNeedsUpdate,Rt.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[Rt.prototype._setValue_array,Rt.prototype._setValue_array_setNeedsUpdate,Rt.prototype._setValue_array_setMatrixWorldNeedsUpdate],[Rt.prototype._setValue_arrayElement,Rt.prototype._setValue_arrayElement_setNeedsUpdate,Rt.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[Rt.prototype._setValue_fromArray,Rt.prototype._setValue_fromArray_setNeedsUpdate,Rt.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var ol=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,o=r.length,a=new Array(o),l={endingStart:vs,endingEnd:vs};for(let c=0;c!==o;++c){let h=r[c].createInterpolant(null);a[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=a,this._propertyBindings=new Array(o),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Wd,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,o=r/i,a=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,o,t),this.warp(a,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,o=this.timeScale,a=this._timeScaleInterpolant;a===null&&(a=i._lendControlInterpolant(),this._timeScaleInterpolant=a);let l=a.parameterPositions,c=a.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/o,c[1]=t/o,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let o=this._updateTime(t),a=this._updateWeight(e);if(a>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case qd:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulateAdditive(a);break;case jl:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulate(i,a)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,o=n===Xd;if(e===0)return r===-1?i:o&&(r&1)===1?t-i:i;if(n===Kl){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,o)):this._setEndings(this.repetitions===0,!0,o)),i>=t||i<0){let a=Math.floor(i/t);i-=t*a,r+=Math.abs(a);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,o)}else this._setEndings(!1,!1,o);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:a})}}else this._loopCount=r,this.time=i;if(o&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=ys,i.endingEnd=ys):(e?i.endingStart=this.zeroSlopeAtStart?ys:vs:i.endingStart=$r,t?i.endingEnd=this.zeroSlopeAtEnd?ys:vs:i.endingEnd=$r)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,o=this._weightInterpolant;o===null&&(o=i._lendControlInterpolant(),this._weightInterpolant=o);let a=o.parameterPositions,l=o.sampleValues;return a[0]=r,l[0]=t,a[1]=r+e,l[1]=n,this}},km=new Float32Array(1),Eo=class extends Vn{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,o=e._propertyBindings,a=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let d=i[u],f=d.name,g=h[f];if(g!==void 0)++g.referenceCount,o[u]=g;else{if(g=o[u],g!==void 0){g._cacheIndex===null&&(++g.referenceCount,this._addInactiveBinding(g,l,f));continue}let M=t&&t._propertyBindings[u].binding.parsedPath;g=new rl(Rt.create(n,f,M),d.ValueTypeName,d.getValueSize()),++g.referenceCount,this._addInactiveBinding(g,l,f),o[u]=g}a[u].resultBuffer=g.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,o=r[t];if(o===void 0)o={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=o;else{let a=o.knownActions;e._byClipCacheIndex=a.length,a.push(e)}e._cacheIndex=i.length,i.push(e),o.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,o=this._actionsByClip,a=o[r],l=a.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=a.actionByRoot,d=(e._localRoot||this._root).uuid;delete u[d],l.length===0&&delete o[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,o=i[t];o===void 0&&(o={},i[t]=o),o[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,o=this._bindingsByRootAndName,a=o[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete a[r],Object.keys(a).length===0&&delete o[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new go(new Float32Array(2),new Float32Array(2),1,km),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,o=typeof e=="string"?Ls.findByName(i,e):e,a=o!==null?o.uuid:e,l=this._actionsByClip[a],c=null;if(n===void 0&&(o!==null?n=o.blendMode:n=jl),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],o===null&&(o=c._clip)}if(o===null)return null;let h=new ol(this,o,t,n);return this._bindAction(h,c),this._addInactiveAction(h,a,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Ls.findByName(n,e):e,o=r?r.uuid:e,a=this._actionsByClip[o];return a!==void 0&&a.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),o=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,o);let a=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)a[c].apply(o);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let o=r.knownActions;for(let a=0,l=o.length;a!==l;++a){let c=o[a];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let o in n){let a=n[o].actionByRoot,l=a[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let o in r){let a=r[o];a.restoreOriginalState(),this._removeInactiveBinding(a)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var vd=new et,wo=class{constructor(e,t,n=0,i=1/0){this.ray=new ui(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new hr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):Qe("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return vd.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(vd),this}intersectObject(e,t=!0,n=[]){return ah(e,this,n,t),n.sort(yd),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)ah(e[i],this,n,t);return n.sort(yd),n}};function yd(s,e){return s.distance-e.distance}function ah(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let o=0,a=r.length;o<a;o++)ah(r[o],e,t,!0)}}var Tr=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=ot(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(ot(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var lh=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var Ao=class extends Vn{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){if(e===void 0){Xe("Controls: connect() now requires an element.");return}this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function wh(s,e,t,n){let i=Hm(n);switch(t){case xh:return s*e;case ml:return s*e/i.components*i.byteLength;case gl:return s*e/i.components*i.byteLength;case as:return s*e*2/i.components*i.byteLength;case xl:return s*e*2/i.components*i.byteLength;case _h:return s*e*3/i.components*i.byteLength;case Un:return s*e*4/i.components*i.byteLength;case _l:return s*e*4/i.components*i.byteLength;case Fo:case Oo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Bo:case zo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case yl:case bl:return Math.max(s,16)*Math.max(e,8)/4;case vl:case Ml:return Math.max(s,8)*Math.max(e,8)/2;case Sl:case Tl:case wl:case Al:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case El:case ko:case Rl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Cl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Pl:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case Il:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case Ll:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case Dl:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case Nl:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case Ul:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case Fl:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case Ol:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case Bl:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case zl:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case kl:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case Hl:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Vl:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case Gl:case Wl:case Xl:return Math.ceil(s/4)*Math.ceil(e/4)*16;case ql:case Yl:return Math.ceil(s/4)*Math.ceil(e/4)*8;case Ho:case Zl:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function Hm(s){switch(s){case Sn:case fh:return{byteLength:1,components:1};case Ar:case ph:case nn:return{byteLength:2,components:1};case fl:case pl:return{byteLength:2,components:4};case ii:case dl:case Nn:return{byteLength:4,components:1};case mh:case gh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"185"}}));typeof window<"u"&&(window.__THREE__?Xe("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="185");function Pf(){let s=null,e=!1,t=null,n=null;function i(r,o){t(r,o),n=s.requestAnimationFrame(i)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function Gm(s){let e=new WeakMap;function t(a,l){let c=a.array,h=a.usage,u=c.byteLength,d=s.createBuffer();s.bindBuffer(l,d),s.bufferData(l,c,h),a.onUploadCallback();let f;if(c instanceof Float32Array)f=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)f=s.HALF_FLOAT;else if(c instanceof Uint16Array)a.isFloat16BufferAttribute?f=s.HALF_FLOAT:f=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)f=s.SHORT;else if(c instanceof Uint32Array)f=s.UNSIGNED_INT;else if(c instanceof Int32Array)f=s.INT;else if(c instanceof Int8Array)f=s.BYTE;else if(c instanceof Uint8Array)f=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)f=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:d,type:f,bytesPerElement:c.BYTES_PER_ELEMENT,version:a.version,size:u}}function n(a,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,a),u.length===0)s.bufferSubData(c,0,h);else{u.sort((f,g)=>f.start-g.start);let d=0;for(let f=1;f<u.length;f++){let g=u[d],M=u[f];M.start<=g.start+g.count+1?g.count=Math.max(g.count,M.start+M.count-g.start):(++d,u[d]=M)}u.length=d+1;for(let f=0,g=u.length;f<g;f++){let M=u[f];s.bufferSubData(c,M.start*h.BYTES_PER_ELEMENT,h,M.start,M.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(a){return a.isInterleavedBufferAttribute&&(a=a.data),e.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let l=e.get(a);l&&(s.deleteBuffer(l.buffer),e.delete(a))}function o(a,l){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let h=e.get(a);(!h||h.version<a.version)&&e.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let c=e.get(a);if(c===void 0)e.set(a,t(a,l));else if(c.version<a.version){if(c.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,a,l),c.version=a.version}}return{get:i,remove:r,update:o}}var Wm=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,Xm=`#ifdef USE_ALPHAHASH
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
#endif`,qm=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,Ym=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,Zm=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,Km=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,jm=`#ifdef USE_AOMAP
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
#endif`,Jm=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,$m=`#ifdef USE_BATCHING
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
#endif`,Qm=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,eg=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,tg=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,ng=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,ig=`#ifdef USE_IRIDESCENCE
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
#endif`,sg=`#ifdef USE_BUMPMAP
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
#endif`,rg=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,og=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,ag=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,lg=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,cg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,hg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,ug=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,dg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,fg=`#define PI 3.141592653589793
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
} // validated`,pg=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,mg=`vec3 transformedNormal = objectNormal;
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
#endif`,gg=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,xg=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,_g=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,vg=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,yg="gl_FragColor = linearToOutputTexel( gl_FragColor );",Mg=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,bg=`#ifdef USE_ENVMAP
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
#endif`,Sg=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,Tg=`#ifdef USE_ENVMAP
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
#endif`,Eg=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,wg=`#ifdef USE_ENVMAP
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
#endif`,Ag=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,Rg=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,Cg=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,Pg=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Ig=`#ifdef USE_GRADIENTMAP
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
}`,Lg=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Dg=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Ng=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,Ug=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,Fg=`#ifdef USE_ENVMAP
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
#endif`,Og=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,Bg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,zg=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,kg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,Hg=`PhysicalMaterial material;
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
#endif`,Vg=`uniform sampler2D dfgLUT;
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
}`,Gg=`
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
#endif`,Wg=`#if defined( RE_IndirectDiffuse )
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
#endif`,Xg=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,qg=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,Yg=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,Zg=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,Kg=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,jg=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,Jg=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,$g=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,Qg=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,e0=`#if defined( USE_POINTS_UV )
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
#endif`,t0=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,n0=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,i0=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,s0=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,r0=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,o0=`#ifdef USE_MORPHTARGETS
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
#endif`,a0=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,l0=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,c0=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,h0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,u0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,d0=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,f0=`#ifdef USE_NORMALMAP
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
#endif`,p0=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,m0=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,g0=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,x0=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,_0=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,v0=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,y0=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,M0=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,b0=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,S0=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,T0=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,E0=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,w0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,A0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,R0=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,C0=`float getShadowMask() {
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
}`,P0=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,I0=`#ifdef USE_SKINNING
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
#endif`,L0=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,D0=`#ifdef USE_SKINNING
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
#endif`,N0=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,U0=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,F0=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,O0=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,B0=`#ifdef USE_TRANSMISSION
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
#endif`,z0=`#ifdef USE_TRANSMISSION
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
#endif`,k0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,H0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,V0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,G0=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,W0=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,X0=`uniform sampler2D t2D;
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
}`,q0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,Y0=`#ifdef ENVMAP_TYPE_CUBE
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
}`,Z0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,K0=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,j0=`#include <common>
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
}`,J0=`#if DEPTH_PACKING == 3200
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
}`,$0=`#define DISTANCE
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
}`,Q0=`#define DISTANCE
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
}`,ex=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,tx=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,nx=`uniform float scale;
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
}`,ix=`uniform vec3 diffuse;
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
}`,sx=`#include <common>
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
}`,rx=`uniform vec3 diffuse;
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
}`,ox=`#define LAMBERT
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
}`,ax=`#define LAMBERT
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
}`,lx=`#define MATCAP
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
}`,cx=`#define MATCAP
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
}`,hx=`#define NORMAL
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
}`,ux=`#define NORMAL
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
}`,dx=`#define PHONG
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
}`,fx=`#define PHONG
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
}`,px=`#define STANDARD
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
}`,mx=`#define STANDARD
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
}`,gx=`#define TOON
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
}`,xx=`#define TOON
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
}`,_x=`uniform float size;
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
}`,vx=`uniform vec3 diffuse;
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
}`,yx=`#include <common>
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
}`,Mx=`uniform vec3 color;
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
}`,bx=`uniform float rotation;
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
}`,Sx=`uniform vec3 diffuse;
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
}`,ct={alphahash_fragment:Wm,alphahash_pars_fragment:Xm,alphamap_fragment:qm,alphamap_pars_fragment:Ym,alphatest_fragment:Zm,alphatest_pars_fragment:Km,aomap_fragment:jm,aomap_pars_fragment:Jm,batching_pars_vertex:$m,batching_vertex:Qm,begin_vertex:eg,beginnormal_vertex:tg,bsdfs:ng,iridescence_fragment:ig,bumpmap_pars_fragment:sg,clipping_planes_fragment:rg,clipping_planes_pars_fragment:og,clipping_planes_pars_vertex:ag,clipping_planes_vertex:lg,color_fragment:cg,color_pars_fragment:hg,color_pars_vertex:ug,color_vertex:dg,common:fg,cube_uv_reflection_fragment:pg,defaultnormal_vertex:mg,displacementmap_pars_vertex:gg,displacementmap_vertex:xg,emissivemap_fragment:_g,emissivemap_pars_fragment:vg,colorspace_fragment:yg,colorspace_pars_fragment:Mg,envmap_fragment:bg,envmap_common_pars_fragment:Sg,envmap_pars_fragment:Tg,envmap_pars_vertex:Eg,envmap_physical_pars_fragment:Fg,envmap_vertex:wg,fog_vertex:Ag,fog_pars_vertex:Rg,fog_fragment:Cg,fog_pars_fragment:Pg,gradientmap_pars_fragment:Ig,lightmap_pars_fragment:Lg,lights_lambert_fragment:Dg,lights_lambert_pars_fragment:Ng,lights_pars_begin:Ug,lights_toon_fragment:Og,lights_toon_pars_fragment:Bg,lights_phong_fragment:zg,lights_phong_pars_fragment:kg,lights_physical_fragment:Hg,lights_physical_pars_fragment:Vg,lights_fragment_begin:Gg,lights_fragment_maps:Wg,lights_fragment_end:Xg,lightprobes_pars_fragment:qg,logdepthbuf_fragment:Yg,logdepthbuf_pars_fragment:Zg,logdepthbuf_pars_vertex:Kg,logdepthbuf_vertex:jg,map_fragment:Jg,map_pars_fragment:$g,map_particle_fragment:Qg,map_particle_pars_fragment:e0,metalnessmap_fragment:t0,metalnessmap_pars_fragment:n0,morphinstance_vertex:i0,morphcolor_vertex:s0,morphnormal_vertex:r0,morphtarget_pars_vertex:o0,morphtarget_vertex:a0,normal_fragment_begin:l0,normal_fragment_maps:c0,normal_pars_fragment:h0,normal_pars_vertex:u0,normal_vertex:d0,normalmap_pars_fragment:f0,clearcoat_normal_fragment_begin:p0,clearcoat_normal_fragment_maps:m0,clearcoat_pars_fragment:g0,iridescence_pars_fragment:x0,opaque_fragment:_0,packing:v0,premultiplied_alpha_fragment:y0,project_vertex:M0,dithering_fragment:b0,dithering_pars_fragment:S0,roughnessmap_fragment:T0,roughnessmap_pars_fragment:E0,shadowmap_pars_fragment:w0,shadowmap_pars_vertex:A0,shadowmap_vertex:R0,shadowmask_pars_fragment:C0,skinbase_vertex:P0,skinning_pars_vertex:I0,skinning_vertex:L0,skinnormal_vertex:D0,specularmap_fragment:N0,specularmap_pars_fragment:U0,tonemapping_fragment:F0,tonemapping_pars_fragment:O0,transmission_fragment:B0,transmission_pars_fragment:z0,uv_pars_fragment:k0,uv_pars_vertex:H0,uv_vertex:V0,worldpos_vertex:G0,background_vert:W0,background_frag:X0,backgroundCube_vert:q0,backgroundCube_frag:Y0,cube_vert:Z0,cube_frag:K0,depth_vert:j0,depth_frag:J0,distance_vert:$0,distance_frag:Q0,equirect_vert:ex,equirect_frag:tx,linedashed_vert:nx,linedashed_frag:ix,meshbasic_vert:sx,meshbasic_frag:rx,meshlambert_vert:ox,meshlambert_frag:ax,meshmatcap_vert:lx,meshmatcap_frag:cx,meshnormal_vert:hx,meshnormal_frag:ux,meshphong_vert:dx,meshphong_frag:fx,meshphysical_vert:px,meshphysical_frag:mx,meshtoon_vert:gx,meshtoon_frag:xx,points_vert:_x,points_frag:vx,shadow_vert:yx,shadow_frag:Mx,sprite_vert:bx,sprite_frag:Sx},Ce={common:{diffuse:{value:new xe(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new tt},alphaMap:{value:null},alphaMapTransform:{value:new tt},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new tt}},envmap:{envMap:{value:null},envMapRotation:{value:new tt},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new tt}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new tt}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new tt},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new tt},normalScale:{value:new ve(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new tt},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new tt}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new tt}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new tt}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new xe(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new z},probesMax:{value:new z},probesResolution:{value:new z}},points:{diffuse:{value:new xe(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new tt},alphaTest:{value:0},uvTransform:{value:new tt}},sprite:{diffuse:{value:new xe(16777215)},opacity:{value:1},center:{value:new ve(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new tt},alphaMap:{value:null},alphaMapTransform:{value:new tt},alphaTest:{value:0}}},vi={basic:{uniforms:fn([Ce.common,Ce.specularmap,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.fog]),vertexShader:ct.meshbasic_vert,fragmentShader:ct.meshbasic_frag},lambert:{uniforms:fn([Ce.common,Ce.specularmap,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.fog,Ce.lights,{emissive:{value:new xe(0)},envMapIntensity:{value:1}}]),vertexShader:ct.meshlambert_vert,fragmentShader:ct.meshlambert_frag},phong:{uniforms:fn([Ce.common,Ce.specularmap,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.fog,Ce.lights,{emissive:{value:new xe(0)},specular:{value:new xe(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:ct.meshphong_vert,fragmentShader:ct.meshphong_frag},standard:{uniforms:fn([Ce.common,Ce.envmap,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.roughnessmap,Ce.metalnessmap,Ce.fog,Ce.lights,{emissive:{value:new xe(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:ct.meshphysical_vert,fragmentShader:ct.meshphysical_frag},toon:{uniforms:fn([Ce.common,Ce.aomap,Ce.lightmap,Ce.emissivemap,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.gradientmap,Ce.fog,Ce.lights,{emissive:{value:new xe(0)}}]),vertexShader:ct.meshtoon_vert,fragmentShader:ct.meshtoon_frag},matcap:{uniforms:fn([Ce.common,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,Ce.fog,{matcap:{value:null}}]),vertexShader:ct.meshmatcap_vert,fragmentShader:ct.meshmatcap_frag},points:{uniforms:fn([Ce.points,Ce.fog]),vertexShader:ct.points_vert,fragmentShader:ct.points_frag},dashed:{uniforms:fn([Ce.common,Ce.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:ct.linedashed_vert,fragmentShader:ct.linedashed_frag},depth:{uniforms:fn([Ce.common,Ce.displacementmap]),vertexShader:ct.depth_vert,fragmentShader:ct.depth_frag},normal:{uniforms:fn([Ce.common,Ce.bumpmap,Ce.normalmap,Ce.displacementmap,{opacity:{value:1}}]),vertexShader:ct.meshnormal_vert,fragmentShader:ct.meshnormal_frag},sprite:{uniforms:fn([Ce.sprite,Ce.fog]),vertexShader:ct.sprite_vert,fragmentShader:ct.sprite_frag},background:{uniforms:{uvTransform:{value:new tt},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:ct.background_vert,fragmentShader:ct.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new tt}},vertexShader:ct.backgroundCube_vert,fragmentShader:ct.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:ct.cube_vert,fragmentShader:ct.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:ct.equirect_vert,fragmentShader:ct.equirect_frag},distance:{uniforms:fn([Ce.common,Ce.displacementmap,{referencePosition:{value:new z},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:ct.distance_vert,fragmentShader:ct.distance_frag},shadow:{uniforms:fn([Ce.lights,Ce.fog,{color:{value:new xe(0)},opacity:{value:1}}]),vertexShader:ct.shadow_vert,fragmentShader:ct.shadow_frag}};vi.physical={uniforms:fn([vi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new tt},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new tt},clearcoatNormalScale:{value:new ve(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new tt},dispersion:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new tt},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new tt},sheen:{value:0},sheenColor:{value:new xe(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new tt},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new tt},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new tt},transmissionSamplerSize:{value:new ve},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new tt},attenuationDistance:{value:0},attenuationColor:{value:new xe(0)},specularColor:{value:new xe(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new tt},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new tt},anisotropyVector:{value:new ve},anisotropyMap:{value:null},anisotropyMapTransform:{value:new tt}}]),vertexShader:ct.meshphysical_vert,fragmentShader:ct.meshphysical_frag};var Ql={r:0,b:0,g:0},Tx=new et,If=new tt;If.set(-1,0,0,0,1,0,0,0,1);function Ex(s,e,t,n,i,r){let o=new xe(0),a=i===!0?0:1,l,c,h=null,u=0,d=null;function f(v){let S=v.isScene===!0?v.background:null;if(S&&S.isTexture){let _=v.backgroundBlurriness>0;S=e.get(S,_)}return S}function g(v){let S=!1,_=f(v);_===null?m(o,a):_&&_.isColor&&(m(_,1),S=!0);let w=s.xr.getEnvironmentBlendMode();w==="additive"?t.buffers.color.setClear(0,0,0,1,r):w==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||S)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function M(v,S){let _=f(S);_&&(_.isCubeTexture||_.mapping===Uo)?(c===void 0&&(c=new je(new fi(1,1,1),new mt({name:"BackgroundCubeMaterial",uniforms:Os(vi.backgroundCube.uniforms),vertexShader:vi.backgroundCube.vertexShader,fragmentShader:vi.backgroundCube.fragmentShader,side:tn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(w,T,L){this.matrixWorld.copyPosition(L.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=_,c.material.uniforms.backgroundBlurriness.value=S.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=S.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Tx.makeRotationFromEuler(S.backgroundRotation)).transpose(),_.isCubeTexture&&_.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(If),c.material.toneMapped=rt.getTransfer(_.colorSpace)!==yt,(h!==_||u!==_.version||d!==s.toneMapping)&&(c.material.needsUpdate=!0,h=_,u=_.version,d=s.toneMapping),c.layers.enableAll(),v.unshift(c,c.geometry,c.material,0,0,null)):_&&_.isTexture&&(l===void 0&&(l=new je(new ln(2,2),new mt({name:"BackgroundMaterial",uniforms:Os(vi.background.uniforms),vertexShader:vi.background.vertexShader,fragmentShader:vi.background.fragmentShader,side:In,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=_,l.material.uniforms.backgroundIntensity.value=S.backgroundIntensity,l.material.toneMapped=rt.getTransfer(_.colorSpace)!==yt,_.matrixAutoUpdate===!0&&_.updateMatrix(),l.material.uniforms.uvTransform.value.copy(_.matrix),(h!==_||u!==_.version||d!==s.toneMapping)&&(l.material.needsUpdate=!0,h=_,u=_.version,d=s.toneMapping),l.layers.enableAll(),v.unshift(l,l.geometry,l.material,0,0,null))}function m(v,S){v.getRGB(Ql,Sh(s)),t.buffers.color.setClear(Ql.r,Ql.g,Ql.b,S,r)}function p(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return o},setClearColor:function(v,S=1){o.set(v),a=S,m(o,a)},getClearAlpha:function(){return a},setClearAlpha:function(v){a=v,m(o,a)},render:g,addToRenderList:M,dispose:p}}function wx(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=d(null),r=i,o=!1;function a(R,F,X,P,N){let O=!1,A=u(R,P,X,F);r!==A&&(r=A,c(r.object)),O=f(R,P,X,N),O&&g(R,P,X,N),N!==null&&e.update(N,s.ELEMENT_ARRAY_BUFFER),(O||o)&&(o=!1,_(R,F,X,P),N!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(N).buffer))}function l(){return s.createVertexArray()}function c(R){return s.bindVertexArray(R)}function h(R){return s.deleteVertexArray(R)}function u(R,F,X,P){let N=P.wireframe===!0,O=n[F.id];O===void 0&&(O={},n[F.id]=O);let A=R.isInstancedMesh===!0?R.id:0,V=O[A];V===void 0&&(V={},O[A]=V);let Y=V[X.id];Y===void 0&&(Y={},V[X.id]=Y);let G=Y[N];return G===void 0&&(G=d(l()),Y[N]=G),G}function d(R){let F=[],X=[],P=[];for(let N=0;N<t;N++)F[N]=0,X[N]=0,P[N]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:F,enabledAttributes:X,attributeDivisors:P,object:R,attributes:{},index:null}}function f(R,F,X,P){let N=r.attributes,O=F.attributes,A=0,V=X.getAttributes();for(let Y in V)if(V[Y].location>=0){let ne=N[Y],le=O[Y];if(le===void 0&&(Y==="instanceMatrix"&&R.instanceMatrix&&(le=R.instanceMatrix),Y==="instanceColor"&&R.instanceColor&&(le=R.instanceColor)),ne===void 0||ne.attribute!==le||le&&ne.data!==le.data)return!0;A++}return r.attributesNum!==A||r.index!==P}function g(R,F,X,P){let N={},O=F.attributes,A=0,V=X.getAttributes();for(let Y in V)if(V[Y].location>=0){let ne=O[Y];ne===void 0&&(Y==="instanceMatrix"&&R.instanceMatrix&&(ne=R.instanceMatrix),Y==="instanceColor"&&R.instanceColor&&(ne=R.instanceColor));let le={};le.attribute=ne,ne&&ne.data&&(le.data=ne.data),N[Y]=le,A++}r.attributes=N,r.attributesNum=A,r.index=P}function M(){let R=r.newAttributes;for(let F=0,X=R.length;F<X;F++)R[F]=0}function m(R){p(R,0)}function p(R,F){let X=r.newAttributes,P=r.enabledAttributes,N=r.attributeDivisors;X[R]=1,P[R]===0&&(s.enableVertexAttribArray(R),P[R]=1),N[R]!==F&&(s.vertexAttribDivisor(R,F),N[R]=F)}function v(){let R=r.newAttributes,F=r.enabledAttributes;for(let X=0,P=F.length;X<P;X++)F[X]!==R[X]&&(s.disableVertexAttribArray(X),F[X]=0)}function S(R,F,X,P,N,O,A){A===!0?s.vertexAttribIPointer(R,F,X,N,O):s.vertexAttribPointer(R,F,X,P,N,O)}function _(R,F,X,P){M();let N=P.attributes,O=X.getAttributes(),A=F.defaultAttributeValues;for(let V in O){let Y=O[V];if(Y.location>=0){let G=N[V];if(G===void 0&&(V==="instanceMatrix"&&R.instanceMatrix&&(G=R.instanceMatrix),V==="instanceColor"&&R.instanceColor&&(G=R.instanceColor)),G!==void 0){let ne=G.normalized,le=G.itemSize,Ae=e.get(G);if(Ae===void 0)continue;let Pe=Ae.buffer,Re=Ae.type,oe=Ae.bytesPerElement,pe=Re===s.INT||Re===s.UNSIGNED_INT||G.gpuType===dl;if(G.isInterleavedBufferAttribute){let ce=G.data,Te=ce.stride,ue=G.offset;if(ce.isInstancedInterleavedBuffer){for(let he=0;he<Y.locationSize;he++)p(Y.location+he,ce.meshPerAttribute);R.isInstancedMesh!==!0&&P._maxInstanceCount===void 0&&(P._maxInstanceCount=ce.meshPerAttribute*ce.count)}else for(let he=0;he<Y.locationSize;he++)m(Y.location+he);s.bindBuffer(s.ARRAY_BUFFER,Pe);for(let he=0;he<Y.locationSize;he++)S(Y.location+he,le/Y.locationSize,Re,ne,Te*oe,(ue+le/Y.locationSize*he)*oe,pe)}else{if(G.isInstancedBufferAttribute){for(let ce=0;ce<Y.locationSize;ce++)p(Y.location+ce,G.meshPerAttribute);R.isInstancedMesh!==!0&&P._maxInstanceCount===void 0&&(P._maxInstanceCount=G.meshPerAttribute*G.count)}else for(let ce=0;ce<Y.locationSize;ce++)m(Y.location+ce);s.bindBuffer(s.ARRAY_BUFFER,Pe);for(let ce=0;ce<Y.locationSize;ce++)S(Y.location+ce,le/Y.locationSize,Re,ne,le*oe,le/Y.locationSize*ce*oe,pe)}}else if(A!==void 0){let ne=A[V];if(ne!==void 0)switch(ne.length){case 2:s.vertexAttrib2fv(Y.location,ne);break;case 3:s.vertexAttrib3fv(Y.location,ne);break;case 4:s.vertexAttrib4fv(Y.location,ne);break;default:s.vertexAttrib1fv(Y.location,ne)}}}}v()}function w(){I();for(let R in n){let F=n[R];for(let X in F){let P=F[X];for(let N in P){let O=P[N];for(let A in O)h(O[A].object),delete O[A];delete P[N]}}delete n[R]}}function T(R){if(n[R.id]===void 0)return;let F=n[R.id];for(let X in F){let P=F[X];for(let N in P){let O=P[N];for(let A in O)h(O[A].object),delete O[A];delete P[N]}}delete n[R.id]}function L(R){for(let F in n){let X=n[F];for(let P in X){let N=X[P];if(N[R.id]===void 0)continue;let O=N[R.id];for(let A in O)h(O[A].object),delete O[A];delete N[R.id]}}}function y(R){for(let F in n){let X=n[F],P=R.isInstancedMesh===!0?R.id:0,N=X[P];if(N!==void 0){for(let O in N){let A=N[O];for(let V in A)h(A[V].object),delete A[V];delete N[O]}delete X[P],Object.keys(X).length===0&&delete n[F]}}}function I(){E(),o=!0,r!==i&&(r=i,c(r.object))}function E(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:a,reset:I,resetDefaultState:E,dispose:w,releaseStatesOfGeometry:T,releaseStatesOfObject:y,releaseStatesOfProgram:L,initAttributes:M,enableAttribute:m,disableUnusedAttributes:v}}function Ax(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function o(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function a(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let d=0;for(let f=0;f<h;f++)d+=c[f];t.update(d,n,1)}this.setMode=i,this.render=r,this.renderInstances=o,this.renderMultiDraw=a}function Rx(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let L=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(L.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function o(L){return!(L!==Un&&n.convert(L)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(L){let y=L===nn&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(L!==Sn&&n.convert(L)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE)&&L!==Nn&&!y)}function l(L){if(L==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";L="mediump"}return L==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(Xe("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,d=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&d===!1&&Xe("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let f=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),g=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),M=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),p=s.getParameter(s.MAX_VERTEX_ATTRIBS),v=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),S=s.getParameter(s.MAX_VARYING_VECTORS),_=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),w=s.getParameter(s.MAX_SAMPLES),T=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:o,textureTypeReadable:a,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:d,maxTextures:f,maxVertexTextures:g,maxTextureSize:M,maxCubemapSize:m,maxAttributes:p,maxVertexUniforms:v,maxVaryings:S,maxFragmentUniforms:_,maxSamples:w,samples:T}}function Cx(s){let e=this,t=null,n=0,i=!1,r=!1,o=new kn,a=new tt,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,d){let f=u.length!==0||d||n!==0||i;return i=d,n=u.length,f},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,d){t=h(u,d,0)},this.setState=function(u,d,f){let g=u.clippingPlanes,M=u.clipIntersection,m=u.clipShadows,p=s.get(u);if(!i||g===null||g.length===0||r&&!m)r?h(null):c();else{let v=r?0:n,S=v*4,_=p.clippingState||null;l.value=_,_=h(g,d,S,f);for(let w=0;w!==S;++w)_[w]=t[w];p.clippingState=_,this.numIntersection=M?this.numPlanes:0,this.numPlanes+=v}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,d,f,g){let M=u!==null?u.length:0,m=null;if(M!==0){if(m=l.value,g!==!0||m===null){let p=f+M*4,v=d.matrixWorldInverse;a.getNormalMatrix(v),(m===null||m.length<p)&&(m=new Float32Array(p));for(let S=0,_=f;S!==M;++S,_+=4)o.copy(u[S]).applyMatrix4(v,a),o.normal.toArray(m,_),m[_+3]=o.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=M,e.numIntersection=0,m}}var ls=4,lf=[.125,.215,.35,.446,.526,.582],Bs=20,Px=256,Wo=new xi,cf=new xe,Ah=null,Rh=0,Ch=0,Ph=!1,Ix=new z,Lr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._sigmas=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:o=256,position:a=Ix}=r;Ah=this._renderer.getRenderTarget(),Rh=this._renderer.getActiveCubeFace(),Ch=this._renderer.getActiveMipmapLevel(),Ph=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(o);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,a),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=df(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=uf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Ah,Rh,Ch),this._renderer.xr.enabled=Ph,e.scissorTest=!1,Pr(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===rs||e.mapping===Us?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Ah=this._renderer.getRenderTarget(),Rh=this._renderer.getActiveCubeFace(),Ch=this._renderer.getActiveMipmapLevel(),Ph=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:Vt,minFilter:Vt,generateMipmaps:!1,type:nn,format:Un,colorSpace:mn,depthBuffer:!1},i=hf(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=hf(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods,sigmas:this._sigmas}=Lx(r)),this._blurMaterial=Nx(r,e,t),this._ggxMaterial=Dx(r,e,t)}return i}_compileMaterial(e){let t=new je(new pt,e);this._renderer.compile(t,Wo)}_sceneToCubeUV(e,t,n,i,r){let l=new $t(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,d=u.autoClear,f=u.toneMapping;u.getClearColor(cf),u.toneMapping=ti,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new je(new fi,new bt({name:"PMREM.Background",side:tn,depthWrite:!1,depthTest:!1})));let M=this._backgroundBox,m=M.material,p=!1,v=e.background;v?v.isColor&&(m.color.copy(v),e.background=null,p=!0):(m.color.copy(cf),p=!0);for(let S=0;S<6;S++){let _=S%3;_===0?(l.up.set(0,c[S],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[S],r.y,r.z)):_===1?(l.up.set(0,0,c[S]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[S],r.z)):(l.up.set(0,c[S],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[S]));let w=this._cubeSize;Pr(i,_*w,S>2?w:0,w,w),u.setRenderTarget(i),p&&u.render(M,l),u.render(e,l)}u.toneMapping=f,u.autoClear=d,e.background=v}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===rs||e.mapping===Us;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=df()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=uf());let r=i?this._cubemapMaterial:this._equirectMaterial,o=this._lodMeshes[0];o.material=r;let a=r.uniforms;a.envMap.value=e;let l=this._cubeSize;Pr(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(o,Wo)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,o=this._ggxMaterial,a=this._lodMeshes[n];a.material=o;let l=o.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),d=0+c*1.25,f=u*d,{_lodMax:g}=this,M=this._sizeLods[n],m=3*M*(n>g-ls?n-g+ls:0),p=4*(this._cubeSize-M);l.envMap.value=e.texture,l.roughness.value=f,l.mipInt.value=g-t,Pr(r,m,p,3*M,2*M),i.setRenderTarget(r),i.render(a,Wo),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=g-n,Pr(e,m,p,3*M,2*M),i.setRenderTarget(e),i.render(a,Wo)}_blur(e,t,n,i,r){let o=this._pingPongRenderTarget;this._halfBlur(e,o,t,n,i,"latitudinal",r),this._halfBlur(o,e,n,n,i,"longitudinal",r)}_halfBlur(e,t,n,i,r,o,a){let l=this._renderer,c=this._blurMaterial;o!=="latitudinal"&&o!=="longitudinal"&&Qe("blur direction must be either latitudinal or longitudinal!");let h=3,u=this._lodMeshes[i];u.material=c;let d=c.uniforms,f=this._sizeLods[n]-1,g=isFinite(r)?Math.PI/(2*f):2*Math.PI/(2*Bs-1),M=r/g,m=isFinite(r)?1+Math.floor(h*M):Bs;m>Bs&&Xe(`sigmaRadians, ${r}, is too large and will clip, as it requested ${m} samples when the maximum is set to ${Bs}`);let p=[],v=0;for(let L=0;L<Bs;++L){let y=L/M,I=Math.exp(-y*y/2);p.push(I),L===0?v+=I:L<m&&(v+=2*I)}for(let L=0;L<p.length;L++)p[L]=p[L]/v;d.envMap.value=e.texture,d.samples.value=m,d.weights.value=p,d.latitudinal.value=o==="latitudinal",a&&(d.poleAxis.value=a);let{_lodMax:S}=this;d.dTheta.value=g,d.mipInt.value=S-n;let _=this._sizeLods[i],w=3*_*(i>S-ls?i-S+ls:0),T=4*(this._cubeSize-_);Pr(t,w,T,3*_,2*_),l.setRenderTarget(t),l.render(u,Wo)}};function Lx(s){let e=[],t=[],n=[],i=s,r=s-ls+1+lf.length;for(let o=0;o<r;o++){let a=Math.pow(2,i);e.push(a);let l=1/a;o>s-ls?l=lf[o-s+ls-1]:o===0&&(l=0),t.push(l);let c=1/(a-2),h=-c,u=1+c,d=[h,h,u,h,u,u,h,h,u,u,h,u],f=6,g=6,M=3,m=2,p=1,v=new Float32Array(M*g*f),S=new Float32Array(m*g*f),_=new Float32Array(p*g*f);for(let T=0;T<f;T++){let L=T%3*2/3-1,y=T>2?0:-1,I=[L,y,0,L+2/3,y,0,L+2/3,y+1,0,L,y,0,L+2/3,y+1,0,L,y+1,0];v.set(I,M*g*T),S.set(d,m*g*T);let E=[T,T,T,T,T,T];_.set(E,p*g*T)}let w=new pt;w.setAttribute("position",new xt(v,M)),w.setAttribute("uv",new xt(S,m)),w.setAttribute("faceIndex",new xt(_,p)),n.push(new je(w,null)),i>ls&&i--}return{lodMeshes:n,sizeLods:e,sigmas:t}}function hf(s,e,t){let n=new Wt(s,e,t);return n.texture.mapping=Uo,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function Pr(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function Dx(s,e,t){return new mt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:Px,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:ic(),fragmentShader:`

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
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function Nx(s,e,t){let n=new Float32Array(Bs),i=new z(0,1,0);return new mt({name:"SphericalGaussianBlur",defines:{n:Bs,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},samples:{value:1},weights:{value:n},latitudinal:{value:!1},dTheta:{value:0},mipInt:{value:0},poleAxis:{value:i}},vertexShader:ic(),fragmentShader:`

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
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function uf(){return new mt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:ic(),fragmentShader:`

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
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function df(){return new mt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:ic(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function ic(){return`

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
	`}var tc=class extends Wt{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new lo(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new fi(5,5,5),r=new mt({name:"CubemapFromEquirect",uniforms:Os(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:tn,blending:Wn});r.uniforms.tEquirect.value=t;let o=new je(i,r),a=t.minFilter;return t.minFilter===ni&&(t.minFilter=Vt),new il(1,10,this).update(e,o),t.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let o=0;o<6;o++)e.setRenderTarget(this,o),e.clear(t,n,i);e.setRenderTarget(r)}};function Ux(s){let e=new WeakMap,t=new WeakMap,n=null;function i(d,f=!1){return d==null?null:f?o(d):r(d)}function r(d){if(d&&d.isTexture){let f=d.mapping;if(f===cl||f===hl)if(e.has(d)){let g=e.get(d).texture;return a(g,d.mapping)}else{let g=d.image;if(g&&g.height>0){let M=new tc(g.height);return M.fromEquirectangularTexture(s,d),e.set(d,M),d.addEventListener("dispose",c),a(M.texture,d.mapping)}else return null}}return d}function o(d){if(d&&d.isTexture){let f=d.mapping,g=f===cl||f===hl,M=f===rs||f===Us;if(g||M){let m=t.get(d),p=m!==void 0?m.texture.pmremVersion:0;if(d.isRenderTargetTexture&&d.pmremVersion!==p)return n===null&&(n=new Lr(s)),m=g?n.fromEquirectangular(d,m):n.fromCubemap(d,m),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),m.texture;if(m!==void 0)return m.texture;{let v=d.image;return g&&v&&v.height>0||M&&v&&l(v)?(n===null&&(n=new Lr(s)),m=g?n.fromEquirectangular(d):n.fromCubemap(d),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),d.addEventListener("dispose",h),m.texture):null}}}return d}function a(d,f){return f===cl?d.mapping=rs:f===hl&&(d.mapping=Us),d}function l(d){let f=0,g=6;for(let M=0;M<g;M++)d[M]!==void 0&&f++;return f===g}function c(d){let f=d.target;f.removeEventListener("dispose",c);let g=e.get(f);g!==void 0&&(e.delete(f),g.dispose())}function h(d){let f=d.target;f.removeEventListener("dispose",h);let g=t.get(f);g!==void 0&&(t.delete(f),g.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function Fx(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&Ms("WebGLRenderer: "+n+" extension not supported."),i}}}function Ox(s,e,t,n){let i={},r=new WeakMap;function o(u){let d=u.target;d.index!==null&&e.remove(d.index);for(let g in d.attributes)e.remove(d.attributes[g]);d.removeEventListener("dispose",o),delete i[d.id];let f=r.get(d);f&&(e.remove(f),r.delete(d)),n.releaseStatesOfGeometry(d),d.isInstancedBufferGeometry===!0&&delete d._maxInstanceCount,t.memory.geometries--}function a(u,d){return i[d.id]===!0||(d.addEventListener("dispose",o),i[d.id]=!0,t.memory.geometries++),d}function l(u){let d=u.attributes;for(let f in d)e.update(d[f],s.ARRAY_BUFFER)}function c(u){let d=[],f=u.index,g=u.attributes.position,M=0;if(g===void 0)return;if(f!==null){let v=f.array;M=f.version;for(let S=0,_=v.length;S<_;S+=3){let w=v[S+0],T=v[S+1],L=v[S+2];d.push(w,T,T,L,L,w)}}else{let v=g.array;M=g.version;for(let S=0,_=v.length/3-1;S<_;S+=3){let w=S+0,T=S+1,L=S+2;d.push(w,T,T,L,L,w)}}let m=new(g.count>=65535?so:io)(d,1);m.version=M;let p=r.get(u);p&&e.remove(p),r.set(u,m)}function h(u){let d=r.get(u);if(d){let f=u.index;f!==null&&d.version<f.version&&c(u)}else c(u);return r.get(u)}return{get:a,update:l,getWireframeAttribute:h}}function Bx(s,e,t){let n;function i(u){n=u}let r,o;function a(u){r=u.type,o=u.bytesPerElement}function l(u,d){s.drawElements(n,d,r,u*o),t.update(d,n,1)}function c(u,d,f){f!==0&&(s.drawElementsInstanced(n,d,r,u*o,f),t.update(d,n,f))}function h(u,d,f){if(f===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,d,0,r,u,0,f);let M=0;for(let m=0;m<f;m++)M+=d[m];t.update(M,n,1)}this.setMode=i,this.setIndex=a,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function zx(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,o,a){switch(t.calls++,o){case s.TRIANGLES:t.triangles+=a*(r/3);break;case s.LINES:t.lines+=a*(r/2);break;case s.LINE_STRIP:t.lines+=a*(r-1);break;case s.LINE_LOOP:t.lines+=a*r;break;case s.POINTS:t.points+=a*r;break;default:Qe("WebGLInfo: Unknown draw mode:",o);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function kx(s,e,t){let n=new WeakMap,i=new Mt;function r(o,a,l){let c=o.morphTargetInfluences,h=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,u=h!==void 0?h.length:0,d=n.get(a);if(d===void 0||d.count!==u){let I=function(){L.dispose(),n.delete(a),a.removeEventListener("dispose",I)};d!==void 0&&d.texture.dispose();let f=a.morphAttributes.position!==void 0,g=a.morphAttributes.normal!==void 0,M=a.morphAttributes.color!==void 0,m=a.morphAttributes.position||[],p=a.morphAttributes.normal||[],v=a.morphAttributes.color||[],S=0;f===!0&&(S=1),g===!0&&(S=2),M===!0&&(S=3);let _=a.attributes.position.count*S,w=1;_>e.maxTextureSize&&(w=Math.ceil(_/e.maxTextureSize),_=e.maxTextureSize);let T=new Float32Array(_*w*4*u),L=new to(T,_,w,u);L.type=Nn,L.needsUpdate=!0;let y=S*4;for(let E=0;E<u;E++){let R=m[E],F=p[E],X=v[E],P=_*w*4*E;for(let N=0;N<R.count;N++){let O=N*y;f===!0&&(i.fromBufferAttribute(R,N),T[P+O+0]=i.x,T[P+O+1]=i.y,T[P+O+2]=i.z,T[P+O+3]=0),g===!0&&(i.fromBufferAttribute(F,N),T[P+O+4]=i.x,T[P+O+5]=i.y,T[P+O+6]=i.z,T[P+O+7]=0),M===!0&&(i.fromBufferAttribute(X,N),T[P+O+8]=i.x,T[P+O+9]=i.y,T[P+O+10]=i.z,T[P+O+11]=X.itemSize===4?i.w:1)}}d={count:u,texture:L,size:new ve(_,w)},n.set(a,d),a.addEventListener("dispose",I)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",o.morphTexture,t);else{let f=0;for(let M=0;M<c.length;M++)f+=c[M];let g=a.morphTargetsRelative?1:1-f;l.getUniforms().setValue(s,"morphTargetBaseInfluence",g),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",d.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",d.size)}return{update:r}}function Hx(s,e,t,n,i){let r=new WeakMap;function o(c){let h=i.render.frame,u=c.geometry,d=e.get(c,u);if(r.get(d)!==h&&(e.update(d),r.set(d,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let f=c.skeleton;r.get(f)!==h&&(f.update(),r.set(f,h))}return d}function a(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:o,dispose:a}}var Vx={[Co]:"LINEAR_TONE_MAPPING",[Po]:"REINHARD_TONE_MAPPING",[Io]:"CINEON_TONE_MAPPING",[Ns]:"ACES_FILMIC_TONE_MAPPING",[Do]:"AGX_TONE_MAPPING",[No]:"NEUTRAL_TONE_MAPPING",[Lo]:"CUSTOM_TONE_MAPPING"};function Gx(s,e,t,n,i,r){let o=new Wt(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,depthTexture:i?new Li(e,t):void 0}),a=new Wt(e,t,{type:nn,depthBuffer:!1,stencilBuffer:!1}),l=new pt;l.setAttribute("position",new nt([-1,3,0,-1,-1,0,3,-1,0],3)),l.setAttribute("uv",new nt([0,2,0,0,2,0],2));let c=new br({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),h=new je(l,c),u=new xi(-1,1,1,-1,0,1),d=null,f=null,g=!1,M,m=null,p=[],v=!1;this.setSize=function(S,_){o.setSize(S,_),a.setSize(S,_);for(let w=0;w<p.length;w++){let T=p[w];T.setSize&&T.setSize(S,_)}},this.setEffects=function(S){p=S,v=p.length>0&&p[0].isRenderPass===!0;let _=o.width,w=o.height;for(let T=0;T<p.length;T++){let L=p[T];L.setSize&&L.setSize(_,w)}},this.begin=function(S,_){if(g||S.toneMapping===ti&&p.length===0)return!1;if(m=_,_!==null){let w=_.width,T=_.height;(o.width!==w||o.height!==T)&&this.setSize(w,T)}return v===!1&&S.setRenderTarget(o),M=S.toneMapping,S.toneMapping=ti,!0},this.hasRenderPass=function(){return v},this.end=function(S,_){S.toneMapping=M,g=!0;let w=o,T=a;for(let L=0;L<p.length;L++){let y=p[L];if(y.enabled!==!1&&(y.render(S,T,w,_),y.needsSwap!==!1)){let I=w;w=T,T=I}}if(d!==S.outputColorSpace||f!==S.toneMapping){d=S.outputColorSpace,f=S.toneMapping,c.defines={},rt.getTransfer(d)===yt&&(c.defines.SRGB_TRANSFER="");let L=Vx[f];L&&(c.defines[L]=""),c.needsUpdate=!0}c.uniforms.tDiffuse.value=w.texture,S.setRenderTarget(m),S.render(h,u),m=null,g=!1},this.isCompositing=function(){return g},this.dispose=function(){o.depthTexture&&o.depthTexture.dispose(),o.dispose(),a.dispose(),l.dispose(),c.dispose()}}var Lf=new Qt,Dh=new Li(1,1),Df=new to,Nf=new Ba,Uf=new lo,ff=[],pf=[],mf=new Float32Array(16),gf=new Float32Array(9),xf=new Float32Array(4);function Dr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=ff[i];if(r===void 0&&(r=new Float32Array(i),ff[i]=r),e!==0){n.toArray(r,0);for(let o=1,a=0;o!==e;++o)a+=t,s[o].toArray(r,a)}return r}function sn(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function rn(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function sc(s,e){let t=pf[e];t===void 0&&(t=new Int32Array(e),pf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function Wx(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function Xx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(sn(t,e))return;s.uniform2fv(this.addr,e),rn(t,e)}}function qx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(sn(t,e))return;s.uniform3fv(this.addr,e),rn(t,e)}}function Yx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(sn(t,e))return;s.uniform4fv(this.addr,e),rn(t,e)}}function Zx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(sn(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),rn(t,e)}else{if(sn(t,n))return;xf.set(n),s.uniformMatrix2fv(this.addr,!1,xf),rn(t,n)}}function Kx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(sn(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),rn(t,e)}else{if(sn(t,n))return;gf.set(n),s.uniformMatrix3fv(this.addr,!1,gf),rn(t,n)}}function jx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(sn(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),rn(t,e)}else{if(sn(t,n))return;mf.set(n),s.uniformMatrix4fv(this.addr,!1,mf),rn(t,n)}}function Jx(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function $x(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(sn(t,e))return;s.uniform2iv(this.addr,e),rn(t,e)}}function Qx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(sn(t,e))return;s.uniform3iv(this.addr,e),rn(t,e)}}function e_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(sn(t,e))return;s.uniform4iv(this.addr,e),rn(t,e)}}function t_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function n_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(sn(t,e))return;s.uniform2uiv(this.addr,e),rn(t,e)}}function i_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(sn(t,e))return;s.uniform3uiv(this.addr,e),rn(t,e)}}function s_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(sn(t,e))return;s.uniform4uiv(this.addr,e),rn(t,e)}}function r_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(Dh.compareFunction=t.isReversedDepthBuffer()?$l:Jl,r=Dh):r=Lf,t.setTexture2D(e||r,i)}function o_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||Nf,i)}function a_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||Uf,i)}function l_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||Df,i)}function c_(s){switch(s){case 5126:return Wx;case 35664:return Xx;case 35665:return qx;case 35666:return Yx;case 35674:return Zx;case 35675:return Kx;case 35676:return jx;case 5124:case 35670:return Jx;case 35667:case 35671:return $x;case 35668:case 35672:return Qx;case 35669:case 35673:return e_;case 5125:return t_;case 36294:return n_;case 36295:return i_;case 36296:return s_;case 35678:case 36198:case 36298:case 36306:case 35682:return r_;case 35679:case 36299:case 36307:return o_;case 35680:case 36300:case 36308:case 36293:return a_;case 36289:case 36303:case 36311:case 36292:return l_}}function h_(s,e){s.uniform1fv(this.addr,e)}function u_(s,e){let t=Dr(e,this.size,2);s.uniform2fv(this.addr,t)}function d_(s,e){let t=Dr(e,this.size,3);s.uniform3fv(this.addr,t)}function f_(s,e){let t=Dr(e,this.size,4);s.uniform4fv(this.addr,t)}function p_(s,e){let t=Dr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function m_(s,e){let t=Dr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function g_(s,e){let t=Dr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function x_(s,e){s.uniform1iv(this.addr,e)}function __(s,e){s.uniform2iv(this.addr,e)}function v_(s,e){s.uniform3iv(this.addr,e)}function y_(s,e){s.uniform4iv(this.addr,e)}function M_(s,e){s.uniform1uiv(this.addr,e)}function b_(s,e){s.uniform2uiv(this.addr,e)}function S_(s,e){s.uniform3uiv(this.addr,e)}function T_(s,e){s.uniform4uiv(this.addr,e)}function E_(s,e,t){let n=this.cache,i=e.length,r=sc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));let o;this.type===s.SAMPLER_2D_SHADOW?o=Dh:o=Lf;for(let a=0;a!==i;++a)t.setTexture2D(e[a]||o,r[a])}function w_(s,e,t){let n=this.cache,i=e.length,r=sc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));for(let o=0;o!==i;++o)t.setTexture3D(e[o]||Nf,r[o])}function A_(s,e,t){let n=this.cache,i=e.length,r=sc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));for(let o=0;o!==i;++o)t.setTextureCube(e[o]||Uf,r[o])}function R_(s,e,t){let n=this.cache,i=e.length,r=sc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));for(let o=0;o!==i;++o)t.setTexture2DArray(e[o]||Df,r[o])}function C_(s){switch(s){case 5126:return h_;case 35664:return u_;case 35665:return d_;case 35666:return f_;case 35674:return p_;case 35675:return m_;case 35676:return g_;case 5124:case 35670:return x_;case 35667:case 35671:return __;case 35668:case 35672:return v_;case 35669:case 35673:return y_;case 5125:return M_;case 36294:return b_;case 36295:return S_;case 36296:return T_;case 35678:case 36198:case 36298:case 36306:case 35682:return E_;case 35679:case 36299:case 36307:return w_;case 35680:case 36300:case 36308:case 36293:return A_;case 36289:case 36303:case 36311:case 36292:return R_}}var Nh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=c_(t.type)}},Uh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=C_(t.type)}},Fh=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,o=i.length;r!==o;++r){let a=i[r];a.setValue(e,t[a.id],n)}}},Ih=/(\w+)(\])?(\[|\.)?/g;function _f(s,e){s.seq.push(e),s.map[e.id]=e}function P_(s,e,t){let n=s.name,i=n.length;for(Ih.lastIndex=0;;){let r=Ih.exec(n),o=Ih.lastIndex,a=r[1],l=r[2]==="]",c=r[3];if(l&&(a=a|0),c===void 0||c==="["&&o+2===i){_f(t,c===void 0?new Nh(a,s,e):new Uh(a,s,e));break}else{let u=t.map[a];u===void 0&&(u=new Fh(a),_f(t,u)),t=u}}}var Ir=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let o=0;o<n;++o){let a=e.getActiveUniform(t,o),l=e.getUniformLocation(t,a.name);P_(a,l,this)}let i=[],r=[];for(let o of this.seq)o.type===e.SAMPLER_2D_SHADOW||o.type===e.SAMPLER_CUBE_SHADOW||o.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(o):r.push(o);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,o=t.length;r!==o;++r){let a=t[r],l=n[a.id];l.needsUpdate!==!1&&a.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let o=e[i];o.id in t&&n.push(o)}return n}};function vf(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var I_=37297,L_=0;function D_(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let o=i;o<r;o++){let a=o+1;n.push(`${a===e?">":" "} ${a}: ${t[o]}`)}return n.join(`
`)}var yf=new tt;function N_(s){rt._getMatrix(yf,rt.workingColorSpace,s);let e=`mat3( ${yf.elements.map(t=>t.toFixed(4))} )`;switch(rt.getTransfer(s)){case Qr:return[e,"LinearTransferOETF"];case yt:return[e,"sRGBTransferOETF"];default:return Xe("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function Mf(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let o=/ERROR: 0:(\d+)/.exec(r);if(o){let a=parseInt(o[1]);return t.toUpperCase()+`

`+r+`

`+D_(s.getShaderSource(e),a)}else return r}function U_(s,e){let t=N_(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var F_={[Co]:"Linear",[Po]:"Reinhard",[Io]:"Cineon",[Ns]:"ACESFilmic",[Do]:"AgX",[No]:"Neutral",[Lo]:"Custom"};function O_(s,e){let t=F_[e];return t===void 0?(Xe("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var ec=new z;function B_(){rt.getLuminanceCoefficients(ec);let s=ec.x.toFixed(4),e=ec.y.toFixed(4),t=ec.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function z_(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(qo).join(`
`)}function k_(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function H_(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),o=r.name,a=1;r.type===s.FLOAT_MAT2&&(a=2),r.type===s.FLOAT_MAT3&&(a=3),r.type===s.FLOAT_MAT4&&(a=4),t[o]={type:r.type,location:s.getAttribLocation(e,o),locationSize:a}}return t}function qo(s){return s!==""}function bf(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function Sf(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var V_=/^[ \t]*#include +<([\w\d./]+)>/gm;function Oh(s){return s.replace(V_,W_)}var G_=new Map;function W_(s,e){let t=ct[e];if(t===void 0){let n=G_.get(e);if(n!==void 0)t=ct[n],Xe('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return Oh(t)}var X_=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function Tf(s){return s.replace(X_,q_)}function q_(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function Ef(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var Y_={[Ro]:"SHADOWMAP_TYPE_PCF",[Er]:"SHADOWMAP_TYPE_VSM"};function Z_(s){return Y_[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var K_={[rs]:"ENVMAP_TYPE_CUBE",[Us]:"ENVMAP_TYPE_CUBE",[Uo]:"ENVMAP_TYPE_CUBE_UV"};function j_(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":K_[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var J_={[Us]:"ENVMAP_MODE_REFRACTION"};function $_(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":J_[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var Q_={[ll]:"ENVMAP_BLENDING_MULTIPLY",[Hd]:"ENVMAP_BLENDING_MIX",[Vd]:"ENVMAP_BLENDING_ADD"};function ev(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":Q_[s.combine]||"ENVMAP_BLENDING_NONE"}function tv(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function nv(s,e,t,n){let i=s.getContext(),r=t.defines,o=t.vertexShader,a=t.fragmentShader,l=Z_(t),c=j_(t),h=$_(t),u=ev(t),d=tv(t),f=z_(t),g=k_(r),M=i.createProgram(),m,p,v=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(qo).join(`
`),m.length>0&&(m+=`
`),p=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(qo).join(`
`),p.length>0&&(p+=`
`)):(m=[Ef(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(qo).join(`
`),p=[Ef(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",d?"#define CUBEUV_TEXEL_WIDTH "+d.texelWidth:"",d?"#define CUBEUV_TEXEL_HEIGHT "+d.texelHeight:"",d?"#define CUBEUV_MAX_MIP "+d.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==ti?"#define TONE_MAPPING":"",t.toneMapping!==ti?ct.tonemapping_pars_fragment:"",t.toneMapping!==ti?O_("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",ct.colorspace_pars_fragment,U_("linearToOutputTexel",t.outputColorSpace),B_(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(qo).join(`
`)),o=Oh(o),o=bf(o,t),o=Sf(o,t),a=Oh(a),a=bf(a,t),a=Sf(a,t),o=Tf(o),a=Tf(a),t.isRawShaderMaterial!==!0&&(v=`#version 300 es
`,m=[f,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,p=["#define varying in",t.glslVersion===yh?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===yh?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+p);let S=v+m+o,_=v+p+a,w=vf(i,i.VERTEX_SHADER,S),T=vf(i,i.FRAGMENT_SHADER,_);i.attachShader(M,w),i.attachShader(M,T),t.index0AttributeName!==void 0?i.bindAttribLocation(M,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(M,0,"position"),i.linkProgram(M);function L(R){if(s.debug.checkShaderErrors){let F=i.getProgramInfoLog(M)||"",X=i.getShaderInfoLog(w)||"",P=i.getShaderInfoLog(T)||"",N=F.trim(),O=X.trim(),A=P.trim(),V=!0,Y=!0;if(i.getProgramParameter(M,i.LINK_STATUS)===!1)if(V=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,M,w,T);else{let G=Mf(i,w,"vertex"),ne=Mf(i,T,"fragment");Qe("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(M,i.VALIDATE_STATUS)+`

Material Name: `+R.name+`
Material Type: `+R.type+`

Program Info Log: `+N+`
`+G+`
`+ne)}else N!==""?Xe("WebGLProgram: Program Info Log:",N):(O===""||A==="")&&(Y=!1);Y&&(R.diagnostics={runnable:V,programLog:N,vertexShader:{log:O,prefix:m},fragmentShader:{log:A,prefix:p}})}i.deleteShader(w),i.deleteShader(T),y=new Ir(i,M),I=H_(i,M)}let y;this.getUniforms=function(){return y===void 0&&L(this),y};let I;this.getAttributes=function(){return I===void 0&&L(this),I};let E=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return E===!1&&(E=i.getProgramParameter(M,I_)),E},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(M),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=L_++,this.cacheKey=e,this.usedTimes=1,this.program=M,this.vertexShader=w,this.fragmentShader=T,this}var iv=0,Bh=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new zh(e),t.set(e,n)),n}},zh=class{constructor(e){this.id=iv++,this.code=e,this.usedTimes=0}};function sv(s){return s===as||s===ko||s===Ho}function rv(s,e,t,n,i,r){let o=new hr,a=new Bh,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,d=n.precision,f={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function g(y){return l.add(y),y===0?"uv":`uv${y}`}function M(y,I,E,R,F,X){let P=R.fog,N=F.geometry,O=y.isMeshStandardMaterial||y.isMeshLambertMaterial||y.isMeshPhongMaterial?R.environment:null,A=y.isMeshStandardMaterial||y.isMeshLambertMaterial&&!y.envMap||y.isMeshPhongMaterial&&!y.envMap,V=e.get(y.envMap||O,A),Y=V&&V.mapping===Uo?V.image.height:null,G=f[y.type];y.precision!==null&&(d=n.getMaxPrecision(y.precision),d!==y.precision&&Xe("WebGLProgram.getParameters:",y.precision,"not supported, using",d,"instead."));let ne=N.morphAttributes.position||N.morphAttributes.normal||N.morphAttributes.color,le=ne!==void 0?ne.length:0,Ae=0;N.morphAttributes.position!==void 0&&(Ae=1),N.morphAttributes.normal!==void 0&&(Ae=2),N.morphAttributes.color!==void 0&&(Ae=3);let Pe,Re,oe,pe;if(G){let Ge=vi[G];Pe=Ge.vertexShader,Re=Ge.fragmentShader}else{Pe=y.vertexShader,Re=y.fragmentShader;let Ge=a.getVertexShaderStage(y),zt=a.getFragmentShaderStage(y);a.update(y,Ge,zt),oe=Ge.id,pe=zt.id}let ce=s.getRenderTarget(),Te=s.state.buffers.depth.getReversed(),ue=F.isInstancedMesh===!0,he=F.isBatchedMesh===!0,H=!!y.map,Z=!!y.matcap,W=!!V,J=!!y.aoMap,te=!!y.lightMap,fe=!!y.bumpMap&&y.wireframe===!1,Me=!!y.normalMap,Oe=!!y.displacementMap,We=!!y.emissiveMap,Ye=!!y.metalnessMap,Je=!!y.roughnessMap,q=y.anisotropy>0,Be=y.clearcoat>0,$e=y.dispersion>0,b=y.iridescence>0,x=y.sheen>0,U=y.transmission>0,B=q&&!!y.anisotropyMap,C=Be&&!!y.clearcoatMap,k=Be&&!!y.clearcoatNormalMap,$=Be&&!!y.clearcoatRoughnessMap,j=b&&!!y.iridescenceMap,Q=b&&!!y.iridescenceThicknessMap,me=x&&!!y.sheenColorMap,Ie=x&&!!y.sheenRoughnessMap,be=!!y.specularMap,ye=!!y.specularColorMap,Fe=!!y.specularIntensityMap,we=U&&!!y.transmissionMap,ze=U&&!!y.thicknessMap,K=!!y.gradientMap,_e=!!y.alphaMap,de=y.alphaTest>0,Se=!!y.alphaHash,Ee=!!y.extensions,ge=ti;y.toneMapped&&(ce===null||ce.isXRRenderTarget===!0)&&(ge=s.toneMapping);let Ve={shaderID:G,shaderType:y.type,shaderName:y.name,vertexShader:Pe,fragmentShader:Re,defines:y.defines,customVertexShaderID:oe,customFragmentShaderID:pe,isRawShaderMaterial:y.isRawShaderMaterial===!0,glslVersion:y.glslVersion,precision:d,batching:he,batchingColor:he&&F._colorsTexture!==null,instancing:ue,instancingColor:ue&&F.instanceColor!==null,instancingMorph:ue&&F.morphTexture!==null,outputColorSpace:ce===null?s.outputColorSpace:ce.isXRRenderTarget===!0?ce.texture.colorSpace:rt.workingColorSpace,alphaToCoverage:!!y.alphaToCoverage,map:H,matcap:Z,envMap:W,envMapMode:W&&V.mapping,envMapCubeUVHeight:Y,aoMap:J,lightMap:te,bumpMap:fe,normalMap:Me,displacementMap:Oe,emissiveMap:We,normalMapObjectSpace:Me&&y.normalMapType===Zd,normalMapTangentSpace:Me&&y.normalMapType===Go,packedNormalMap:Me&&y.normalMapType===Go&&sv(y.normalMap.format),metalnessMap:Ye,roughnessMap:Je,anisotropy:q,anisotropyMap:B,clearcoat:Be,clearcoatMap:C,clearcoatNormalMap:k,clearcoatRoughnessMap:$,dispersion:$e,iridescence:b,iridescenceMap:j,iridescenceThicknessMap:Q,sheen:x,sheenColorMap:me,sheenRoughnessMap:Ie,specularMap:be,specularColorMap:ye,specularIntensityMap:Fe,transmission:U,transmissionMap:we,thicknessMap:ze,gradientMap:K,opaque:y.transparent===!1&&y.blending===bs&&y.alphaToCoverage===!1,alphaMap:_e,alphaTest:de,alphaHash:Se,combine:y.combine,mapUv:H&&g(y.map.channel),aoMapUv:J&&g(y.aoMap.channel),lightMapUv:te&&g(y.lightMap.channel),bumpMapUv:fe&&g(y.bumpMap.channel),normalMapUv:Me&&g(y.normalMap.channel),displacementMapUv:Oe&&g(y.displacementMap.channel),emissiveMapUv:We&&g(y.emissiveMap.channel),metalnessMapUv:Ye&&g(y.metalnessMap.channel),roughnessMapUv:Je&&g(y.roughnessMap.channel),anisotropyMapUv:B&&g(y.anisotropyMap.channel),clearcoatMapUv:C&&g(y.clearcoatMap.channel),clearcoatNormalMapUv:k&&g(y.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:$&&g(y.clearcoatRoughnessMap.channel),iridescenceMapUv:j&&g(y.iridescenceMap.channel),iridescenceThicknessMapUv:Q&&g(y.iridescenceThicknessMap.channel),sheenColorMapUv:me&&g(y.sheenColorMap.channel),sheenRoughnessMapUv:Ie&&g(y.sheenRoughnessMap.channel),specularMapUv:be&&g(y.specularMap.channel),specularColorMapUv:ye&&g(y.specularColorMap.channel),specularIntensityMapUv:Fe&&g(y.specularIntensityMap.channel),transmissionMapUv:we&&g(y.transmissionMap.channel),thicknessMapUv:ze&&g(y.thicknessMap.channel),alphaMapUv:_e&&g(y.alphaMap.channel),vertexTangents:!!N.attributes.tangent&&(Me||q),vertexNormals:!!N.attributes.normal,vertexColors:y.vertexColors,vertexAlphas:y.vertexColors===!0&&!!N.attributes.color&&N.attributes.color.itemSize===4,pointsUvs:F.isPoints===!0&&!!N.attributes.uv&&(H||_e),fog:!!P,useFog:y.fog===!0,fogExp2:!!P&&P.isFogExp2,flatShading:y.wireframe===!1&&(y.flatShading===!0||N.attributes.normal===void 0&&Me===!1&&(y.isMeshLambertMaterial||y.isMeshPhongMaterial||y.isMeshStandardMaterial||y.isMeshPhysicalMaterial)),sizeAttenuation:y.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:Te,skinning:F.isSkinnedMesh===!0,hasPositionAttribute:N.attributes.position!==void 0,morphTargets:N.morphAttributes.position!==void 0,morphNormals:N.morphAttributes.normal!==void 0,morphColors:N.morphAttributes.color!==void 0,morphTargetsCount:le,morphTextureStride:Ae,numDirLights:I.directional.length,numPointLights:I.point.length,numSpotLights:I.spot.length,numSpotLightMaps:I.spotLightMap.length,numRectAreaLights:I.rectArea.length,numHemiLights:I.hemi.length,numDirLightShadows:I.directionalShadowMap.length,numPointLightShadows:I.pointShadowMap.length,numSpotLightShadows:I.spotShadowMap.length,numSpotLightShadowsWithMaps:I.numSpotLightShadowsWithMaps,numLightProbes:I.numLightProbes,numLightProbeGrids:X.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:y.dithering,shadowMapEnabled:s.shadowMap.enabled&&E.length>0,shadowMapType:s.shadowMap.type,toneMapping:ge,decodeVideoTexture:H&&y.map.isVideoTexture===!0&&rt.getTransfer(y.map.colorSpace)===yt,decodeVideoTextureEmissive:We&&y.emissiveMap.isVideoTexture===!0&&rt.getTransfer(y.emissiveMap.colorSpace)===yt,premultipliedAlpha:y.premultipliedAlpha,doubleSided:y.side===Nt,flipSided:y.side===tn,useDepthPacking:y.depthPacking>=0,depthPacking:y.depthPacking||0,index0AttributeName:y.index0AttributeName,extensionClipCullDistance:Ee&&y.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(Ee&&y.extensions.multiDraw===!0||he)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:y.customProgramCacheKey()};return Ve.vertexUv1s=l.has(1),Ve.vertexUv2s=l.has(2),Ve.vertexUv3s=l.has(3),l.clear(),Ve}function m(y){let I=[];if(y.shaderID?I.push(y.shaderID):(I.push(y.customVertexShaderID),I.push(y.customFragmentShaderID)),y.defines!==void 0)for(let E in y.defines)I.push(E),I.push(y.defines[E]);return y.isRawShaderMaterial===!1&&(p(I,y),v(I,y),I.push(s.outputColorSpace)),I.push(y.customProgramCacheKey),I.join()}function p(y,I){y.push(I.precision),y.push(I.outputColorSpace),y.push(I.envMapMode),y.push(I.envMapCubeUVHeight),y.push(I.mapUv),y.push(I.alphaMapUv),y.push(I.lightMapUv),y.push(I.aoMapUv),y.push(I.bumpMapUv),y.push(I.normalMapUv),y.push(I.displacementMapUv),y.push(I.emissiveMapUv),y.push(I.metalnessMapUv),y.push(I.roughnessMapUv),y.push(I.anisotropyMapUv),y.push(I.clearcoatMapUv),y.push(I.clearcoatNormalMapUv),y.push(I.clearcoatRoughnessMapUv),y.push(I.iridescenceMapUv),y.push(I.iridescenceThicknessMapUv),y.push(I.sheenColorMapUv),y.push(I.sheenRoughnessMapUv),y.push(I.specularMapUv),y.push(I.specularColorMapUv),y.push(I.specularIntensityMapUv),y.push(I.transmissionMapUv),y.push(I.thicknessMapUv),y.push(I.combine),y.push(I.fogExp2),y.push(I.sizeAttenuation),y.push(I.morphTargetsCount),y.push(I.morphAttributeCount),y.push(I.numDirLights),y.push(I.numPointLights),y.push(I.numSpotLights),y.push(I.numSpotLightMaps),y.push(I.numHemiLights),y.push(I.numRectAreaLights),y.push(I.numDirLightShadows),y.push(I.numPointLightShadows),y.push(I.numSpotLightShadows),y.push(I.numSpotLightShadowsWithMaps),y.push(I.numLightProbes),y.push(I.shadowMapType),y.push(I.toneMapping),y.push(I.numClippingPlanes),y.push(I.numClipIntersection),y.push(I.depthPacking)}function v(y,I){o.disableAll(),I.instancing&&o.enable(0),I.instancingColor&&o.enable(1),I.instancingMorph&&o.enable(2),I.matcap&&o.enable(3),I.envMap&&o.enable(4),I.normalMapObjectSpace&&o.enable(5),I.normalMapTangentSpace&&o.enable(6),I.clearcoat&&o.enable(7),I.iridescence&&o.enable(8),I.alphaTest&&o.enable(9),I.vertexColors&&o.enable(10),I.vertexAlphas&&o.enable(11),I.vertexUv1s&&o.enable(12),I.vertexUv2s&&o.enable(13),I.vertexUv3s&&o.enable(14),I.vertexTangents&&o.enable(15),I.anisotropy&&o.enable(16),I.alphaHash&&o.enable(17),I.batching&&o.enable(18),I.dispersion&&o.enable(19),I.batchingColor&&o.enable(20),I.gradientMap&&o.enable(21),I.packedNormalMap&&o.enable(22),I.vertexNormals&&o.enable(23),y.push(o.mask),o.disableAll(),I.fog&&o.enable(0),I.useFog&&o.enable(1),I.flatShading&&o.enable(2),I.logarithmicDepthBuffer&&o.enable(3),I.reversedDepthBuffer&&o.enable(4),I.skinning&&o.enable(5),I.morphTargets&&o.enable(6),I.morphNormals&&o.enable(7),I.morphColors&&o.enable(8),I.premultipliedAlpha&&o.enable(9),I.shadowMapEnabled&&o.enable(10),I.doubleSided&&o.enable(11),I.flipSided&&o.enable(12),I.useDepthPacking&&o.enable(13),I.dithering&&o.enable(14),I.transmission&&o.enable(15),I.sheen&&o.enable(16),I.opaque&&o.enable(17),I.pointsUvs&&o.enable(18),I.decodeVideoTexture&&o.enable(19),I.decodeVideoTextureEmissive&&o.enable(20),I.alphaToCoverage&&o.enable(21),I.numLightProbeGrids>0&&o.enable(22),I.hasPositionAttribute&&o.enable(23),y.push(o.mask)}function S(y){let I=f[y.type],E;if(I){let R=vi[I];E=si.clone(R.uniforms)}else E=y.uniforms;return E}function _(y,I){let E=h.get(I);return E!==void 0?++E.usedTimes:(E=new nv(s,I,y,i),c.push(E),h.set(I,E)),E}function w(y){if(--y.usedTimes===0){let I=c.indexOf(y);c[I]=c[c.length-1],c.pop(),h.delete(y.cacheKey),y.destroy()}}function T(y){a.remove(y)}function L(){a.dispose()}return{getParameters:M,getProgramCacheKey:m,getUniforms:S,acquireProgram:_,releaseProgram:w,releaseShaderCache:T,programs:c,dispose:L}}function ov(){let s=new WeakMap;function e(o){return s.has(o)}function t(o){let a=s.get(o);return a===void 0&&(a={},s.set(o,a)),a}function n(o){s.delete(o)}function i(o,a,l){s.get(o)[a]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function av(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function wf(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function Af(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function o(d){let f=0;return d.isInstancedMesh&&(f+=2),d.isSkinnedMesh&&(f+=1),f}function a(d,f,g,M,m,p){let v=s[e];return v===void 0?(v={id:d.id,object:d,geometry:f,material:g,materialVariant:o(d),groupOrder:M,renderOrder:d.renderOrder,z:m,group:p},s[e]=v):(v.id=d.id,v.object=d,v.geometry=f,v.material=g,v.materialVariant=o(d),v.groupOrder=M,v.renderOrder=d.renderOrder,v.z=m,v.group=p),e++,v}function l(d,f,g,M,m,p){let v=a(d,f,g,M,m,p);g.transmission>0?n.push(v):g.transparent===!0?i.push(v):t.push(v)}function c(d,f,g,M,m,p){let v=a(d,f,g,M,m,p);g.transmission>0?n.unshift(v):g.transparent===!0?i.unshift(v):t.unshift(v)}function h(d,f,g){t.length>1&&t.sort(d||av),n.length>1&&n.sort(f||wf),i.length>1&&i.sort(f||wf),g&&(t.reverse(),n.reverse(),i.reverse())}function u(){for(let d=e,f=s.length;d<f;d++){let g=s[d];if(g.id===null)break;g.id=null,g.object=null,g.geometry=null,g.material=null,g.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function lv(){let s=new WeakMap;function e(n,i){let r=s.get(n),o;return r===void 0?(o=new Af,s.set(n,[o])):i>=r.length?(o=new Af,r.push(o)):o=r[i],o}function t(){s=new WeakMap}return{get:e,dispose:t}}function cv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={direction:new z,color:new xe};break;case"SpotLight":t={position:new z,direction:new z,color:new xe,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new z,color:new xe,distance:0,decay:0};break;case"HemisphereLight":t={direction:new z,skyColor:new xe,groundColor:new xe};break;case"RectAreaLight":t={color:new xe,position:new z,halfWidth:new z,halfHeight:new z};break}return s[e.id]=t,t}}}function hv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ve};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ve};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new ve,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var uv=0;function dv(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function fv(s){let e=new cv,t=hv(),n={version:0,hash:{directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new z);let i=new z,r=new et,o=new et;function a(c){let h=0,u=0,d=0;for(let I=0;I<9;I++)n.probe[I].set(0,0,0);let f=0,g=0,M=0,m=0,p=0,v=0,S=0,_=0,w=0,T=0,L=0;c.sort(dv);for(let I=0,E=c.length;I<E;I++){let R=c[I],F=R.color,X=R.intensity,P=R.distance,N=null;if(R.shadow&&R.shadow.map&&(R.shadow.map.texture.format===as?N=R.shadow.map.texture:N=R.shadow.map.depthTexture||R.shadow.map.texture),R.isAmbientLight)h+=F.r*X,u+=F.g*X,d+=F.b*X;else if(R.isLightProbe){for(let O=0;O<9;O++)n.probe[O].addScaledVector(R.sh.coefficients[O],X);L++}else if(R.isDirectionalLight){let O=e.get(R);if(O.color.copy(R.color).multiplyScalar(R.intensity),R.castShadow){let A=R.shadow,V=t.get(R);V.shadowIntensity=A.intensity,V.shadowBias=A.bias,V.shadowNormalBias=A.normalBias,V.shadowRadius=A.radius,V.shadowMapSize=A.mapSize,n.directionalShadow[f]=V,n.directionalShadowMap[f]=N,n.directionalShadowMatrix[f]=R.shadow.matrix,v++}n.directional[f]=O,f++}else if(R.isSpotLight){let O=e.get(R);O.position.setFromMatrixPosition(R.matrixWorld),O.color.copy(F).multiplyScalar(X),O.distance=P,O.coneCos=Math.cos(R.angle),O.penumbraCos=Math.cos(R.angle*(1-R.penumbra)),O.decay=R.decay,n.spot[M]=O;let A=R.shadow;if(R.map&&(n.spotLightMap[w]=R.map,w++,A.updateMatrices(R),R.castShadow&&T++),n.spotLightMatrix[M]=A.matrix,R.castShadow){let V=t.get(R);V.shadowIntensity=A.intensity,V.shadowBias=A.bias,V.shadowNormalBias=A.normalBias,V.shadowRadius=A.radius,V.shadowMapSize=A.mapSize,n.spotShadow[M]=V,n.spotShadowMap[M]=N,_++}M++}else if(R.isRectAreaLight){let O=e.get(R);O.color.copy(F).multiplyScalar(X),O.halfWidth.set(R.width*.5,0,0),O.halfHeight.set(0,R.height*.5,0),n.rectArea[m]=O,m++}else if(R.isPointLight){let O=e.get(R);if(O.color.copy(R.color).multiplyScalar(R.intensity),O.distance=R.distance,O.decay=R.decay,R.castShadow){let A=R.shadow,V=t.get(R);V.shadowIntensity=A.intensity,V.shadowBias=A.bias,V.shadowNormalBias=A.normalBias,V.shadowRadius=A.radius,V.shadowMapSize=A.mapSize,V.shadowCameraNear=A.camera.near,V.shadowCameraFar=A.camera.far,n.pointShadow[g]=V,n.pointShadowMap[g]=N,n.pointShadowMatrix[g]=R.shadow.matrix,S++}n.point[g]=O,g++}else if(R.isHemisphereLight){let O=e.get(R);O.skyColor.copy(R.color).multiplyScalar(X),O.groundColor.copy(R.groundColor).multiplyScalar(X),n.hemi[p]=O,p++}}m>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Ce.LTC_FLOAT_1,n.rectAreaLTC2=Ce.LTC_FLOAT_2):(n.rectAreaLTC1=Ce.LTC_HALF_1,n.rectAreaLTC2=Ce.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=d;let y=n.hash;(y.directionalLength!==f||y.pointLength!==g||y.spotLength!==M||y.rectAreaLength!==m||y.hemiLength!==p||y.numDirectionalShadows!==v||y.numPointShadows!==S||y.numSpotShadows!==_||y.numSpotMaps!==w||y.numLightProbes!==L)&&(n.directional.length=f,n.spot.length=M,n.rectArea.length=m,n.point.length=g,n.hemi.length=p,n.directionalShadow.length=v,n.directionalShadowMap.length=v,n.pointShadow.length=S,n.pointShadowMap.length=S,n.spotShadow.length=_,n.spotShadowMap.length=_,n.directionalShadowMatrix.length=v,n.pointShadowMatrix.length=S,n.spotLightMatrix.length=_+w-T,n.spotLightMap.length=w,n.numSpotLightShadowsWithMaps=T,n.numLightProbes=L,y.directionalLength=f,y.pointLength=g,y.spotLength=M,y.rectAreaLength=m,y.hemiLength=p,y.numDirectionalShadows=v,y.numPointShadows=S,y.numSpotShadows=_,y.numSpotMaps=w,y.numLightProbes=L,n.version=uv++)}function l(c,h){let u=0,d=0,f=0,g=0,M=0,m=h.matrixWorldInverse;for(let p=0,v=c.length;p<v;p++){let S=c[p];if(S.isDirectionalLight){let _=n.directional[u];_.direction.setFromMatrixPosition(S.matrixWorld),i.setFromMatrixPosition(S.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(m),u++}else if(S.isSpotLight){let _=n.spot[f];_.position.setFromMatrixPosition(S.matrixWorld),_.position.applyMatrix4(m),_.direction.setFromMatrixPosition(S.matrixWorld),i.setFromMatrixPosition(S.target.matrixWorld),_.direction.sub(i),_.direction.transformDirection(m),f++}else if(S.isRectAreaLight){let _=n.rectArea[g];_.position.setFromMatrixPosition(S.matrixWorld),_.position.applyMatrix4(m),o.identity(),r.copy(S.matrixWorld),r.premultiply(m),o.extractRotation(r),_.halfWidth.set(S.width*.5,0,0),_.halfHeight.set(0,S.height*.5,0),_.halfWidth.applyMatrix4(o),_.halfHeight.applyMatrix4(o),g++}else if(S.isPointLight){let _=n.point[d];_.position.setFromMatrixPosition(S.matrixWorld),_.position.applyMatrix4(m),d++}else if(S.isHemisphereLight){let _=n.hemi[M];_.direction.setFromMatrixPosition(S.matrixWorld),_.direction.transformDirection(m),M++}}}return{setup:a,setupView:l,state:n}}function Rf(s){let e=new fv(s),t=[],n=[],i=[];function r(d){u.camera=d,t.length=0,n.length=0,i.length=0}function o(d){t.push(d)}function a(d){n.push(d)}function l(d){i.push(d)}function c(){e.setup(t)}function h(d){e.setupView(t,d)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:o,pushShadow:a,pushLightProbeGrid:l}}function pv(s){let e=new WeakMap;function t(i,r=0){let o=e.get(i),a;return o===void 0?(a=new Rf(s),e.set(i,[a])):r>=o.length?(a=new Rf(s),o.push(a)):a=o[r],a}function n(){e=new WeakMap}return{get:t,dispose:n}}var mv=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,gv=`uniform sampler2D shadow_pass;
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
}`,xv=[new z(1,0,0),new z(-1,0,0),new z(0,1,0),new z(0,-1,0),new z(0,0,1),new z(0,0,-1)],_v=[new z(0,-1,0),new z(0,-1,0),new z(0,0,1),new z(0,0,-1),new z(0,-1,0),new z(0,-1,0)],Cf=new et,Xo=new z,Lh=new z;function vv(s,e,t){let n=new gr,i=new ve,r=new ve,o=new Mt,a=new Ka,l=new ja,c={},h=t.maxTextureSize,u={[In]:tn,[tn]:In,[Nt]:Nt},d=new mt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new ve},radius:{value:4}},vertexShader:mv,fragmentShader:gv}),f=d.clone();f.defines.HORIZONTAL_PASS=1;let g=new pt;g.setAttribute("position",new xt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let M=new je(g,d),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=Ro;let p=this.type;this.render=function(T,L,y){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||T.length===0)return;this.type===al&&(Xe("WebGLShadowMap: PCFSoftShadowMap has been deprecated. Using PCFShadowMap instead."),this.type=Ro);let I=s.getRenderTarget(),E=s.getActiveCubeFace(),R=s.getActiveMipmapLevel(),F=s.state;F.setBlending(Wn),F.buffers.depth.getReversed()===!0?F.buffers.color.setClear(0,0,0,0):F.buffers.color.setClear(1,1,1,1),F.buffers.depth.setTest(!0),F.setScissorTest(!1);let X=p!==this.type;X&&L.traverse(function(P){P.material&&(Array.isArray(P.material)?P.material.forEach(N=>N.needsUpdate=!0):P.material.needsUpdate=!0)});for(let P=0,N=T.length;P<N;P++){let O=T[P],A=O.shadow;if(A===void 0){Xe("WebGLShadowMap:",O,"has no shadow.");continue}if(A.autoUpdate===!1&&A.needsUpdate===!1)continue;i.copy(A.mapSize);let V=A.getFrameExtents();i.multiply(V),r.copy(A.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/V.x),i.x=r.x*V.x,A.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/V.y),i.y=r.y*V.y,A.mapSize.y=r.y));let Y=s.state.buffers.depth.getReversed();if(A.camera._reversedDepth=Y,A.map===null||X===!0){if(A.map!==null&&(A.map.depthTexture!==null&&(A.map.depthTexture.dispose(),A.map.depthTexture=null),A.map.dispose()),this.type===Er){if(O.isPointLight){Xe("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}A.map=new Wt(i.x,i.y,{format:as,type:nn,minFilter:Vt,magFilter:Vt,generateMipmaps:!1}),A.map.texture.name=O.name+".shadowMap",A.map.depthTexture=new Li(i.x,i.y,Nn),A.map.depthTexture.name=O.name+".shadowMapDepth",A.map.depthTexture.format=hi,A.map.depthTexture.compareFunction=null,A.map.depthTexture.minFilter=Kt,A.map.depthTexture.magFilter=Kt}else O.isPointLight?(A.map=new tc(i.x),A.map.depthTexture=new Ha(i.x,ii)):(A.map=new Wt(i.x,i.y),A.map.depthTexture=new Li(i.x,i.y,ii)),A.map.depthTexture.name=O.name+".shadowMap",A.map.depthTexture.format=hi,this.type===Ro?(A.map.depthTexture.compareFunction=Y?$l:Jl,A.map.depthTexture.minFilter=Vt,A.map.depthTexture.magFilter=Vt):(A.map.depthTexture.compareFunction=null,A.map.depthTexture.minFilter=Kt,A.map.depthTexture.magFilter=Kt);A.camera.updateProjectionMatrix()}let G=A.map.isWebGLCubeRenderTarget?6:1;for(let ne=0;ne<G;ne++){if(A.map.isWebGLCubeRenderTarget)s.setRenderTarget(A.map,ne),s.clear();else{ne===0&&(s.setRenderTarget(A.map),s.clear());let le=A.getViewport(ne);o.set(r.x*le.x,r.y*le.y,r.x*le.z,r.y*le.w),F.viewport(o)}if(O.isPointLight){let le=A.camera,Ae=A.matrix,Pe=O.distance||le.far;Pe!==le.far&&(le.far=Pe,le.updateProjectionMatrix()),Xo.setFromMatrixPosition(O.matrixWorld),le.position.copy(Xo),Lh.copy(le.position),Lh.add(xv[ne]),le.up.copy(_v[ne]),le.lookAt(Lh),le.updateMatrixWorld(),Ae.makeTranslation(-Xo.x,-Xo.y,-Xo.z),Cf.multiplyMatrices(le.projectionMatrix,le.matrixWorldInverse),A._frustum.setFromProjectionMatrix(Cf,le.coordinateSystem,le.reversedDepth)}else A.updateMatrices(O);n=A.getFrustum(),_(L,y,A.camera,O,this.type)}A.isPointLightShadow!==!0&&this.type===Er&&v(A,y),A.needsUpdate=!1}p=this.type,m.needsUpdate=!1,s.setRenderTarget(I,E,R)};function v(T,L){let y=e.update(M);d.defines.VSM_SAMPLES!==T.blurSamples&&(d.defines.VSM_SAMPLES=T.blurSamples,f.defines.VSM_SAMPLES=T.blurSamples,d.needsUpdate=!0,f.needsUpdate=!0),T.mapPass===null&&(T.mapPass=new Wt(i.x,i.y,{format:as,type:nn})),d.uniforms.shadow_pass.value=T.map.depthTexture,d.uniforms.resolution.value=T.mapSize,d.uniforms.radius.value=T.radius,s.setRenderTarget(T.mapPass),s.clear(),s.renderBufferDirect(L,null,y,d,M,null),f.uniforms.shadow_pass.value=T.mapPass.texture,f.uniforms.resolution.value=T.mapSize,f.uniforms.radius.value=T.radius,s.setRenderTarget(T.map),s.clear(),s.renderBufferDirect(L,null,y,f,M,null)}function S(T,L,y,I){let E=null,R=y.isPointLight===!0?T.customDistanceMaterial:T.customDepthMaterial;if(R!==void 0)E=R;else if(E=y.isPointLight===!0?l:a,s.localClippingEnabled&&L.clipShadows===!0&&Array.isArray(L.clippingPlanes)&&L.clippingPlanes.length!==0||L.displacementMap&&L.displacementScale!==0||L.alphaMap&&L.alphaTest>0||L.map&&L.alphaTest>0||L.alphaToCoverage===!0){let F=E.uuid,X=L.uuid,P=c[F];P===void 0&&(P={},c[F]=P);let N=P[X];N===void 0&&(N=E.clone(),P[X]=N,L.addEventListener("dispose",w)),E=N}if(E.visible=L.visible,E.wireframe=L.wireframe,I===Er?E.side=L.shadowSide!==null?L.shadowSide:L.side:E.side=L.shadowSide!==null?L.shadowSide:u[L.side],E.alphaMap=L.alphaMap,E.alphaTest=L.alphaToCoverage===!0?.5:L.alphaTest,E.map=L.map,E.clipShadows=L.clipShadows,E.clippingPlanes=L.clippingPlanes,E.clipIntersection=L.clipIntersection,E.displacementMap=L.displacementMap,E.displacementScale=L.displacementScale,E.displacementBias=L.displacementBias,E.wireframeLinewidth=L.wireframeLinewidth,E.linewidth=L.linewidth,y.isPointLight===!0&&E.isMeshDistanceMaterial===!0){let F=s.properties.get(E);F.light=y}return E}function _(T,L,y,I,E){if(T.visible===!1)return;if(T.layers.test(L.layers)&&(T.isMesh||T.isLine||T.isPoints)&&(T.castShadow||T.receiveShadow&&E===Er)&&(!T.frustumCulled||n.intersectsObject(T))){T.modelViewMatrix.multiplyMatrices(y.matrixWorldInverse,T.matrixWorld);let X=e.update(T),P=T.material;if(Array.isArray(P)){let N=X.groups;for(let O=0,A=N.length;O<A;O++){let V=N[O],Y=P[V.materialIndex];if(Y&&Y.visible){let G=S(T,Y,I,E);T.onBeforeShadow(s,T,L,y,X,G,V),s.renderBufferDirect(y,null,X,G,T,V),T.onAfterShadow(s,T,L,y,X,G,V)}}}else if(P.visible){let N=S(T,P,I,E);T.onBeforeShadow(s,T,L,y,X,N,null),s.renderBufferDirect(y,null,X,N,T,null),T.onAfterShadow(s,T,L,y,X,N,null)}}let F=T.children;for(let X=0,P=F.length;X<P;X++)_(F[X],L,y,I,E)}function w(T){T.target.removeEventListener("dispose",w);for(let y in c){let I=c[y],E=T.target.uuid;E in I&&(I[E].dispose(),delete I[E])}}}function yv(s,e){function t(){let K=!1,_e=new Mt,de=null,Se=new Mt(0,0,0,0);return{setMask:function(Ee){de!==Ee&&!K&&(s.colorMask(Ee,Ee,Ee,Ee),de=Ee)},setLocked:function(Ee){K=Ee},setClear:function(Ee,ge,Ve,Ge,zt){zt===!0&&(Ee*=Ge,ge*=Ge,Ve*=Ge),_e.set(Ee,ge,Ve,Ge),Se.equals(_e)===!1&&(s.clearColor(Ee,ge,Ve,Ge),Se.copy(_e))},reset:function(){K=!1,de=null,Se.set(-1,0,0,0)}}}function n(){let K=!1,_e=!1,de=null,Se=null,Ee=null;return{setReversed:function(ge){if(_e!==ge){let Ve=e.get("EXT_clip_control");ge?Ve.clipControlEXT(Ve.LOWER_LEFT_EXT,Ve.ZERO_TO_ONE_EXT):Ve.clipControlEXT(Ve.LOWER_LEFT_EXT,Ve.NEGATIVE_ONE_TO_ONE_EXT),_e=ge;let Ge=Ee;Ee=null,this.setClear(Ge)}},getReversed:function(){return _e},setTest:function(ge){ge?ce(s.DEPTH_TEST):Te(s.DEPTH_TEST)},setMask:function(ge){de!==ge&&!K&&(s.depthMask(ge),de=ge)},setFunc:function(ge){if(_e&&(ge=rf[ge]),Se!==ge){switch(ge){case Ra:s.depthFunc(s.NEVER);break;case Ca:s.depthFunc(s.ALWAYS);break;case Pa:s.depthFunc(s.LESS);break;case Ss:s.depthFunc(s.LEQUAL);break;case Ia:s.depthFunc(s.EQUAL);break;case La:s.depthFunc(s.GEQUAL);break;case Da:s.depthFunc(s.GREATER);break;case Na:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}Se=ge}},setLocked:function(ge){K=ge},setClear:function(ge){Ee!==ge&&(Ee=ge,_e&&(ge=1-ge),s.clearDepth(ge))},reset:function(){K=!1,de=null,Se=null,Ee=null,_e=!1}}}function i(){let K=!1,_e=null,de=null,Se=null,Ee=null,ge=null,Ve=null,Ge=null,zt=null;return{setTest:function(Tt){K||(Tt?ce(s.STENCIL_TEST):Te(s.STENCIL_TEST))},setMask:function(Tt){_e!==Tt&&!K&&(s.stencilMask(Tt),_e=Tt)},setFunc:function(Tt,An,On){(de!==Tt||Se!==An||Ee!==On)&&(s.stencilFunc(Tt,An,On),de=Tt,Se=An,Ee=On)},setOp:function(Tt,An,On){(ge!==Tt||Ve!==An||Ge!==On)&&(s.stencilOp(Tt,An,On),ge=Tt,Ve=An,Ge=On)},setLocked:function(Tt){K=Tt},setClear:function(Tt){zt!==Tt&&(s.clearStencil(Tt),zt=Tt)},reset:function(){K=!1,_e=null,de=null,Se=null,Ee=null,ge=null,Ve=null,Ge=null,zt=null}}}let r=new t,o=new n,a=new i,l=new WeakMap,c=new WeakMap,h={},u={},d={},f=new WeakMap,g=[],M=null,m=!1,p=null,v=null,S=null,_=null,w=null,T=null,L=null,y=new xe(0,0,0),I=0,E=!1,R=null,F=null,X=null,P=null,N=null,O=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),A=!1,V=0,Y=s.getParameter(s.VERSION);Y.indexOf("WebGL")!==-1?(V=parseFloat(/^WebGL (\d)/.exec(Y)[1]),A=V>=1):Y.indexOf("OpenGL ES")!==-1&&(V=parseFloat(/^OpenGL ES (\d)/.exec(Y)[1]),A=V>=2);let G=null,ne={},le=s.getParameter(s.SCISSOR_BOX),Ae=s.getParameter(s.VIEWPORT),Pe=new Mt().fromArray(le),Re=new Mt().fromArray(Ae);function oe(K,_e,de,Se){let Ee=new Uint8Array(4),ge=s.createTexture();s.bindTexture(K,ge),s.texParameteri(K,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(K,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let Ve=0;Ve<de;Ve++)K===s.TEXTURE_3D||K===s.TEXTURE_2D_ARRAY?s.texImage3D(_e,0,s.RGBA,1,1,Se,0,s.RGBA,s.UNSIGNED_BYTE,Ee):s.texImage2D(_e+Ve,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,Ee);return ge}let pe={};pe[s.TEXTURE_2D]=oe(s.TEXTURE_2D,s.TEXTURE_2D,1),pe[s.TEXTURE_CUBE_MAP]=oe(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),pe[s.TEXTURE_2D_ARRAY]=oe(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),pe[s.TEXTURE_3D]=oe(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),o.setClear(1),a.setClear(0),ce(s.DEPTH_TEST),o.setFunc(Ss),fe(!1),Me(ch),ce(s.CULL_FACE),J(Wn);function ce(K){h[K]!==!0&&(s.enable(K),h[K]=!0)}function Te(K){h[K]!==!1&&(s.disable(K),h[K]=!1)}function ue(K,_e){return d[K]!==_e?(s.bindFramebuffer(K,_e),d[K]=_e,K===s.DRAW_FRAMEBUFFER&&(d[s.FRAMEBUFFER]=_e),K===s.FRAMEBUFFER&&(d[s.DRAW_FRAMEBUFFER]=_e),!0):!1}function he(K,_e){let de=g,Se=!1;if(K){de=f.get(_e),de===void 0&&(de=[],f.set(_e,de));let Ee=K.textures;if(de.length!==Ee.length||de[0]!==s.COLOR_ATTACHMENT0){for(let ge=0,Ve=Ee.length;ge<Ve;ge++)de[ge]=s.COLOR_ATTACHMENT0+ge;de.length=Ee.length,Se=!0}}else de[0]!==s.BACK&&(de[0]=s.BACK,Se=!0);Se&&s.drawBuffers(de)}function H(K){return M!==K?(s.useProgram(K),M=K,!0):!1}let Z={[Ji]:s.FUNC_ADD,[Td]:s.FUNC_SUBTRACT,[Ed]:s.FUNC_REVERSE_SUBTRACT};Z[wd]=s.MIN,Z[Ad]=s.MAX;let W={[Rd]:s.ZERO,[Cd]:s.ONE,[Pd]:s.SRC_COLOR,[wa]:s.SRC_ALPHA,[Fd]:s.SRC_ALPHA_SATURATE,[Nd]:s.DST_COLOR,[Ld]:s.DST_ALPHA,[Id]:s.ONE_MINUS_SRC_COLOR,[Aa]:s.ONE_MINUS_SRC_ALPHA,[Ud]:s.ONE_MINUS_DST_COLOR,[Dd]:s.ONE_MINUS_DST_ALPHA,[Od]:s.CONSTANT_COLOR,[Bd]:s.ONE_MINUS_CONSTANT_COLOR,[zd]:s.CONSTANT_ALPHA,[kd]:s.ONE_MINUS_CONSTANT_ALPHA};function J(K,_e,de,Se,Ee,ge,Ve,Ge,zt,Tt){if(K===Wn){m===!0&&(Te(s.BLEND),m=!1);return}if(m===!1&&(ce(s.BLEND),m=!0),K!==Sd){if(K!==p||Tt!==E){if((v!==Ji||w!==Ji)&&(s.blendEquation(s.FUNC_ADD),v=Ji,w=Ji),Tt)switch(K){case bs:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case Xt:s.blendFunc(s.ONE,s.ONE);break;case hh:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case uh:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:Qe("WebGLState: Invalid blending: ",K);break}else switch(K){case bs:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case Xt:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case hh:Qe("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case uh:Qe("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:Qe("WebGLState: Invalid blending: ",K);break}S=null,_=null,T=null,L=null,y.set(0,0,0),I=0,p=K,E=Tt}return}Ee=Ee||_e,ge=ge||de,Ve=Ve||Se,(_e!==v||Ee!==w)&&(s.blendEquationSeparate(Z[_e],Z[Ee]),v=_e,w=Ee),(de!==S||Se!==_||ge!==T||Ve!==L)&&(s.blendFuncSeparate(W[de],W[Se],W[ge],W[Ve]),S=de,_=Se,T=ge,L=Ve),(Ge.equals(y)===!1||zt!==I)&&(s.blendColor(Ge.r,Ge.g,Ge.b,zt),y.copy(Ge),I=zt),p=K,E=!1}function te(K,_e){K.side===Nt?Te(s.CULL_FACE):ce(s.CULL_FACE);let de=K.side===tn;_e&&(de=!de),fe(de),K.blending===bs&&K.transparent===!1?J(Wn):J(K.blending,K.blendEquation,K.blendSrc,K.blendDst,K.blendEquationAlpha,K.blendSrcAlpha,K.blendDstAlpha,K.blendColor,K.blendAlpha,K.premultipliedAlpha),o.setFunc(K.depthFunc),o.setTest(K.depthTest),o.setMask(K.depthWrite),r.setMask(K.colorWrite);let Se=K.stencilWrite;a.setTest(Se),Se&&(a.setMask(K.stencilWriteMask),a.setFunc(K.stencilFunc,K.stencilRef,K.stencilFuncMask),a.setOp(K.stencilFail,K.stencilZFail,K.stencilZPass)),We(K.polygonOffset,K.polygonOffsetFactor,K.polygonOffsetUnits),K.alphaToCoverage===!0?ce(s.SAMPLE_ALPHA_TO_COVERAGE):Te(s.SAMPLE_ALPHA_TO_COVERAGE)}function fe(K){R!==K&&(K?s.frontFace(s.CW):s.frontFace(s.CCW),R=K)}function Me(K){K!==Md?(ce(s.CULL_FACE),K!==F&&(K===ch?s.cullFace(s.BACK):K===bd?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):Te(s.CULL_FACE),F=K}function Oe(K){K!==X&&(A&&s.lineWidth(K),X=K)}function We(K,_e,de){K?(ce(s.POLYGON_OFFSET_FILL),(P!==_e||N!==de)&&(P=_e,N=de,o.getReversed()&&(_e=-_e),s.polygonOffset(_e,de))):Te(s.POLYGON_OFFSET_FILL)}function Ye(K){K?ce(s.SCISSOR_TEST):Te(s.SCISSOR_TEST)}function Je(K){K===void 0&&(K=s.TEXTURE0+O-1),G!==K&&(s.activeTexture(K),G=K)}function q(K,_e,de){de===void 0&&(G===null?de=s.TEXTURE0+O-1:de=G);let Se=ne[de];Se===void 0&&(Se={type:void 0,texture:void 0},ne[de]=Se),(Se.type!==K||Se.texture!==_e)&&(G!==de&&(s.activeTexture(de),G=de),s.bindTexture(K,_e||pe[K]),Se.type=K,Se.texture=_e)}function Be(){let K=ne[G];K!==void 0&&K.type!==void 0&&(s.bindTexture(K.type,null),K.type=void 0,K.texture=void 0)}function $e(){try{s.compressedTexImage2D(...arguments)}catch(K){Qe("WebGLState:",K)}}function b(){try{s.compressedTexImage3D(...arguments)}catch(K){Qe("WebGLState:",K)}}function x(){try{s.texSubImage2D(...arguments)}catch(K){Qe("WebGLState:",K)}}function U(){try{s.texSubImage3D(...arguments)}catch(K){Qe("WebGLState:",K)}}function B(){try{s.compressedTexSubImage2D(...arguments)}catch(K){Qe("WebGLState:",K)}}function C(){try{s.compressedTexSubImage3D(...arguments)}catch(K){Qe("WebGLState:",K)}}function k(){try{s.texStorage2D(...arguments)}catch(K){Qe("WebGLState:",K)}}function $(){try{s.texStorage3D(...arguments)}catch(K){Qe("WebGLState:",K)}}function j(){try{s.texImage2D(...arguments)}catch(K){Qe("WebGLState:",K)}}function Q(){try{s.texImage3D(...arguments)}catch(K){Qe("WebGLState:",K)}}function me(K){return u[K]!==void 0?u[K]:s.getParameter(K)}function Ie(K,_e){u[K]!==_e&&(s.pixelStorei(K,_e),u[K]=_e)}function be(K){Pe.equals(K)===!1&&(s.scissor(K.x,K.y,K.z,K.w),Pe.copy(K))}function ye(K){Re.equals(K)===!1&&(s.viewport(K.x,K.y,K.z,K.w),Re.copy(K))}function Fe(K,_e){let de=c.get(_e);de===void 0&&(de=new WeakMap,c.set(_e,de));let Se=de.get(K);Se===void 0&&(Se=s.getUniformBlockIndex(_e,K.name),de.set(K,Se))}function we(K,_e){let Se=c.get(_e).get(K);l.get(_e)!==Se&&(s.uniformBlockBinding(_e,Se,K.__bindingPointIndex),l.set(_e,Se))}function ze(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),o.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},G=null,ne={},d={},f=new WeakMap,g=[],M=null,m=!1,p=null,v=null,S=null,_=null,w=null,T=null,L=null,y=new xe(0,0,0),I=0,E=!1,R=null,F=null,X=null,P=null,N=null,Pe.set(0,0,s.canvas.width,s.canvas.height),Re.set(0,0,s.canvas.width,s.canvas.height),r.reset(),o.reset(),a.reset()}return{buffers:{color:r,depth:o,stencil:a},enable:ce,disable:Te,bindFramebuffer:ue,drawBuffers:he,useProgram:H,setBlending:J,setMaterial:te,setFlipSided:fe,setCullFace:Me,setLineWidth:Oe,setPolygonOffset:We,setScissorTest:Ye,activeTexture:Je,bindTexture:q,unbindTexture:Be,compressedTexImage2D:$e,compressedTexImage3D:b,texImage2D:j,texImage3D:Q,pixelStorei:Ie,getParameter:me,updateUBOMapping:Fe,uniformBlockBinding:we,texStorage2D:k,texStorage3D:$,texSubImage2D:x,texSubImage3D:U,compressedTexSubImage2D:B,compressedTexSubImage3D:C,scissor:be,viewport:ye,reset:ze}}function Mv(s,e,t,n,i,r,o){let a=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new ve,h=new WeakMap,u=new Set,d,f=new WeakMap,g=!1;try{g=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function M(b,x){return g?new OffscreenCanvas(b,x):ar("canvas")}function m(b,x,U){let B=1,C=$e(b);if((C.width>U||C.height>U)&&(B=U/Math.max(C.width,C.height)),B<1)if(typeof HTMLImageElement<"u"&&b instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&b instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&b instanceof ImageBitmap||typeof VideoFrame<"u"&&b instanceof VideoFrame){let k=Math.floor(B*C.width),$=Math.floor(B*C.height);d===void 0&&(d=M(k,$));let j=x?M(k,$):d;return j.width=k,j.height=$,j.getContext("2d").drawImage(b,0,0,k,$),Xe("WebGLRenderer: Texture has been resized from ("+C.width+"x"+C.height+") to ("+k+"x"+$+")."),j}else return"data"in b&&Xe("WebGLRenderer: Image in DataTexture is too big ("+C.width+"x"+C.height+")."),b;return b}function p(b){return b.generateMipmaps}function v(b){s.generateMipmap(b)}function S(b){return b.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:b.isWebGL3DRenderTarget?s.TEXTURE_3D:b.isWebGLArrayRenderTarget||b.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function _(b,x,U,B,C,k=!1){if(b!==null){if(s[b]!==void 0)return s[b];Xe("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+b+"'")}let $;B&&($=e.get("EXT_texture_norm16"),$||Xe("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let j=x;if(x===s.RED&&(U===s.FLOAT&&(j=s.R32F),U===s.HALF_FLOAT&&(j=s.R16F),U===s.UNSIGNED_BYTE&&(j=s.R8),U===s.UNSIGNED_SHORT&&$&&(j=$.R16_EXT),U===s.SHORT&&$&&(j=$.R16_SNORM_EXT)),x===s.RED_INTEGER&&(U===s.UNSIGNED_BYTE&&(j=s.R8UI),U===s.UNSIGNED_SHORT&&(j=s.R16UI),U===s.UNSIGNED_INT&&(j=s.R32UI),U===s.BYTE&&(j=s.R8I),U===s.SHORT&&(j=s.R16I),U===s.INT&&(j=s.R32I)),x===s.RG&&(U===s.FLOAT&&(j=s.RG32F),U===s.HALF_FLOAT&&(j=s.RG16F),U===s.UNSIGNED_BYTE&&(j=s.RG8),U===s.UNSIGNED_SHORT&&$&&(j=$.RG16_EXT),U===s.SHORT&&$&&(j=$.RG16_SNORM_EXT)),x===s.RG_INTEGER&&(U===s.UNSIGNED_BYTE&&(j=s.RG8UI),U===s.UNSIGNED_SHORT&&(j=s.RG16UI),U===s.UNSIGNED_INT&&(j=s.RG32UI),U===s.BYTE&&(j=s.RG8I),U===s.SHORT&&(j=s.RG16I),U===s.INT&&(j=s.RG32I)),x===s.RGB_INTEGER&&(U===s.UNSIGNED_BYTE&&(j=s.RGB8UI),U===s.UNSIGNED_SHORT&&(j=s.RGB16UI),U===s.UNSIGNED_INT&&(j=s.RGB32UI),U===s.BYTE&&(j=s.RGB8I),U===s.SHORT&&(j=s.RGB16I),U===s.INT&&(j=s.RGB32I)),x===s.RGBA_INTEGER&&(U===s.UNSIGNED_BYTE&&(j=s.RGBA8UI),U===s.UNSIGNED_SHORT&&(j=s.RGBA16UI),U===s.UNSIGNED_INT&&(j=s.RGBA32UI),U===s.BYTE&&(j=s.RGBA8I),U===s.SHORT&&(j=s.RGBA16I),U===s.INT&&(j=s.RGBA32I)),x===s.RGB&&(U===s.UNSIGNED_SHORT&&$&&(j=$.RGB16_EXT),U===s.SHORT&&$&&(j=$.RGB16_SNORM_EXT),U===s.UNSIGNED_INT_5_9_9_9_REV&&(j=s.RGB9_E5),U===s.UNSIGNED_INT_10F_11F_11F_REV&&(j=s.R11F_G11F_B10F)),x===s.RGBA){let Q=k?Qr:rt.getTransfer(C);U===s.FLOAT&&(j=s.RGBA32F),U===s.HALF_FLOAT&&(j=s.RGBA16F),U===s.UNSIGNED_BYTE&&(j=Q===yt?s.SRGB8_ALPHA8:s.RGBA8),U===s.UNSIGNED_SHORT&&$&&(j=$.RGBA16_EXT),U===s.SHORT&&$&&(j=$.RGBA16_SNORM_EXT),U===s.UNSIGNED_SHORT_4_4_4_4&&(j=s.RGBA4),U===s.UNSIGNED_SHORT_5_5_5_1&&(j=s.RGB5_A1)}return(j===s.R16F||j===s.R32F||j===s.RG16F||j===s.RG32F||j===s.RGBA16F||j===s.RGBA32F)&&e.get("EXT_color_buffer_float"),j}function w(b,x){let U;return b?x===null||x===ii||x===Rr?U=s.DEPTH24_STENCIL8:x===Nn?U=s.DEPTH32F_STENCIL8:x===Ar&&(U=s.DEPTH24_STENCIL8,Xe("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):x===null||x===ii||x===Rr?U=s.DEPTH_COMPONENT24:x===Nn?U=s.DEPTH_COMPONENT32F:x===Ar&&(U=s.DEPTH_COMPONENT16),U}function T(b,x){return p(b)===!0||b.isFramebufferTexture&&b.minFilter!==Kt&&b.minFilter!==Vt?Math.log2(Math.max(x.width,x.height))+1:b.mipmaps!==void 0&&b.mipmaps.length>0?b.mipmaps.length:b.isCompressedTexture&&Array.isArray(b.image)?x.mipmaps.length:1}function L(b){let x=b.target;x.removeEventListener("dispose",L),I(x),x.isVideoTexture&&h.delete(x),x.isHTMLTexture&&u.delete(x)}function y(b){let x=b.target;x.removeEventListener("dispose",y),R(x)}function I(b){let x=n.get(b);if(x.__webglInit===void 0)return;let U=b.source,B=f.get(U);if(B){let C=B[x.__cacheKey];C.usedTimes--,C.usedTimes===0&&E(b),Object.keys(B).length===0&&f.delete(U)}n.remove(b)}function E(b){let x=n.get(b);s.deleteTexture(x.__webglTexture);let U=b.source,B=f.get(U);delete B[x.__cacheKey],o.memory.textures--}function R(b){let x=n.get(b);if(b.depthTexture&&(b.depthTexture.dispose(),n.remove(b.depthTexture)),b.isWebGLCubeRenderTarget)for(let B=0;B<6;B++){if(Array.isArray(x.__webglFramebuffer[B]))for(let C=0;C<x.__webglFramebuffer[B].length;C++)s.deleteFramebuffer(x.__webglFramebuffer[B][C]);else s.deleteFramebuffer(x.__webglFramebuffer[B]);x.__webglDepthbuffer&&s.deleteRenderbuffer(x.__webglDepthbuffer[B])}else{if(Array.isArray(x.__webglFramebuffer))for(let B=0;B<x.__webglFramebuffer.length;B++)s.deleteFramebuffer(x.__webglFramebuffer[B]);else s.deleteFramebuffer(x.__webglFramebuffer);if(x.__webglDepthbuffer&&s.deleteRenderbuffer(x.__webglDepthbuffer),x.__webglMultisampledFramebuffer&&s.deleteFramebuffer(x.__webglMultisampledFramebuffer),x.__webglColorRenderbuffer)for(let B=0;B<x.__webglColorRenderbuffer.length;B++)x.__webglColorRenderbuffer[B]&&s.deleteRenderbuffer(x.__webglColorRenderbuffer[B]);x.__webglDepthRenderbuffer&&s.deleteRenderbuffer(x.__webglDepthRenderbuffer)}let U=b.textures;for(let B=0,C=U.length;B<C;B++){let k=n.get(U[B]);k.__webglTexture&&(s.deleteTexture(k.__webglTexture),o.memory.textures--),n.remove(U[B])}n.remove(b)}let F=0;function X(){F=0}function P(){return F}function N(b){F=b}function O(){let b=F;return b>=i.maxTextures&&Xe("WebGLTextures: Trying to use "+b+" texture units while this GPU supports only "+i.maxTextures),F+=1,b}function A(b){let x=[];return x.push(b.wrapS),x.push(b.wrapT),x.push(b.wrapR||0),x.push(b.magFilter),x.push(b.minFilter),x.push(b.anisotropy),x.push(b.internalFormat),x.push(b.format),x.push(b.type),x.push(b.generateMipmaps),x.push(b.premultiplyAlpha),x.push(b.flipY),x.push(b.unpackAlignment),x.push(b.colorSpace),x.join()}function V(b,x){let U=n.get(b);if(b.isVideoTexture&&q(b),b.isRenderTargetTexture===!1&&b.isExternalTexture!==!0&&b.version>0&&U.__version!==b.version){let B=b.image;if(B===null)Xe("WebGLRenderer: Texture marked for update but no image data found.");else if(B.complete===!1)Xe("WebGLRenderer: Texture marked for update but image is incomplete");else{Te(U,b,x);return}}else b.isExternalTexture&&(U.__webglTexture=b.sourceTexture?b.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,U.__webglTexture,s.TEXTURE0+x)}function Y(b,x){let U=n.get(b);if(b.isRenderTargetTexture===!1&&b.version>0&&U.__version!==b.version){Te(U,b,x);return}else b.isExternalTexture&&(U.__webglTexture=b.sourceTexture?b.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,U.__webglTexture,s.TEXTURE0+x)}function G(b,x){let U=n.get(b);if(b.isRenderTargetTexture===!1&&b.version>0&&U.__version!==b.version){Te(U,b,x);return}t.bindTexture(s.TEXTURE_3D,U.__webglTexture,s.TEXTURE0+x)}function ne(b,x){let U=n.get(b);if(b.isCubeDepthTexture!==!0&&b.version>0&&U.__version!==b.version){ue(U,b,x);return}t.bindTexture(s.TEXTURE_CUBE_MAP,U.__webglTexture,s.TEXTURE0+x)}let le={[$i]:s.REPEAT,[Hn]:s.CLAMP_TO_EDGE,[rr]:s.MIRRORED_REPEAT},Ae={[Kt]:s.NEAREST,[ul]:s.NEAREST_MIPMAP_NEAREST,[Fs]:s.NEAREST_MIPMAP_LINEAR,[Vt]:s.LINEAR,[wr]:s.LINEAR_MIPMAP_NEAREST,[ni]:s.LINEAR_MIPMAP_LINEAR},Pe={[Kd]:s.NEVER,[ef]:s.ALWAYS,[jd]:s.LESS,[Jl]:s.LEQUAL,[Jd]:s.EQUAL,[$l]:s.GEQUAL,[$d]:s.GREATER,[Qd]:s.NOTEQUAL};function Re(b,x){if(x.type===Nn&&e.has("OES_texture_float_linear")===!1&&(x.magFilter===Vt||x.magFilter===wr||x.magFilter===Fs||x.magFilter===ni||x.minFilter===Vt||x.minFilter===wr||x.minFilter===Fs||x.minFilter===ni)&&Xe("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(b,s.TEXTURE_WRAP_S,le[x.wrapS]),s.texParameteri(b,s.TEXTURE_WRAP_T,le[x.wrapT]),(b===s.TEXTURE_3D||b===s.TEXTURE_2D_ARRAY)&&s.texParameteri(b,s.TEXTURE_WRAP_R,le[x.wrapR]),s.texParameteri(b,s.TEXTURE_MAG_FILTER,Ae[x.magFilter]),s.texParameteri(b,s.TEXTURE_MIN_FILTER,Ae[x.minFilter]),x.compareFunction&&(s.texParameteri(b,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(b,s.TEXTURE_COMPARE_FUNC,Pe[x.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(x.magFilter===Kt||x.minFilter!==Fs&&x.minFilter!==ni||x.type===Nn&&e.has("OES_texture_float_linear")===!1)return;if(x.anisotropy>1||n.get(x).__currentAnisotropy){let U=e.get("EXT_texture_filter_anisotropic");s.texParameterf(b,U.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(x.anisotropy,i.getMaxAnisotropy())),n.get(x).__currentAnisotropy=x.anisotropy}}}function oe(b,x){let U=!1;b.__webglInit===void 0&&(b.__webglInit=!0,x.addEventListener("dispose",L));let B=x.source,C=f.get(B);C===void 0&&(C={},f.set(B,C));let k=A(x);if(k!==b.__cacheKey){C[k]===void 0&&(C[k]={texture:s.createTexture(),usedTimes:0},o.memory.textures++,U=!0),C[k].usedTimes++;let $=C[b.__cacheKey];$!==void 0&&(C[b.__cacheKey].usedTimes--,$.usedTimes===0&&E(x)),b.__cacheKey=k,b.__webglTexture=C[k].texture}return U}function pe(b,x,U){return Math.floor(Math.floor(b/U)/x)}function ce(b,x,U,B){let k=b.updateRanges;if(k.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,x.width,x.height,U,B,x.data);else{k.sort((Ie,be)=>Ie.start-be.start);let $=0;for(let Ie=1;Ie<k.length;Ie++){let be=k[$],ye=k[Ie],Fe=be.start+be.count,we=pe(ye.start,x.width,4),ze=pe(be.start,x.width,4);ye.start<=Fe+1&&we===ze&&pe(ye.start+ye.count-1,x.width,4)===we?be.count=Math.max(be.count,ye.start+ye.count-be.start):(++$,k[$]=ye)}k.length=$+1;let j=t.getParameter(s.UNPACK_ROW_LENGTH),Q=t.getParameter(s.UNPACK_SKIP_PIXELS),me=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,x.width);for(let Ie=0,be=k.length;Ie<be;Ie++){let ye=k[Ie],Fe=Math.floor(ye.start/4),we=Math.ceil(ye.count/4),ze=Fe%x.width,K=Math.floor(Fe/x.width),_e=we,de=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,ze),t.pixelStorei(s.UNPACK_SKIP_ROWS,K),t.texSubImage2D(s.TEXTURE_2D,0,ze,K,_e,de,U,B,x.data)}b.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,j),t.pixelStorei(s.UNPACK_SKIP_PIXELS,Q),t.pixelStorei(s.UNPACK_SKIP_ROWS,me)}}function Te(b,x,U){let B=s.TEXTURE_2D;(x.isDataArrayTexture||x.isCompressedArrayTexture)&&(B=s.TEXTURE_2D_ARRAY),x.isData3DTexture&&(B=s.TEXTURE_3D);let C=oe(b,x),k=x.source;t.bindTexture(B,b.__webglTexture,s.TEXTURE0+U);let $=n.get(k);if(k.version!==$.__version||C===!0){if(t.activeTexture(s.TEXTURE0+U),(typeof ImageBitmap<"u"&&x.image instanceof ImageBitmap)===!1){let de=rt.getPrimaries(rt.workingColorSpace),Se=x.colorSpace===Bi?null:rt.getPrimaries(x.colorSpace),Ee=x.colorSpace===Bi||de===Se?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,x.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,x.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Ee)}t.pixelStorei(s.UNPACK_ALIGNMENT,x.unpackAlignment);let Q=m(x.image,!1,i.maxTextureSize);Q=Be(x,Q);let me=r.convert(x.format,x.colorSpace),Ie=r.convert(x.type),be=_(x.internalFormat,me,Ie,x.normalized,x.colorSpace,x.isVideoTexture);Re(B,x);let ye,Fe=x.mipmaps,we=x.isVideoTexture!==!0,ze=$.__version===void 0||C===!0,K=k.dataReady,_e=T(x,Q);if(x.isDepthTexture)be=w(x.format===os,x.type),ze&&(we?t.texStorage2D(s.TEXTURE_2D,1,be,Q.width,Q.height):t.texImage2D(s.TEXTURE_2D,0,be,Q.width,Q.height,0,me,Ie,null));else if(x.isDataTexture)if(Fe.length>0){we&&ze&&t.texStorage2D(s.TEXTURE_2D,_e,be,Fe[0].width,Fe[0].height);for(let de=0,Se=Fe.length;de<Se;de++)ye=Fe[de],we?K&&t.texSubImage2D(s.TEXTURE_2D,de,0,0,ye.width,ye.height,me,Ie,ye.data):t.texImage2D(s.TEXTURE_2D,de,be,ye.width,ye.height,0,me,Ie,ye.data);x.generateMipmaps=!1}else we?(ze&&t.texStorage2D(s.TEXTURE_2D,_e,be,Q.width,Q.height),K&&ce(x,Q,me,Ie)):t.texImage2D(s.TEXTURE_2D,0,be,Q.width,Q.height,0,me,Ie,Q.data);else if(x.isCompressedTexture)if(x.isCompressedArrayTexture){we&&ze&&t.texStorage3D(s.TEXTURE_2D_ARRAY,_e,be,Fe[0].width,Fe[0].height,Q.depth);for(let de=0,Se=Fe.length;de<Se;de++)if(ye=Fe[de],x.format!==Un)if(me!==null)if(we){if(K)if(x.layerUpdates.size>0){let Ee=wh(ye.width,ye.height,x.format,x.type);for(let ge of x.layerUpdates){let Ve=ye.data.subarray(ge*Ee/ye.data.BYTES_PER_ELEMENT,(ge+1)*Ee/ye.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,de,0,0,ge,ye.width,ye.height,1,me,Ve)}x.clearLayerUpdates()}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,de,0,0,0,ye.width,ye.height,Q.depth,me,ye.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,de,be,ye.width,ye.height,Q.depth,0,ye.data,0,0);else Xe("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else we?K&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,de,0,0,0,ye.width,ye.height,Q.depth,me,Ie,ye.data):t.texImage3D(s.TEXTURE_2D_ARRAY,de,be,ye.width,ye.height,Q.depth,0,me,Ie,ye.data)}else{we&&ze&&t.texStorage2D(s.TEXTURE_2D,_e,be,Fe[0].width,Fe[0].height);for(let de=0,Se=Fe.length;de<Se;de++)ye=Fe[de],x.format!==Un?me!==null?we?K&&t.compressedTexSubImage2D(s.TEXTURE_2D,de,0,0,ye.width,ye.height,me,ye.data):t.compressedTexImage2D(s.TEXTURE_2D,de,be,ye.width,ye.height,0,ye.data):Xe("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):we?K&&t.texSubImage2D(s.TEXTURE_2D,de,0,0,ye.width,ye.height,me,Ie,ye.data):t.texImage2D(s.TEXTURE_2D,de,be,ye.width,ye.height,0,me,Ie,ye.data)}else if(x.isDataArrayTexture)if(we){if(ze&&t.texStorage3D(s.TEXTURE_2D_ARRAY,_e,be,Q.width,Q.height,Q.depth),K)if(x.layerUpdates.size>0){let de=wh(Q.width,Q.height,x.format,x.type);for(let Se of x.layerUpdates){let Ee=Q.data.subarray(Se*de/Q.data.BYTES_PER_ELEMENT,(Se+1)*de/Q.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,Se,Q.width,Q.height,1,me,Ie,Ee)}x.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,Q.width,Q.height,Q.depth,me,Ie,Q.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,be,Q.width,Q.height,Q.depth,0,me,Ie,Q.data);else if(x.isData3DTexture)we?(ze&&t.texStorage3D(s.TEXTURE_3D,_e,be,Q.width,Q.height,Q.depth),K&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,Q.width,Q.height,Q.depth,me,Ie,Q.data)):t.texImage3D(s.TEXTURE_3D,0,be,Q.width,Q.height,Q.depth,0,me,Ie,Q.data);else if(x.isFramebufferTexture){if(ze)if(we)t.texStorage2D(s.TEXTURE_2D,_e,be,Q.width,Q.height);else{let de=Q.width,Se=Q.height;for(let Ee=0;Ee<_e;Ee++)t.texImage2D(s.TEXTURE_2D,Ee,be,de,Se,0,me,Ie,null),de>>=1,Se>>=1}}else if(x.isHTMLTexture){if("texElementImage2D"in s){let de=s.canvas;if(de.hasAttribute("layoutsubtree")||de.setAttribute("layoutsubtree","true"),Q.parentNode!==de){de.appendChild(Q),u.add(x),de.onpaint=Se=>{let Ee=Se.changedElements;for(let ge of u)Ee.includes(ge.image)&&(ge.needsUpdate=!0)},de.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,Q);else{let Ee=s.RGBA,ge=s.RGBA,Ve=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,Ee,ge,Ve,Q)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(Fe.length>0){if(we&&ze){let de=$e(Fe[0]);t.texStorage2D(s.TEXTURE_2D,_e,be,de.width,de.height)}for(let de=0,Se=Fe.length;de<Se;de++)ye=Fe[de],we?K&&t.texSubImage2D(s.TEXTURE_2D,de,0,0,me,Ie,ye):t.texImage2D(s.TEXTURE_2D,de,be,me,Ie,ye);x.generateMipmaps=!1}else if(we){if(ze){let de=$e(Q);t.texStorage2D(s.TEXTURE_2D,_e,be,de.width,de.height)}K&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,me,Ie,Q)}else t.texImage2D(s.TEXTURE_2D,0,be,me,Ie,Q);p(x)&&v(B),$.__version=k.version,x.onUpdate&&x.onUpdate(x)}b.__version=x.version}function ue(b,x,U){if(x.image.length!==6)return;let B=oe(b,x),C=x.source;t.bindTexture(s.TEXTURE_CUBE_MAP,b.__webglTexture,s.TEXTURE0+U);let k=n.get(C);if(C.version!==k.__version||B===!0){t.activeTexture(s.TEXTURE0+U);let $=rt.getPrimaries(rt.workingColorSpace),j=x.colorSpace===Bi?null:rt.getPrimaries(x.colorSpace),Q=x.colorSpace===Bi||$===j?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,x.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,x.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,x.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Q);let me=x.isCompressedTexture||x.image[0].isCompressedTexture,Ie=x.image[0]&&x.image[0].isDataTexture,be=[];for(let ge=0;ge<6;ge++)!me&&!Ie?be[ge]=m(x.image[ge],!0,i.maxCubemapSize):be[ge]=Ie?x.image[ge].image:x.image[ge],be[ge]=Be(x,be[ge]);let ye=be[0],Fe=r.convert(x.format,x.colorSpace),we=r.convert(x.type),ze=_(x.internalFormat,Fe,we,x.normalized,x.colorSpace),K=x.isVideoTexture!==!0,_e=k.__version===void 0||B===!0,de=C.dataReady,Se=T(x,ye);Re(s.TEXTURE_CUBE_MAP,x);let Ee;if(me){K&&_e&&t.texStorage2D(s.TEXTURE_CUBE_MAP,Se,ze,ye.width,ye.height);for(let ge=0;ge<6;ge++){Ee=be[ge].mipmaps;for(let Ve=0;Ve<Ee.length;Ve++){let Ge=Ee[Ve];x.format!==Un?Fe!==null?K?de&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve,0,0,Ge.width,Ge.height,Fe,Ge.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve,ze,Ge.width,Ge.height,0,Ge.data):Xe("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):K?de&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve,0,0,Ge.width,Ge.height,Fe,we,Ge.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve,ze,Ge.width,Ge.height,0,Fe,we,Ge.data)}}}else{if(Ee=x.mipmaps,K&&_e){Ee.length>0&&Se++;let ge=$e(be[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,Se,ze,ge.width,ge.height)}for(let ge=0;ge<6;ge++)if(Ie){K?de&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,0,0,0,be[ge].width,be[ge].height,Fe,we,be[ge].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,0,ze,be[ge].width,be[ge].height,0,Fe,we,be[ge].data);for(let Ve=0;Ve<Ee.length;Ve++){let zt=Ee[Ve].image[ge].image;K?de&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve+1,0,0,zt.width,zt.height,Fe,we,zt.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve+1,ze,zt.width,zt.height,0,Fe,we,zt.data)}}else{K?de&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,0,0,0,Fe,we,be[ge]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,0,ze,Fe,we,be[ge]);for(let Ve=0;Ve<Ee.length;Ve++){let Ge=Ee[Ve];K?de&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve+1,0,0,Fe,we,Ge.image[ge]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ge,Ve+1,ze,Fe,we,Ge.image[ge])}}}p(x)&&v(s.TEXTURE_CUBE_MAP),k.__version=C.version,x.onUpdate&&x.onUpdate(x)}b.__version=x.version}function he(b,x,U,B,C,k){let $=r.convert(U.format,U.colorSpace),j=r.convert(U.type),Q=_(U.internalFormat,$,j,U.normalized,U.colorSpace),me=n.get(x),Ie=n.get(U);if(Ie.__renderTarget=x,!me.__hasExternalTextures){let be=Math.max(1,x.width>>k),ye=Math.max(1,x.height>>k);C===s.TEXTURE_3D||C===s.TEXTURE_2D_ARRAY?t.texImage3D(C,k,Q,be,ye,x.depth,0,$,j,null):t.texImage2D(C,k,Q,be,ye,0,$,j,null)}t.bindFramebuffer(s.FRAMEBUFFER,b),Je(x)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,B,C,Ie.__webglTexture,0,Ye(x)):(C===s.TEXTURE_2D||C>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&C<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,B,C,Ie.__webglTexture,k),t.bindFramebuffer(s.FRAMEBUFFER,null)}function H(b,x,U){if(s.bindRenderbuffer(s.RENDERBUFFER,b),x.depthBuffer){let B=x.depthTexture,C=B&&B.isDepthTexture?B.type:null,k=w(x.stencilBuffer,C),$=x.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;Je(x)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Ye(x),k,x.width,x.height):U?s.renderbufferStorageMultisample(s.RENDERBUFFER,Ye(x),k,x.width,x.height):s.renderbufferStorage(s.RENDERBUFFER,k,x.width,x.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,$,s.RENDERBUFFER,b)}else{let B=x.textures;for(let C=0;C<B.length;C++){let k=B[C],$=r.convert(k.format,k.colorSpace),j=r.convert(k.type),Q=_(k.internalFormat,$,j,k.normalized,k.colorSpace);Je(x)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Ye(x),Q,x.width,x.height):U?s.renderbufferStorageMultisample(s.RENDERBUFFER,Ye(x),Q,x.width,x.height):s.renderbufferStorage(s.RENDERBUFFER,Q,x.width,x.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function Z(b,x,U){let B=x.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,b),!(x.depthTexture&&x.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let C=n.get(x.depthTexture);if(C.__renderTarget=x,(!C.__webglTexture||x.depthTexture.image.width!==x.width||x.depthTexture.image.height!==x.height)&&(x.depthTexture.image.width=x.width,x.depthTexture.image.height=x.height,x.depthTexture.needsUpdate=!0),B){if(C.__webglInit===void 0&&(C.__webglInit=!0,x.depthTexture.addEventListener("dispose",L)),C.__webglTexture===void 0){C.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,C.__webglTexture),Re(s.TEXTURE_CUBE_MAP,x.depthTexture);let me=r.convert(x.depthTexture.format),Ie=r.convert(x.depthTexture.type),be;x.depthTexture.format===hi?be=s.DEPTH_COMPONENT24:x.depthTexture.format===os&&(be=s.DEPTH24_STENCIL8);for(let ye=0;ye<6;ye++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ye,0,be,x.width,x.height,0,me,Ie,null)}}else V(x.depthTexture,0);let k=C.__webglTexture,$=Ye(x),j=B?s.TEXTURE_CUBE_MAP_POSITIVE_X+U:s.TEXTURE_2D,Q=x.depthTexture.format===os?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(x.depthTexture.format===hi)Je(x)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,Q,j,k,0,$):s.framebufferTexture2D(s.FRAMEBUFFER,Q,j,k,0);else if(x.depthTexture.format===os)Je(x)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,Q,j,k,0,$):s.framebufferTexture2D(s.FRAMEBUFFER,Q,j,k,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function W(b){let x=n.get(b),U=b.isWebGLCubeRenderTarget===!0;if(x.__boundDepthTexture!==b.depthTexture){let B=b.depthTexture;if(x.__depthDisposeCallback&&x.__depthDisposeCallback(),B){let C=()=>{delete x.__boundDepthTexture,delete x.__depthDisposeCallback,B.removeEventListener("dispose",C)};B.addEventListener("dispose",C),x.__depthDisposeCallback=C}x.__boundDepthTexture=B}if(b.depthTexture&&!x.__autoAllocateDepthBuffer)if(U)for(let B=0;B<6;B++)Z(x.__webglFramebuffer[B],b,B);else{let B=b.texture.mipmaps;B&&B.length>0?Z(x.__webglFramebuffer[0],b,0):Z(x.__webglFramebuffer,b,0)}else if(U){x.__webglDepthbuffer=[];for(let B=0;B<6;B++)if(t.bindFramebuffer(s.FRAMEBUFFER,x.__webglFramebuffer[B]),x.__webglDepthbuffer[B]===void 0)x.__webglDepthbuffer[B]=s.createRenderbuffer(),H(x.__webglDepthbuffer[B],b,!1);else{let C=b.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,k=x.__webglDepthbuffer[B];s.bindRenderbuffer(s.RENDERBUFFER,k),s.framebufferRenderbuffer(s.FRAMEBUFFER,C,s.RENDERBUFFER,k)}}else{let B=b.texture.mipmaps;if(B&&B.length>0?t.bindFramebuffer(s.FRAMEBUFFER,x.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,x.__webglFramebuffer),x.__webglDepthbuffer===void 0)x.__webglDepthbuffer=s.createRenderbuffer(),H(x.__webglDepthbuffer,b,!1);else{let C=b.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,k=x.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,k),s.framebufferRenderbuffer(s.FRAMEBUFFER,C,s.RENDERBUFFER,k)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function J(b,x,U){let B=n.get(b);x!==void 0&&he(B.__webglFramebuffer,b,b.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),U!==void 0&&W(b)}function te(b){let x=b.texture,U=n.get(b),B=n.get(x);b.addEventListener("dispose",y);let C=b.textures,k=b.isWebGLCubeRenderTarget===!0,$=C.length>1;if($||(B.__webglTexture===void 0&&(B.__webglTexture=s.createTexture()),B.__version=x.version,o.memory.textures++),k){U.__webglFramebuffer=[];for(let j=0;j<6;j++)if(x.mipmaps&&x.mipmaps.length>0){U.__webglFramebuffer[j]=[];for(let Q=0;Q<x.mipmaps.length;Q++)U.__webglFramebuffer[j][Q]=s.createFramebuffer()}else U.__webglFramebuffer[j]=s.createFramebuffer()}else{if(x.mipmaps&&x.mipmaps.length>0){U.__webglFramebuffer=[];for(let j=0;j<x.mipmaps.length;j++)U.__webglFramebuffer[j]=s.createFramebuffer()}else U.__webglFramebuffer=s.createFramebuffer();if($)for(let j=0,Q=C.length;j<Q;j++){let me=n.get(C[j]);me.__webglTexture===void 0&&(me.__webglTexture=s.createTexture(),o.memory.textures++)}if(b.samples>0&&Je(b)===!1){U.__webglMultisampledFramebuffer=s.createFramebuffer(),U.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,U.__webglMultisampledFramebuffer);for(let j=0;j<C.length;j++){let Q=C[j];U.__webglColorRenderbuffer[j]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,U.__webglColorRenderbuffer[j]);let me=r.convert(Q.format,Q.colorSpace),Ie=r.convert(Q.type),be=_(Q.internalFormat,me,Ie,Q.normalized,Q.colorSpace,b.isXRRenderTarget===!0),ye=Ye(b);s.renderbufferStorageMultisample(s.RENDERBUFFER,ye,be,b.width,b.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+j,s.RENDERBUFFER,U.__webglColorRenderbuffer[j])}s.bindRenderbuffer(s.RENDERBUFFER,null),b.depthBuffer&&(U.__webglDepthRenderbuffer=s.createRenderbuffer(),H(U.__webglDepthRenderbuffer,b,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(k){t.bindTexture(s.TEXTURE_CUBE_MAP,B.__webglTexture),Re(s.TEXTURE_CUBE_MAP,x);for(let j=0;j<6;j++)if(x.mipmaps&&x.mipmaps.length>0)for(let Q=0;Q<x.mipmaps.length;Q++)he(U.__webglFramebuffer[j][Q],b,x,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+j,Q);else he(U.__webglFramebuffer[j],b,x,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+j,0);p(x)&&v(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if($){for(let j=0,Q=C.length;j<Q;j++){let me=C[j],Ie=n.get(me),be=s.TEXTURE_2D;(b.isWebGL3DRenderTarget||b.isWebGLArrayRenderTarget)&&(be=b.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(be,Ie.__webglTexture),Re(be,me),he(U.__webglFramebuffer,b,me,s.COLOR_ATTACHMENT0+j,be,0),p(me)&&v(be)}t.unbindTexture()}else{let j=s.TEXTURE_2D;if((b.isWebGL3DRenderTarget||b.isWebGLArrayRenderTarget)&&(j=b.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(j,B.__webglTexture),Re(j,x),x.mipmaps&&x.mipmaps.length>0)for(let Q=0;Q<x.mipmaps.length;Q++)he(U.__webglFramebuffer[Q],b,x,s.COLOR_ATTACHMENT0,j,Q);else he(U.__webglFramebuffer,b,x,s.COLOR_ATTACHMENT0,j,0);p(x)&&v(j),t.unbindTexture()}b.depthBuffer&&W(b)}function fe(b){let x=b.textures;for(let U=0,B=x.length;U<B;U++){let C=x[U];if(p(C)){let k=S(b),$=n.get(C).__webglTexture;t.bindTexture(k,$),v(k),t.unbindTexture()}}}let Me=[],Oe=[];function We(b){if(b.samples>0){if(Je(b)===!1){let x=b.textures,U=b.width,B=b.height,C=s.COLOR_BUFFER_BIT,k=b.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,$=n.get(b),j=x.length>1;if(j)for(let me=0;me<x.length;me++)t.bindFramebuffer(s.FRAMEBUFFER,$.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,$.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,$.__webglMultisampledFramebuffer);let Q=b.texture.mipmaps;Q&&Q.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,$.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,$.__webglFramebuffer);for(let me=0;me<x.length;me++){if(b.resolveDepthBuffer&&(b.depthBuffer&&(C|=s.DEPTH_BUFFER_BIT),b.stencilBuffer&&b.resolveStencilBuffer&&(C|=s.STENCIL_BUFFER_BIT)),j){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,$.__webglColorRenderbuffer[me]);let Ie=n.get(x[me]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,Ie,0)}s.blitFramebuffer(0,0,U,B,0,0,U,B,C,s.NEAREST),l===!0&&(Me.length=0,Oe.length=0,Me.push(s.COLOR_ATTACHMENT0+me),b.depthBuffer&&b.resolveDepthBuffer===!1&&(Me.push(k),Oe.push(k),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Oe)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,Me))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),j)for(let me=0;me<x.length;me++){t.bindFramebuffer(s.FRAMEBUFFER,$.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.RENDERBUFFER,$.__webglColorRenderbuffer[me]);let Ie=n.get(x[me]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,$.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+me,s.TEXTURE_2D,Ie,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,$.__webglMultisampledFramebuffer)}else if(b.depthBuffer&&b.resolveDepthBuffer===!1&&l){let x=b.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[x])}}}function Ye(b){return Math.min(i.maxSamples,b.samples)}function Je(b){let x=n.get(b);return b.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&x.__useRenderToTexture!==!1}function q(b){let x=o.render.frame;h.get(b)!==x&&(h.set(b,x),b.update())}function Be(b,x){let U=b.colorSpace,B=b.format,C=b.type;return b.isCompressedTexture===!0||b.isVideoTexture===!0||U!==mn&&U!==Bi&&(rt.getTransfer(U)===yt?(B!==Un||C!==Sn)&&Xe("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):Qe("WebGLTextures: Unsupported texture color space:",U)),x}function $e(b){return typeof HTMLImageElement<"u"&&b instanceof HTMLImageElement?(c.width=b.naturalWidth||b.width,c.height=b.naturalHeight||b.height):typeof VideoFrame<"u"&&b instanceof VideoFrame?(c.width=b.displayWidth,c.height=b.displayHeight):(c.width=b.width,c.height=b.height),c}this.allocateTextureUnit=O,this.resetTextureUnits=X,this.getTextureUnits=P,this.setTextureUnits=N,this.setTexture2D=V,this.setTexture2DArray=Y,this.setTexture3D=G,this.setTextureCube=ne,this.rebindTextures=J,this.setupRenderTarget=te,this.updateRenderTargetMipmap=fe,this.updateMultisampleRenderTarget=We,this.setupDepthRenderbuffer=W,this.setupFrameBufferTexture=he,this.useMultisampledRTT=Je,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function bv(s,e){function t(n,i=Bi){let r,o=rt.getTransfer(i);if(n===Sn)return s.UNSIGNED_BYTE;if(n===fl)return s.UNSIGNED_SHORT_4_4_4_4;if(n===pl)return s.UNSIGNED_SHORT_5_5_5_1;if(n===mh)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===gh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===fh)return s.BYTE;if(n===ph)return s.SHORT;if(n===Ar)return s.UNSIGNED_SHORT;if(n===dl)return s.INT;if(n===ii)return s.UNSIGNED_INT;if(n===Nn)return s.FLOAT;if(n===nn)return s.HALF_FLOAT;if(n===xh)return s.ALPHA;if(n===_h)return s.RGB;if(n===Un)return s.RGBA;if(n===hi)return s.DEPTH_COMPONENT;if(n===os)return s.DEPTH_STENCIL;if(n===ml)return s.RED;if(n===gl)return s.RED_INTEGER;if(n===as)return s.RG;if(n===xl)return s.RG_INTEGER;if(n===_l)return s.RGBA_INTEGER;if(n===Fo||n===Oo||n===Bo||n===zo)if(o===yt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===Fo)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===Oo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===Bo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===zo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===Fo)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===Oo)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===Bo)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===zo)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===vl||n===yl||n===Ml||n===bl)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===vl)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===yl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===Ml)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===bl)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===Sl||n===Tl||n===El||n===wl||n===Al||n===ko||n===Rl)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===Sl||n===Tl)return o===yt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===El)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===wl)return r.COMPRESSED_R11_EAC;if(n===Al)return r.COMPRESSED_SIGNED_R11_EAC;if(n===ko)return r.COMPRESSED_RG11_EAC;if(n===Rl)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===Cl||n===Pl||n===Il||n===Ll||n===Dl||n===Nl||n===Ul||n===Fl||n===Ol||n===Bl||n===zl||n===kl||n===Hl||n===Vl)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===Cl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Pl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===Il)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===Ll)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===Dl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===Nl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===Ul)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===Fl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===Ol)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===Bl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===zl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===kl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===Hl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Vl)return o===yt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===Gl||n===Wl||n===Xl)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===Gl)return o===yt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===Wl)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===Xl)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===ql||n===Yl||n===Ho||n===Zl)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===ql)return r.COMPRESSED_RED_RGTC1_EXT;if(n===Yl)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===Ho)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===Zl)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Rr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var Sv=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,Tv=`
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

}`,kh=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new co(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new mt({vertexShader:Sv,fragmentShader:Tv,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new je(new ln(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Hh=class extends Vn{constructor(e,t){super();let n=this,i=null,r=1,o=null,a="local-floor",l=1,c=null,h=null,u=null,d=null,f=null,g=null,M=typeof XRWebGLBinding<"u",m=new kh,p={},v=t.getContextAttributes(),S=null,_=null,w=[],T=[],L=new ve,y=null,I=new $t;I.viewport=new Mt;let E=new $t;E.viewport=new Mt;let R=[I,E],F=new sl,X=null,P=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(oe){let pe=w[oe];return pe===void 0&&(pe=new ur,w[oe]=pe),pe.getTargetRaySpace()},this.getControllerGrip=function(oe){let pe=w[oe];return pe===void 0&&(pe=new ur,w[oe]=pe),pe.getGripSpace()},this.getHand=function(oe){let pe=w[oe];return pe===void 0&&(pe=new ur,w[oe]=pe),pe.getHandSpace()};function N(oe){let pe=T.indexOf(oe.inputSource);if(pe===-1)return;let ce=w[pe];ce!==void 0&&(ce.update(oe.inputSource,oe.frame,c||o),ce.dispatchEvent({type:oe.type,data:oe.inputSource}))}function O(){i.removeEventListener("select",N),i.removeEventListener("selectstart",N),i.removeEventListener("selectend",N),i.removeEventListener("squeeze",N),i.removeEventListener("squeezestart",N),i.removeEventListener("squeezeend",N),i.removeEventListener("end",O),i.removeEventListener("inputsourceschange",A);for(let oe=0;oe<w.length;oe++){let pe=T[oe];pe!==null&&(T[oe]=null,w[oe].disconnect(pe))}X=null,P=null,m.reset();for(let oe in p)delete p[oe];e.setRenderTarget(S),f=null,d=null,u=null,i=null,_=null,Re.stop(),n.isPresenting=!1,e.setPixelRatio(y),e.setSize(L.width,L.height,!1),n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(oe){r=oe,n.isPresenting===!0&&Xe("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(oe){a=oe,n.isPresenting===!0&&Xe("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||o},this.setReferenceSpace=function(oe){c=oe},this.getBaseLayer=function(){return d!==null?d:f},this.getBinding=function(){return u===null&&M&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return g},this.getSession=function(){return i},this.setSession=async function(oe){if(i=oe,i!==null){if(S=e.getRenderTarget(),i.addEventListener("select",N),i.addEventListener("selectstart",N),i.addEventListener("selectend",N),i.addEventListener("squeeze",N),i.addEventListener("squeezestart",N),i.addEventListener("squeezeend",N),i.addEventListener("end",O),i.addEventListener("inputsourceschange",A),v.xrCompatible!==!0&&await t.makeXRCompatible(),y=e.getPixelRatio(),e.getSize(L),M&&"createProjectionLayer"in XRWebGLBinding.prototype){let ce=null,Te=null,ue=null;v.depth&&(ue=v.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,ce=v.stencil?os:hi,Te=v.stencil?Rr:ii);let he={colorFormat:t.RGBA8,depthFormat:ue,scaleFactor:r};u=this.getBinding(),d=u.createProjectionLayer(he),i.updateRenderState({layers:[d]}),e.setPixelRatio(1),e.setSize(d.textureWidth,d.textureHeight,!1),_=new Wt(d.textureWidth,d.textureHeight,{format:Un,type:Sn,depthTexture:new Li(d.textureWidth,d.textureHeight,Te,void 0,void 0,void 0,void 0,void 0,void 0,ce),stencilBuffer:v.stencil,colorSpace:e.outputColorSpace,samples:v.antialias?4:0,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1})}else{let ce={antialias:v.antialias,alpha:!0,depth:v.depth,stencil:v.stencil,framebufferScaleFactor:r};f=new XRWebGLLayer(i,t,ce),i.updateRenderState({baseLayer:f}),e.setPixelRatio(1),e.setSize(f.framebufferWidth,f.framebufferHeight,!1),_=new Wt(f.framebufferWidth,f.framebufferHeight,{format:Un,type:Sn,colorSpace:e.outputColorSpace,stencilBuffer:v.stencil,resolveDepthBuffer:f.ignoreDepthValues===!1,resolveStencilBuffer:f.ignoreDepthValues===!1})}_.isXRRenderTarget=!0,this.setFoveation(l),c=null,o=await i.requestReferenceSpace(a),Re.setContext(i),Re.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function A(oe){for(let pe=0;pe<oe.removed.length;pe++){let ce=oe.removed[pe],Te=T.indexOf(ce);Te>=0&&(T[Te]=null,w[Te].disconnect(ce))}for(let pe=0;pe<oe.added.length;pe++){let ce=oe.added[pe],Te=T.indexOf(ce);if(Te===-1){for(let he=0;he<w.length;he++)if(he>=T.length){T.push(ce),Te=he;break}else if(T[he]===null){T[he]=ce,Te=he;break}if(Te===-1)break}let ue=w[Te];ue&&ue.connect(ce)}}let V=new z,Y=new z;function G(oe,pe,ce){V.setFromMatrixPosition(pe.matrixWorld),Y.setFromMatrixPosition(ce.matrixWorld);let Te=V.distanceTo(Y),ue=pe.projectionMatrix.elements,he=ce.projectionMatrix.elements,H=ue[14]/(ue[10]-1),Z=ue[14]/(ue[10]+1),W=(ue[9]+1)/ue[5],J=(ue[9]-1)/ue[5],te=(ue[8]-1)/ue[0],fe=(he[8]+1)/he[0],Me=H*te,Oe=H*fe,We=Te/(-te+fe),Ye=We*-te;if(pe.matrixWorld.decompose(oe.position,oe.quaternion,oe.scale),oe.translateX(Ye),oe.translateZ(We),oe.matrixWorld.compose(oe.position,oe.quaternion,oe.scale),oe.matrixWorldInverse.copy(oe.matrixWorld).invert(),ue[10]===-1)oe.projectionMatrix.copy(pe.projectionMatrix),oe.projectionMatrixInverse.copy(pe.projectionMatrixInverse);else{let Je=H+We,q=Z+We,Be=Me-Ye,$e=Oe+(Te-Ye),b=W*Z/q*Je,x=J*Z/q*Je;oe.projectionMatrix.makePerspective(Be,$e,b,x,Je,q),oe.projectionMatrixInverse.copy(oe.projectionMatrix).invert()}}function ne(oe,pe){pe===null?oe.matrixWorld.copy(oe.matrix):oe.matrixWorld.multiplyMatrices(pe.matrixWorld,oe.matrix),oe.matrixWorldInverse.copy(oe.matrixWorld).invert()}this.updateCamera=function(oe){if(i===null)return;let pe=oe.near,ce=oe.far;m.texture!==null&&(m.depthNear>0&&(pe=m.depthNear),m.depthFar>0&&(ce=m.depthFar)),F.near=E.near=I.near=pe,F.far=E.far=I.far=ce,(X!==F.near||P!==F.far)&&(i.updateRenderState({depthNear:F.near,depthFar:F.far}),X=F.near,P=F.far),F.layers.mask=oe.layers.mask|6,I.layers.mask=F.layers.mask&-5,E.layers.mask=F.layers.mask&-3;let Te=oe.parent,ue=F.cameras;ne(F,Te);for(let he=0;he<ue.length;he++)ne(ue[he],Te);ue.length===2?G(F,I,E):F.projectionMatrix.copy(I.projectionMatrix),le(oe,F,Te)};function le(oe,pe,ce){ce===null?oe.matrix.copy(pe.matrixWorld):(oe.matrix.copy(ce.matrixWorld),oe.matrix.invert(),oe.matrix.multiply(pe.matrixWorld)),oe.matrix.decompose(oe.position,oe.quaternion,oe.scale),oe.updateMatrixWorld(!0),oe.projectionMatrix.copy(pe.projectionMatrix),oe.projectionMatrixInverse.copy(pe.projectionMatrixInverse),oe.isPerspectiveCamera&&(oe.fov=ws*2*Math.atan(1/oe.projectionMatrix.elements[5]),oe.zoom=1)}this.getCamera=function(){return F},this.getFoveation=function(){if(!(d===null&&f===null))return l},this.setFoveation=function(oe){l=oe,d!==null&&(d.fixedFoveation=oe),f!==null&&f.fixedFoveation!==void 0&&(f.fixedFoveation=oe)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(F)},this.getCameraTexture=function(oe){return p[oe]};let Ae=null;function Pe(oe,pe){if(h=pe.getViewerPose(c||o),g=pe,h!==null){let ce=h.views;f!==null&&(e.setRenderTargetFramebuffer(_,f.framebuffer),e.setRenderTarget(_));let Te=!1;ce.length!==F.cameras.length&&(F.cameras.length=0,Te=!0);for(let Z=0;Z<ce.length;Z++){let W=ce[Z],J=null;if(f!==null)J=f.getViewport(W);else{let fe=u.getViewSubImage(d,W);J=fe.viewport,Z===0&&(e.setRenderTargetTextures(_,fe.colorTexture,fe.depthStencilTexture),e.setRenderTarget(_))}let te=R[Z];te===void 0&&(te=new $t,te.layers.enable(Z),te.viewport=new Mt,R[Z]=te),te.matrix.fromArray(W.transform.matrix),te.matrix.decompose(te.position,te.quaternion,te.scale),te.projectionMatrix.fromArray(W.projectionMatrix),te.projectionMatrixInverse.copy(te.projectionMatrix).invert(),te.viewport.set(J.x,J.y,J.width,J.height),Z===0&&(F.matrix.copy(te.matrix),F.matrix.decompose(F.position,F.quaternion,F.scale)),Te===!0&&F.cameras.push(te)}let ue=i.enabledFeatures;if(ue&&ue.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&M){u=n.getBinding();let Z=u.getDepthInformation(ce[0]);Z&&Z.isValid&&Z.texture&&m.init(Z,i.renderState)}if(ue&&ue.includes("camera-access")&&M){e.state.unbindTexture(),u=n.getBinding();for(let Z=0;Z<ce.length;Z++){let W=ce[Z].camera;if(W){let J=p[W];J||(J=new co,p[W]=J);let te=u.getCameraImage(W);J.sourceTexture=te}}}}for(let ce=0;ce<w.length;ce++){let Te=T[ce],ue=w[ce];Te!==null&&ue!==void 0&&ue.update(Te,pe,c||o)}Ae&&Ae(oe,pe),pe.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:pe}),g=null}let Re=new Pf;Re.setAnimationLoop(Pe),this.setAnimationLoop=function(oe){Ae=oe},this.dispose=function(){}}},Ev=new et,Ff=new tt;Ff.set(-1,0,0,0,1,0,0,0,1);function wv(s,e){function t(m,p){m.matrixAutoUpdate===!0&&m.updateMatrix(),p.value.copy(m.matrix)}function n(m,p){p.color.getRGB(m.fogColor.value,Sh(s)),p.isFog?(m.fogNear.value=p.near,m.fogFar.value=p.far):p.isFogExp2&&(m.fogDensity.value=p.density)}function i(m,p,v,S,_){p.isNodeMaterial?p.uniformsNeedUpdate=!1:p.isMeshBasicMaterial?r(m,p):p.isMeshLambertMaterial?(r(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshToonMaterial?(r(m,p),u(m,p)):p.isMeshPhongMaterial?(r(m,p),h(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshStandardMaterial?(r(m,p),d(m,p),p.isMeshPhysicalMaterial&&f(m,p,_)):p.isMeshMatcapMaterial?(r(m,p),g(m,p)):p.isMeshDepthMaterial?r(m,p):p.isMeshDistanceMaterial?(r(m,p),M(m,p)):p.isMeshNormalMaterial?r(m,p):p.isLineBasicMaterial?(o(m,p),p.isLineDashedMaterial&&a(m,p)):p.isPointsMaterial?l(m,p,v,S):p.isSpriteMaterial?c(m,p):p.isShadowMaterial?(m.color.value.copy(p.color),m.opacity.value=p.opacity):p.isShaderMaterial&&(p.uniformsNeedUpdate=!1)}function r(m,p){m.opacity.value=p.opacity,p.color&&m.diffuse.value.copy(p.color),p.emissive&&m.emissive.value.copy(p.emissive).multiplyScalar(p.emissiveIntensity),p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.bumpMap&&(m.bumpMap.value=p.bumpMap,t(p.bumpMap,m.bumpMapTransform),m.bumpScale.value=p.bumpScale,p.side===tn&&(m.bumpScale.value*=-1)),p.normalMap&&(m.normalMap.value=p.normalMap,t(p.normalMap,m.normalMapTransform),m.normalScale.value.copy(p.normalScale),p.side===tn&&m.normalScale.value.negate()),p.displacementMap&&(m.displacementMap.value=p.displacementMap,t(p.displacementMap,m.displacementMapTransform),m.displacementScale.value=p.displacementScale,m.displacementBias.value=p.displacementBias),p.emissiveMap&&(m.emissiveMap.value=p.emissiveMap,t(p.emissiveMap,m.emissiveMapTransform)),p.specularMap&&(m.specularMap.value=p.specularMap,t(p.specularMap,m.specularMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest);let v=e.get(p),S=v.envMap,_=v.envMapRotation;S&&(m.envMap.value=S,m.envMapRotation.value.setFromMatrix4(Ev.makeRotationFromEuler(_)).transpose(),S.isCubeTexture&&S.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(Ff),m.reflectivity.value=p.reflectivity,m.ior.value=p.ior,m.refractionRatio.value=p.refractionRatio),p.lightMap&&(m.lightMap.value=p.lightMap,m.lightMapIntensity.value=p.lightMapIntensity,t(p.lightMap,m.lightMapTransform)),p.aoMap&&(m.aoMap.value=p.aoMap,m.aoMapIntensity.value=p.aoMapIntensity,t(p.aoMap,m.aoMapTransform))}function o(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform))}function a(m,p){m.dashSize.value=p.dashSize,m.totalSize.value=p.dashSize+p.gapSize,m.scale.value=p.scale}function l(m,p,v,S){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.size.value=p.size*v,m.scale.value=S*.5,p.map&&(m.map.value=p.map,t(p.map,m.uvTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function c(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.rotation.value=p.rotation,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function h(m,p){m.specular.value.copy(p.specular),m.shininess.value=Math.max(p.shininess,1e-4)}function u(m,p){p.gradientMap&&(m.gradientMap.value=p.gradientMap)}function d(m,p){m.metalness.value=p.metalness,p.metalnessMap&&(m.metalnessMap.value=p.metalnessMap,t(p.metalnessMap,m.metalnessMapTransform)),m.roughness.value=p.roughness,p.roughnessMap&&(m.roughnessMap.value=p.roughnessMap,t(p.roughnessMap,m.roughnessMapTransform)),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)}function f(m,p,v){m.ior.value=p.ior,p.sheen>0&&(m.sheenColor.value.copy(p.sheenColor).multiplyScalar(p.sheen),m.sheenRoughness.value=p.sheenRoughness,p.sheenColorMap&&(m.sheenColorMap.value=p.sheenColorMap,t(p.sheenColorMap,m.sheenColorMapTransform)),p.sheenRoughnessMap&&(m.sheenRoughnessMap.value=p.sheenRoughnessMap,t(p.sheenRoughnessMap,m.sheenRoughnessMapTransform))),p.clearcoat>0&&(m.clearcoat.value=p.clearcoat,m.clearcoatRoughness.value=p.clearcoatRoughness,p.clearcoatMap&&(m.clearcoatMap.value=p.clearcoatMap,t(p.clearcoatMap,m.clearcoatMapTransform)),p.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=p.clearcoatRoughnessMap,t(p.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),p.clearcoatNormalMap&&(m.clearcoatNormalMap.value=p.clearcoatNormalMap,t(p.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(p.clearcoatNormalScale),p.side===tn&&m.clearcoatNormalScale.value.negate())),p.dispersion>0&&(m.dispersion.value=p.dispersion),p.iridescence>0&&(m.iridescence.value=p.iridescence,m.iridescenceIOR.value=p.iridescenceIOR,m.iridescenceThicknessMinimum.value=p.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=p.iridescenceThicknessRange[1],p.iridescenceMap&&(m.iridescenceMap.value=p.iridescenceMap,t(p.iridescenceMap,m.iridescenceMapTransform)),p.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=p.iridescenceThicknessMap,t(p.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),p.transmission>0&&(m.transmission.value=p.transmission,m.transmissionSamplerMap.value=v.texture,m.transmissionSamplerSize.value.set(v.width,v.height),p.transmissionMap&&(m.transmissionMap.value=p.transmissionMap,t(p.transmissionMap,m.transmissionMapTransform)),m.thickness.value=p.thickness,p.thicknessMap&&(m.thicknessMap.value=p.thicknessMap,t(p.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=p.attenuationDistance,m.attenuationColor.value.copy(p.attenuationColor)),p.anisotropy>0&&(m.anisotropyVector.value.set(p.anisotropy*Math.cos(p.anisotropyRotation),p.anisotropy*Math.sin(p.anisotropyRotation)),p.anisotropyMap&&(m.anisotropyMap.value=p.anisotropyMap,t(p.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=p.specularIntensity,m.specularColor.value.copy(p.specularColor),p.specularColorMap&&(m.specularColorMap.value=p.specularColorMap,t(p.specularColorMap,m.specularColorMapTransform)),p.specularIntensityMap&&(m.specularIntensityMap.value=p.specularIntensityMap,t(p.specularIntensityMap,m.specularIntensityMapTransform))}function g(m,p){p.matcap&&(m.matcap.value=p.matcap)}function M(m,p){let v=e.get(p).light;m.referencePosition.value.setFromMatrixPosition(v.matrixWorld),m.nearDistance.value=v.shadow.camera.near,m.farDistance.value=v.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function Av(s,e,t,n){let i={},r={},o=[],a=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(_,w){let T=w.program;n.uniformBlockBinding(_,T)}function c(_,w){let T=i[_.id];T===void 0&&(m(_),T=h(_),i[_.id]=T,_.addEventListener("dispose",v));let L=w.program;n.updateUBOMapping(_,L);let y=e.render.frame;r[_.id]!==y&&(d(_),r[_.id]=y)}function h(_){let w=u();_.__bindingPointIndex=w;let T=s.createBuffer(),L=_.__size,y=_.usage;return s.bindBuffer(s.UNIFORM_BUFFER,T),s.bufferData(s.UNIFORM_BUFFER,L,y),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,w,T),T}function u(){for(let _=0;_<a;_++)if(o.indexOf(_)===-1)return o.push(_),_;return Qe("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function d(_){let w=i[_.id],T=_.uniforms,L=_.__cache;s.bindBuffer(s.UNIFORM_BUFFER,w);for(let y=0,I=T.length;y<I;y++){let E=T[y];if(Array.isArray(E))for(let R=0,F=E.length;R<F;R++)f(E[R],y,R,L);else f(E,y,0,L)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function f(_,w,T,L){if(M(_,w,T,L)===!0){let y=_.__offset,I=_.value;if(Array.isArray(I)){let E=0;for(let R=0;R<I.length;R++){let F=I[R],X=p(F);g(F,_.__data,E),typeof F!="number"&&typeof F!="boolean"&&!F.isMatrix3&&!ArrayBuffer.isView(F)&&(E+=X.storage/Float32Array.BYTES_PER_ELEMENT)}}else g(I,_.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,y,_.__data)}}function g(_,w,T){typeof _=="number"||typeof _=="boolean"?w[0]=_:_.isMatrix3?(w[0]=_.elements[0],w[1]=_.elements[1],w[2]=_.elements[2],w[3]=0,w[4]=_.elements[3],w[5]=_.elements[4],w[6]=_.elements[5],w[7]=0,w[8]=_.elements[6],w[9]=_.elements[7],w[10]=_.elements[8],w[11]=0):ArrayBuffer.isView(_)?w.set(new _.constructor(_.buffer,_.byteOffset,w.length)):_.toArray(w,T)}function M(_,w,T,L){let y=_.value,I=w+"_"+T;if(L[I]===void 0)return typeof y=="number"||typeof y=="boolean"?L[I]=y:ArrayBuffer.isView(y)?L[I]=y.slice():L[I]=y.clone(),!0;{let E=L[I];if(typeof y=="number"||typeof y=="boolean"){if(E!==y)return L[I]=y,!0}else{if(ArrayBuffer.isView(y))return!0;if(E.equals(y)===!1)return E.copy(y),!0}}return!1}function m(_){let w=_.uniforms,T=0,L=16;for(let I=0,E=w.length;I<E;I++){let R=Array.isArray(w[I])?w[I]:[w[I]];for(let F=0,X=R.length;F<X;F++){let P=R[F],N=Array.isArray(P.value)?P.value:[P.value];for(let O=0,A=N.length;O<A;O++){let V=N[O],Y=p(V),G=T%L,ne=G%Y.boundary,le=G+ne;T+=ne,le!==0&&L-le<Y.storage&&(T+=L-le),P.__data=new Float32Array(Y.storage/Float32Array.BYTES_PER_ELEMENT),P.__offset=T,T+=Y.storage}}}let y=T%L;return y>0&&(T+=L-y),_.__size=T,_.__cache={},this}function p(_){let w={boundary:0,storage:0};return typeof _=="number"||typeof _=="boolean"?(w.boundary=4,w.storage=4):_.isVector2?(w.boundary=8,w.storage=8):_.isVector3||_.isColor?(w.boundary=16,w.storage=12):_.isVector4?(w.boundary=16,w.storage=16):_.isMatrix3?(w.boundary=48,w.storage=48):_.isMatrix4?(w.boundary=64,w.storage=64):_.isTexture?Xe("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(_)?(w.boundary=16,w.storage=_.byteLength):Xe("WebGLRenderer: Unsupported uniform value type.",_),w}function v(_){let w=_.target;w.removeEventListener("dispose",v);let T=o.indexOf(w.__bindingPointIndex);o.splice(T,1),s.deleteBuffer(i[w.id]),delete i[w.id],delete r[w.id]}function S(){for(let _ in i)s.deleteBuffer(i[_]);o=[],i={},r={}}return{bind:l,update:c,dispose:S}}var Rv=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),_i=null;function Cv(){return _i===null&&(_i=new mr(Rv,16,16,as,nn),_i.name="DFG_LUT",_i.minFilter=Vt,_i.magFilter=Vt,_i.wrapS=Hn,_i.wrapT=Hn,_i.generateMipmaps=!1,_i.needsUpdate=!0),_i}var nc=class{constructor(e={}){let{canvas:t=tf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:d=!1,outputBufferType:f=Sn}=e;this.isWebGLRenderer=!0;let g;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");g=n.getContextAttributes().alpha}else g=o;let M=f,m=new Set([_l,xl,gl]),p=new Set([Sn,ii,Ar,Rr,fl,pl]),v=new Uint32Array(4),S=new Int32Array(4),_=new z,w=null,T=null,L=[],y=[],I=null;this.domElement=t,this.debug={checkShaderErrors:!0,onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=ti,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let E=this,R=!1,F=null,X=null,P=null,N=null;this._outputColorSpace=Ht;let O=0,A=0,V=null,Y=-1,G=null,ne=new Mt,le=new Mt,Ae=null,Pe=new xe(0),Re=0,oe=t.width,pe=t.height,ce=1,Te=null,ue=null,he=new Mt(0,0,oe,pe),H=new Mt(0,0,oe,pe),Z=!1,W=new gr,J=!1,te=!1,fe=new et,Me=new z,Oe=new Mt,We={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},Ye=!1;function Je(){return V===null?ce:1}let q=n;function Be(D,ee){return t.getContext(D,ee)}try{let D={alpha:!0,depth:i,stencil:r,antialias:a,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"185"}`),t.addEventListener("webglcontextlost",zt,!1),t.addEventListener("webglcontextrestored",Tt,!1),t.addEventListener("webglcontextcreationerror",An,!1),q===null){let ee="webgl2";if(q=Be(ee,D),q===null)throw Be(ee)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}}catch(D){throw Qe("WebGLRenderer: "+D.message),D}let $e,b,x,U,B,C,k,$,j,Q,me,Ie,be,ye,Fe,we,ze,K,_e,de,Se,Ee,ge;function Ve(){$e=new Fx(q),$e.init(),Se=new bv(q,$e),b=new Rx(q,$e,e,Se),x=new yv(q,$e),b.reversedDepthBuffer&&d&&x.buffers.depth.setReversed(!0),X=q.createFramebuffer(),P=q.createFramebuffer(),N=q.createFramebuffer(),U=new zx(q),B=new ov,C=new Mv(q,$e,x,B,b,Se,U),k=new Ux(E),$=new Gm(q),Ee=new wx(q,$),j=new Ox(q,$,U,Ee),Q=new Hx(q,j,$,Ee,U),K=new kx(q,b,C),Fe=new Cx(B),me=new rv(E,k,$e,b,Ee,Fe),Ie=new wv(E,B),be=new lv,ye=new pv($e),ze=new Ex(E,k,x,Q,g,l),we=new vv(E,Q,b),ge=new Av(q,U,b,x),_e=new Ax(q,$e,U),de=new Bx(q,$e,U),U.programs=me.programs,E.capabilities=b,E.extensions=$e,E.properties=B,E.renderLists=be,E.shadowMap=we,E.state=x,E.info=U}Ve(),M!==Sn&&(I=new Gx(M,t.width,t.height,a,i,r));let Ge=new Hh(E,q);this.xr=Ge,this.getContext=function(){return q},this.getContextAttributes=function(){return q.getContextAttributes()},this.forceContextLoss=function(){let D=$e.get("WEBGL_lose_context");D&&D.loseContext()},this.forceContextRestore=function(){let D=$e.get("WEBGL_lose_context");D&&D.restoreContext()},this.getPixelRatio=function(){return ce},this.setPixelRatio=function(D){D!==void 0&&(ce=D,this.setSize(oe,pe,!1))},this.getSize=function(D){return D.set(oe,pe)},this.setSize=function(D,ee,ae=!0){if(Ge.isPresenting){Xe("WebGLRenderer: Can't change size while VR device is presenting.");return}oe=D,pe=ee,t.width=Math.floor(D*ce),t.height=Math.floor(ee*ce),ae===!0&&(t.style.width=D+"px",t.style.height=ee+"px"),I!==null&&I.setSize(t.width,t.height),this.setViewport(0,0,D,ee)},this.getDrawingBufferSize=function(D){return D.set(oe*ce,pe*ce).floor()},this.setDrawingBufferSize=function(D,ee,ae){oe=D,pe=ee,ce=ae,t.width=Math.floor(D*ae),t.height=Math.floor(ee*ae),this.setViewport(0,0,D,ee)},this.setEffects=function(D){if(M===Sn){Qe("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(D){for(let ee=0;ee<D.length;ee++)if(D[ee].isOutputPass===!0){Xe("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}I.setEffects(D||[])},this.getCurrentViewport=function(D){return D.copy(ne)},this.getViewport=function(D){return D.copy(he)},this.setViewport=function(D,ee,ae,se){D.isVector4?he.set(D.x,D.y,D.z,D.w):he.set(D,ee,ae,se),x.viewport(ne.copy(he).multiplyScalar(ce).round())},this.getScissor=function(D){return D.copy(H)},this.setScissor=function(D,ee,ae,se){D.isVector4?H.set(D.x,D.y,D.z,D.w):H.set(D,ee,ae,se),x.scissor(le.copy(H).multiplyScalar(ce).round())},this.getScissorTest=function(){return Z},this.setScissorTest=function(D){x.setScissorTest(Z=D)},this.setOpaqueSort=function(D){Te=D},this.setTransparentSort=function(D){ue=D},this.getClearColor=function(D){return D.copy(ze.getClearColor())},this.setClearColor=function(){ze.setClearColor(...arguments)},this.getClearAlpha=function(){return ze.getClearAlpha()},this.setClearAlpha=function(){ze.setClearAlpha(...arguments)},this.clear=function(D=!0,ee=!0,ae=!0){let se=0;if(D){let re=!1;if(V!==null){let De=V.texture.format;re=m.has(De)}if(re){let De=V.texture.type,He=p.has(De),Le=ze.getClearColor(),qe=ze.getClearAlpha(),Ze=Le.r,st=Le.g,ft=Le.b;He?(v[0]=Ze,v[1]=st,v[2]=ft,v[3]=qe,q.clearBufferuiv(q.COLOR,0,v)):(S[0]=Ze,S[1]=st,S[2]=ft,S[3]=qe,q.clearBufferiv(q.COLOR,0,S))}else se|=q.COLOR_BUFFER_BIT}ee&&(se|=q.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),ae&&(se|=q.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),se!==0&&q.clear(se)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(D){D.setRenderer(this),F=D},this.dispose=function(){t.removeEventListener("webglcontextlost",zt,!1),t.removeEventListener("webglcontextrestored",Tt,!1),t.removeEventListener("webglcontextcreationerror",An,!1),ze.dispose(),be.dispose(),ye.dispose(),B.dispose(),k.dispose(),Q.dispose(),Ee.dispose(),ge.dispose(),me.dispose(),Ge.dispose(),Ge.removeEventListener("sessionstart",at),Ge.removeEventListener("sessionend",ut),Ut.stop()};function zt(D){D.preventDefault(),eo("WebGLRenderer: Context Lost."),R=!0}function Tt(){eo("WebGLRenderer: Context Restored."),R=!1;let D=U.autoReset,ee=we.enabled,ae=we.autoUpdate,se=we.needsUpdate,re=we.type;Ve(),U.autoReset=D,we.enabled=ee,we.autoUpdate=ae,we.needsUpdate=se,we.type=re}function An(D){Qe("WebGLRenderer: A WebGL context could not be created. Reason: ",D.statusMessage)}function On(D){let ee=D.target;ee.removeEventListener("dispose",On),Mc(ee)}function Mc(D){fs(D),B.remove(D)}function fs(D){let ee=B.get(D).programs;ee!==void 0&&(ee.forEach(function(ae){me.releaseProgram(ae)}),D.isShaderMaterial&&me.releaseShaderCache(D))}this.renderBufferDirect=function(D,ee,ae,se,re,De){ee===null&&(ee=We);let He=re.isMesh&&re.matrixWorld.determinantAffine()<0,Le=Si(D,ee,ae,se,re);x.setMaterial(se,He);let qe=ae.index,Ze=1;if(se.wireframe===!0){if(qe=j.getWireframeAttribute(ae),qe===void 0)return;Ze=2}let st=ae.drawRange,ft=ae.attributes.position,Ke=st.start*Ze,At=(st.start+st.count)*Ze;De!==null&&(Ke=Math.max(Ke,De.start*Ze),At=Math.min(At,(De.start+De.count)*Ze)),qe!==null?(Ke=Math.max(Ke,0),At=Math.min(At,qe.count)):ft!=null&&(Ke=Math.max(Ke,0),At=Math.min(At,ft.count));let qt=At-Ke;if(qt<0||qt===1/0)return;Ee.setup(re,se,Le,ae,qe);let Gt,Lt=_e;if(qe!==null&&(Gt=$.get(qe),Lt=de,Lt.setIndex(Gt)),re.isMesh)se.wireframe===!0?(x.setLineWidth(se.wireframeLinewidth*Je()),Lt.setMode(q.LINES)):Lt.setMode(q.TRIANGLES);else if(re.isLine){let cn=se.linewidth;cn===void 0&&(cn=1),x.setLineWidth(cn*Je()),re.isLineSegments?Lt.setMode(q.LINES):re.isLineLoop?Lt.setMode(q.LINE_LOOP):Lt.setMode(q.LINE_STRIP)}else re.isPoints?Lt.setMode(q.POINTS):re.isSprite&&Lt.setMode(q.TRIANGLES);if(re.isBatchedMesh)if($e.get("WEBGL_multi_draw"))Lt.renderMultiDraw(re._multiDrawStarts,re._multiDrawCounts,re._multiDrawCount);else{let cn=re._multiDrawStarts,ke=re._multiDrawCounts,Rn=re._multiDrawCount,_t=qe?$.get(qe).bytesPerElement:1,Bn=B.get(se).currentProgram.getUniforms();for(let ai=0;ai<Rn;ai++)Bn.setValue(q,"_gl_DrawID",ai),Lt.render(cn[ai]/_t,ke[ai])}else if(re.isInstancedMesh)Lt.renderInstances(Ke,qt,re.count);else if(ae.isInstancedBufferGeometry){let cn=ae._maxInstanceCount!==void 0?ae._maxInstanceCount:1/0,ke=Math.min(ae.instanceCount,cn);Lt.renderInstances(Ke,qt,ke)}else Lt.render(Ke,qt)};function ie(D,ee,ae){D.transparent===!0&&D.side===Nt&&D.forceSinglePass===!1?(D.side=tn,D.needsUpdate=!0,It(D,ee,ae),D.side=In,D.needsUpdate=!0,It(D,ee,ae),D.side=Nt):It(D,ee,ae)}this.compile=function(D,ee,ae=null){ae===null&&(ae=D),T=ye.get(ae),T.init(ee),y.push(T),ae.traverseVisible(function(re){re.isLight&&re.layers.test(ee.layers)&&(T.pushLight(re),re.castShadow&&T.pushShadow(re))}),D!==ae&&D.traverseVisible(function(re){re.isLight&&re.layers.test(ee.layers)&&(T.pushLight(re),re.castShadow&&T.pushShadow(re))}),T.setupLights();let se=new Set;return D.traverse(function(re){if(!(re.isMesh||re.isPoints||re.isLine||re.isSprite))return;let De=re.material;if(De)if(Array.isArray(De))for(let He=0;He<De.length;He++){let Le=De[He];ie(Le,ae,re),se.add(Le)}else ie(De,ae,re),se.add(De)}),T=y.pop(),se},this.compileAsync=function(D,ee,ae=null){let se=this.compile(D,ee,ae);return new Promise(re=>{function De(){if(se.forEach(function(He){B.get(He).currentProgram.isReady()&&se.delete(He)}),se.size===0){re(D);return}setTimeout(De,10)}$e.get("KHR_parallel_shader_compile")!==null?De():setTimeout(De,10)})};let Ne=null;function it(D){Ne&&Ne(D)}function at(){Ut.stop()}function ut(){Ut.start()}let Ut=new Pf;Ut.setAnimationLoop(it),typeof self<"u"&&Ut.setContext(self),this.setAnimationLoop=function(D){Ne=D,Ge.setAnimationLoop(D),D===null?Ut.stop():Ut.start()},Ge.addEventListener("sessionstart",at),Ge.addEventListener("sessionend",ut),this.render=function(D,ee){if(ee!==void 0&&ee.isCamera!==!0){Qe("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(R===!0)return;F!==null&&F.renderStart(D,ee);let ae=Ge.enabled===!0&&Ge.isPresenting===!0,se=I!==null&&(V===null||ae)&&I.begin(E,V);if(D.matrixWorldAutoUpdate===!0&&D.updateMatrixWorld(),ee.parent===null&&ee.matrixWorldAutoUpdate===!0&&ee.updateMatrixWorld(),Ge.enabled===!0&&Ge.isPresenting===!0&&(I===null||I.isCompositing()===!1)&&(Ge.cameraAutoUpdate===!0&&Ge.updateCamera(ee),ee=Ge.getCamera()),D.isScene===!0&&D.onBeforeRender(E,D,ee,V),T=ye.get(D,y.length),T.init(ee),T.state.textureUnits=C.getTextureUnits(),y.push(T),fe.multiplyMatrices(ee.projectionMatrix,ee.matrixWorldInverse),W.setFromProjectionMatrix(fe,Jn,ee.reversedDepth),te=this.localClippingEnabled,J=Fe.init(this.clippingPlanes,te),w=be.get(D,L.length),w.init(),L.push(w),Ge.enabled===!0&&Ge.isPresenting===!0){let He=E.xr.getDepthSensingMesh();He!==null&&wt(He,ee,-1/0,E.sortObjects)}wt(D,ee,0,E.sortObjects),w.finish(),E.sortObjects===!0&&w.sort(Te,ue,ee.reversedDepth),Ye=Ge.enabled===!1||Ge.isPresenting===!1||Ge.hasDepthSensing()===!1,Ye&&ze.addToRenderList(w,D),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),J===!0&&Fe.beginShadows();let re=T.state.shadowsArray;if(we.render(re,D,ee),J===!0&&Fe.endShadows(),(se&&I.hasRenderPass())===!1){let He=w.opaque,Le=w.transmissive;if(T.setupLights(),ee.isArrayCamera){let qe=ee.cameras;if(Le.length>0)for(let Ze=0,st=qe.length;Ze<st;Ze++){let ft=qe[Ze];vt(He,Le,D,ft)}Ye&&ze.render(D);for(let Ze=0,st=qe.length;Ze<st;Ze++){let ft=qe[Ze];Bt(w,D,ft,ft.viewport)}}else Le.length>0&&vt(He,Le,D,ee),Ye&&ze.render(D),Bt(w,D,ee)}V!==null&&A===0&&(C.updateMultisampleRenderTarget(V),C.updateRenderTargetMipmap(V)),se&&I.end(E),D.isScene===!0&&D.onAfterRender(E,D,ee),Ee.resetDefaultState(),Y=-1,G=null,y.pop(),y.length>0?(T=y[y.length-1],C.setTextureUnits(T.state.textureUnits),J===!0&&Fe.setGlobalState(E.clippingPlanes,T.state.camera)):T=null,L.pop(),L.length>0?w=L[L.length-1]:w=null,F!==null&&F.renderEnd()};function wt(D,ee,ae,se){if(D.visible===!1)return;if(D.layers.test(ee.layers)){if(D.isGroup)ae=D.renderOrder;else if(D.isLOD)D.autoUpdate===!0&&D.update(ee);else if(D.isLightProbeGrid)T.pushLightProbeGrid(D);else if(D.isLight)T.pushLight(D),D.castShadow&&T.pushShadow(D);else if(D.isSprite){if(!D.frustumCulled||W.intersectsSprite(D)){se&&Oe.setFromMatrixPosition(D.matrixWorld).applyMatrix4(fe);let He=Q.update(D),Le=D.material;Le.visible&&w.push(D,He,Le,ae,Oe.z,null)}}else if((D.isMesh||D.isLine||D.isPoints)&&(!D.frustumCulled||W.intersectsObject(D))){let He=Q.update(D),Le=D.material;if(se&&(D.boundingSphere!==void 0?(D.boundingSphere===null&&D.computeBoundingSphere(),Oe.copy(D.boundingSphere.center)):(He.boundingSphere===null&&He.computeBoundingSphere(),Oe.copy(He.boundingSphere.center)),Oe.applyMatrix4(D.matrixWorld).applyMatrix4(fe)),Array.isArray(Le)){let qe=He.groups;for(let Ze=0,st=qe.length;Ze<st;Ze++){let ft=qe[Ze],Ke=Le[ft.materialIndex];Ke&&Ke.visible&&w.push(D,He,Ke,ae,Oe.z,ft)}}else Le.visible&&w.push(D,He,Le,ae,Oe.z,null)}}let De=D.children;for(let He=0,Le=De.length;He<Le;He++)wt(De[He],ee,ae,se)}function Bt(D,ee,ae,se){let{opaque:re,transmissive:De,transparent:He}=D;T.setupLightsView(ae),J===!0&&Fe.setGlobalState(E.clippingPlanes,ae),se&&x.viewport(ne.copy(se)),re.length>0&&Ue(re,ee,ae),De.length>0&&Ue(De,ee,ae),He.length>0&&Ue(He,ee,ae),x.buffers.depth.setTest(!0),x.buffers.depth.setMask(!0),x.buffers.color.setMask(!0),x.setPolygonOffset(!1)}function vt(D,ee,ae,se){if((ae.isScene===!0?ae.overrideMaterial:null)!==null)return;if(T.state.transmissionRenderTarget[se.id]===void 0){let Ke=$e.has("EXT_color_buffer_half_float")||$e.has("EXT_color_buffer_float");T.state.transmissionRenderTarget[se.id]=new Wt(1,1,{generateMipmaps:!0,type:Ke?nn:Sn,minFilter:ni,samples:Math.max(4,b.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,colorSpace:rt.workingColorSpace})}let De=T.state.transmissionRenderTarget[se.id],He=se.viewport||ne;De.setSize(He.z*E.transmissionResolutionScale,He.w*E.transmissionResolutionScale);let Le=E.getRenderTarget(),qe=E.getActiveCubeFace(),Ze=E.getActiveMipmapLevel();E.setRenderTarget(De),E.getClearColor(Pe),Re=E.getClearAlpha(),Re<1&&E.setClearColor(16777215,.5),E.clear(),Ye&&ze.render(ae);let st=E.toneMapping;E.toneMapping=ti;let ft=se.viewport;if(se.viewport!==void 0&&(se.viewport=void 0),T.setupLightsView(se),J===!0&&Fe.setGlobalState(E.clippingPlanes,se),Ue(D,ae,se),C.updateMultisampleRenderTarget(De),C.updateRenderTargetMipmap(De),$e.has("WEBGL_multisampled_render_to_texture")===!1){let Ke=!1;for(let At=0,qt=ee.length;At<qt;At++){let Gt=ee[At],{object:Lt,geometry:cn,material:ke,group:Rn}=Gt;if(ke.side===Nt&&Lt.layers.test(se.layers)){let _t=ke.side;ke.side=tn,ke.needsUpdate=!0,Ft(Lt,ae,se,cn,ke,Rn),ke.side=_t,ke.needsUpdate=!0,Ke=!0}}Ke===!0&&(C.updateMultisampleRenderTarget(De),C.updateRenderTargetMipmap(De))}E.setRenderTarget(Le,qe,Ze),E.setClearColor(Pe,Re),ft!==void 0&&(se.viewport=ft),E.toneMapping=st}function Ue(D,ee,ae){let se=ee.isScene===!0?ee.overrideMaterial:null;for(let re=0,De=D.length;re<De;re++){let He=D[re],{object:Le,geometry:qe,group:Ze}=He,st=He.material;st.allowOverride===!0&&se!==null&&(st=se),Le.layers.test(ae.layers)&&Ft(Le,ee,ae,qe,st,Ze)}}function Ft(D,ee,ae,se,re,De){D.onBeforeRender(E,ee,ae,se,re,De),D.modelViewMatrix.multiplyMatrices(ae.matrixWorldInverse,D.matrixWorld),D.normalMatrix.getNormalMatrix(D.modelViewMatrix),re.onBeforeRender(E,ee,ae,se,D,De),re.transparent===!0&&re.side===Nt&&re.forceSinglePass===!1?(re.side=tn,re.needsUpdate=!0,E.renderBufferDirect(ae,ee,se,re,D,De),re.side=In,re.needsUpdate=!0,E.renderBufferDirect(ae,ee,se,re,D,De),re.side=Nt):E.renderBufferDirect(ae,ee,se,re,D,De),D.onAfterRender(E,ee,ae,se,re,De)}function It(D,ee,ae){ee.isScene!==!0&&(ee=We);let se=B.get(D),re=T.state.lights,De=T.state.shadowsArray,He=re.state.version,Le=me.getParameters(D,re.state,De,ee,ae,T.state.lightProbeGridArray),qe=me.getProgramCacheKey(Le),Ze=se.programs;se.environment=D.isMeshStandardMaterial||D.isMeshLambertMaterial||D.isMeshPhongMaterial?ee.environment:null,se.fog=ee.fog;let st=D.isMeshStandardMaterial||D.isMeshLambertMaterial&&!D.envMap||D.isMeshPhongMaterial&&!D.envMap;se.envMap=k.get(D.envMap||se.environment,st),se.envMapRotation=se.environment!==null&&D.envMap===null?ee.environmentRotation:D.envMapRotation,Ze===void 0&&(D.addEventListener("dispose",On),Ze=new Map,se.programs=Ze);let ft=Ze.get(qe);if(ft!==void 0){if(se.currentProgram===ft&&se.lightsStateVersion===He)return kt(D,Le),ft}else Le.uniforms=me.getUniforms(D),F!==null&&D.isNodeMaterial&&F.build(D,ae,Le),D.onBeforeCompile(Le,E),ft=me.acquireProgram(Le,qe),Ze.set(qe,ft),se.uniforms=Le.uniforms;let Ke=se.uniforms;return(!D.isShaderMaterial&&!D.isRawShaderMaterial||D.clipping===!0)&&(Ke.clippingPlanes=Fe.uniform),kt(D,Le),se.needsLights=Ip(D),se.lightsStateVersion=He,se.needsLights&&(Ke.ambientLightColor.value=re.state.ambient,Ke.lightProbe.value=re.state.probe,Ke.directionalLights.value=re.state.directional,Ke.directionalLightShadows.value=re.state.directionalShadow,Ke.spotLights.value=re.state.spot,Ke.spotLightShadows.value=re.state.spotShadow,Ke.rectAreaLights.value=re.state.rectArea,Ke.ltc_1.value=re.state.rectAreaLTC1,Ke.ltc_2.value=re.state.rectAreaLTC2,Ke.pointLights.value=re.state.point,Ke.pointLightShadows.value=re.state.pointShadow,Ke.hemisphereLights.value=re.state.hemi,Ke.directionalShadowMatrix.value=re.state.directionalShadowMatrix,Ke.spotLightMatrix.value=re.state.spotLightMatrix,Ke.spotLightMap.value=re.state.spotLightMap,Ke.pointShadowMatrix.value=re.state.pointShadowMatrix),se.lightProbeGrid=T.state.lightProbeGridArray.length>0,se.currentProgram=ft,se.uniformsList=null,ft}function dt(D){if(D.uniformsList===null){let ee=D.currentProgram.getUniforms();D.uniformsList=Ir.seqWithValue(ee.seq,D.uniforms)}return D.uniformsList}function kt(D,ee){let ae=B.get(D);ae.outputColorSpace=ee.outputColorSpace,ae.batching=ee.batching,ae.batchingColor=ee.batchingColor,ae.instancing=ee.instancing,ae.instancingColor=ee.instancingColor,ae.instancingMorph=ee.instancingMorph,ae.skinning=ee.skinning,ae.morphTargets=ee.morphTargets,ae.morphNormals=ee.morphNormals,ae.morphColors=ee.morphColors,ae.morphTargetsCount=ee.morphTargetsCount,ae.numClippingPlanes=ee.numClippingPlanes,ae.numIntersection=ee.numClipIntersection,ae.vertexAlphas=ee.vertexAlphas,ae.vertexTangents=ee.vertexTangents,ae.toneMapping=ee.toneMapping}function $o(D,ee){if(D.length===0)return null;if(D.length===1)return D[0].texture!==null?D[0]:null;_.setFromMatrixPosition(ee.matrixWorld);for(let ae=0,se=D.length;ae<se;ae++){let re=D[ae];if(re.texture!==null&&re.boundingBox.containsPoint(_))return re}return null}function Si(D,ee,ae,se,re){ee.isScene!==!0&&(ee=We),C.resetTextureUnits();let De=ee.fog,He=se.isMeshStandardMaterial||se.isMeshLambertMaterial||se.isMeshPhongMaterial?ee.environment:null,Le=V===null?E.outputColorSpace:V.isXRRenderTarget===!0?V.texture.colorSpace:rt.workingColorSpace,qe=se.isMeshStandardMaterial||se.isMeshLambertMaterial&&!se.envMap||se.isMeshPhongMaterial&&!se.envMap,Ze=k.get(se.envMap||He,qe),st=se.vertexColors===!0&&!!ae.attributes.color&&ae.attributes.color.itemSize===4,ft=!!ae.attributes.tangent&&(!!se.normalMap||se.anisotropy>0),Ke=!!ae.morphAttributes.position,At=!!ae.morphAttributes.normal,qt=!!ae.morphAttributes.color,Gt=ti;se.toneMapped&&(V===null||V.isXRRenderTarget===!0)&&(Gt=E.toneMapping);let Lt=ae.morphAttributes.position||ae.morphAttributes.normal||ae.morphAttributes.color,cn=Lt!==void 0?Lt.length:0,ke=B.get(se),Rn=T.state.lights;if(J===!0&&(te===!0||D!==G)){let Ot=D===G&&se.id===Y;Fe.setState(se,D,Ot)}let _t=!1;se.version===ke.__version?(ke.needsLights&&ke.lightsStateVersion!==Rn.state.version||ke.outputColorSpace!==Le||re.isBatchedMesh&&ke.batching===!1||!re.isBatchedMesh&&ke.batching===!0||re.isBatchedMesh&&ke.batchingColor===!0&&re.colorTexture===null||re.isBatchedMesh&&ke.batchingColor===!1&&re.colorTexture!==null||re.isInstancedMesh&&ke.instancing===!1||!re.isInstancedMesh&&ke.instancing===!0||re.isSkinnedMesh&&ke.skinning===!1||!re.isSkinnedMesh&&ke.skinning===!0||re.isInstancedMesh&&ke.instancingColor===!0&&re.instanceColor===null||re.isInstancedMesh&&ke.instancingColor===!1&&re.instanceColor!==null||re.isInstancedMesh&&ke.instancingMorph===!0&&re.morphTexture===null||re.isInstancedMesh&&ke.instancingMorph===!1&&re.morphTexture!==null||ke.envMap!==Ze||se.fog===!0&&ke.fog!==De||ke.numClippingPlanes!==void 0&&(ke.numClippingPlanes!==Fe.numPlanes||ke.numIntersection!==Fe.numIntersection)||ke.vertexAlphas!==st||ke.vertexTangents!==ft||ke.morphTargets!==Ke||ke.morphNormals!==At||ke.morphColors!==qt||ke.toneMapping!==Gt||ke.morphTargetsCount!==cn||!!ke.lightProbeGrid!=T.state.lightProbeGridArray.length>0)&&(_t=!0):(_t=!0,ke.__version=se.version);let Bn=ke.currentProgram;_t===!0&&(Bn=It(se,ee,re),F&&se.isNodeMaterial&&F.onUpdateProgram(se,Bn,ke));let ai=!1,Hi=!1,Vs=!1,Dt=Bn.getUniforms(),Yt=ke.uniforms;if(x.useProgram(Bn.program)&&(ai=!0,Hi=!0,Vs=!0),se.id!==Y&&(Y=se.id,Hi=!0),ke.needsLights){let Ot=$o(T.state.lightProbeGridArray,re);ke.lightProbeGrid!==Ot&&(ke.lightProbeGrid=Ot,Hi=!0)}if(ai||G!==D){x.buffers.depth.getReversed()&&D.reversedDepth!==!0&&(D._reversedDepth=!0,D.updateProjectionMatrix()),Dt.setValue(q,"projectionMatrix",D.projectionMatrix),Dt.setValue(q,"viewMatrix",D.matrixWorldInverse);let Gi=Dt.map.cameraPosition;Gi!==void 0&&Gi.setValue(q,Me.setFromMatrixPosition(D.matrixWorld)),b.logarithmicDepthBuffer&&Dt.setValue(q,"logDepthBufFC",2/(Math.log(D.far+1)/Math.LN2)),(se.isMeshPhongMaterial||se.isMeshToonMaterial||se.isMeshLambertMaterial||se.isMeshBasicMaterial||se.isMeshStandardMaterial||se.isShaderMaterial)&&Dt.setValue(q,"isOrthographic",D.isOrthographicCamera===!0),G!==D&&(G=D,Hi=!0,Vs=!0)}if(ke.needsLights&&(Rn.state.directionalShadowMap.length>0&&Dt.setValue(q,"directionalShadowMap",Rn.state.directionalShadowMap,C),Rn.state.spotShadowMap.length>0&&Dt.setValue(q,"spotShadowMap",Rn.state.spotShadowMap,C),Rn.state.pointShadowMap.length>0&&Dt.setValue(q,"pointShadowMap",Rn.state.pointShadowMap,C)),re.isSkinnedMesh){Dt.setOptional(q,re,"bindMatrix"),Dt.setOptional(q,re,"bindMatrixInverse");let Ot=re.skeleton;Ot&&(Ot.boneTexture===null&&Ot.computeBoneTexture(),Dt.setValue(q,"boneTexture",Ot.boneTexture,C))}re.isBatchedMesh&&(Dt.setOptional(q,re,"batchingTexture"),Dt.setValue(q,"batchingTexture",re._matricesTexture,C),Dt.setOptional(q,re,"batchingIdTexture"),Dt.setValue(q,"batchingIdTexture",re._indirectTexture,C),Dt.setOptional(q,re,"batchingColorTexture"),re._colorsTexture!==null&&Dt.setValue(q,"batchingColorTexture",re._colorsTexture,C));let Vi=ae.morphAttributes;if((Vi.position!==void 0||Vi.normal!==void 0||Vi.color!==void 0)&&K.update(re,ae,Bn),(Hi||ke.receiveShadow!==re.receiveShadow)&&(ke.receiveShadow=re.receiveShadow,Dt.setValue(q,"receiveShadow",re.receiveShadow)),(se.isMeshStandardMaterial||se.isMeshLambertMaterial||se.isMeshPhongMaterial)&&se.envMap===null&&ee.environment!==null&&(Yt.envMapIntensity.value=ee.environmentIntensity),Yt.dfgLUT!==void 0&&(Yt.dfgLUT.value=Cv()),Hi){if(Dt.setValue(q,"toneMappingExposure",E.toneMappingExposure),ke.needsLights&&Pp(Yt,Vs),De&&se.fog===!0&&Ie.refreshFogUniforms(Yt,De),Ie.refreshMaterialUniforms(Yt,se,ce,pe,T.state.transmissionRenderTarget[D.id]),ke.needsLights&&ke.lightProbeGrid){let Ot=ke.lightProbeGrid;Yt.probesSH.value=Ot.texture,Yt.probesMin.value.copy(Ot.boundingBox.min),Yt.probesMax.value.copy(Ot.boundingBox.max),Yt.probesResolution.value.copy(Ot.resolution)}Ir.upload(q,dt(ke),Yt,C)}if(se.isShaderMaterial&&se.uniformsNeedUpdate===!0&&(Ir.upload(q,dt(ke),Yt,C),se.uniformsNeedUpdate=!1),se.isSpriteMaterial&&Dt.setValue(q,"center",re.center),Dt.setValue(q,"modelViewMatrix",re.modelViewMatrix),Dt.setValue(q,"normalMatrix",re.normalMatrix),Dt.setValue(q,"modelMatrix",re.matrixWorld),se.uniformsGroups!==void 0){let Ot=se.uniformsGroups;for(let Gi=0,Gs=Ot.length;Gi<Gs;Gi++){let Lu=Ot[Gi];ge.update(Lu,Bn),ge.bind(Lu,Bn)}}return Bn}function Pp(D,ee){D.ambientLightColor.needsUpdate=ee,D.lightProbe.needsUpdate=ee,D.directionalLights.needsUpdate=ee,D.directionalLightShadows.needsUpdate=ee,D.pointLights.needsUpdate=ee,D.pointLightShadows.needsUpdate=ee,D.spotLights.needsUpdate=ee,D.spotLightShadows.needsUpdate=ee,D.rectAreaLights.needsUpdate=ee,D.hemisphereLights.needsUpdate=ee}function Ip(D){return D.isMeshLambertMaterial||D.isMeshToonMaterial||D.isMeshPhongMaterial||D.isMeshStandardMaterial||D.isShadowMaterial||D.isShaderMaterial&&D.lights===!0}this.getActiveCubeFace=function(){return O},this.getActiveMipmapLevel=function(){return A},this.getRenderTarget=function(){return V},this.setRenderTargetTextures=function(D,ee,ae){let se=B.get(D);se.__autoAllocateDepthBuffer=D.resolveDepthBuffer===!1,se.__autoAllocateDepthBuffer===!1&&(se.__useRenderToTexture=!1),B.get(D.texture).__webglTexture=ee,B.get(D.depthTexture).__webglTexture=se.__autoAllocateDepthBuffer?void 0:ae,se.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(D,ee){let ae=B.get(D);ae.__webglFramebuffer=ee,ae.__useDefaultFramebuffer=ee===void 0},this.setRenderTarget=function(D,ee=0,ae=0){V=D,O=ee,A=ae;let se=null,re=!1,De=!1;if(D){let Le=B.get(D);if(Le.__useDefaultFramebuffer!==void 0){x.bindFramebuffer(q.FRAMEBUFFER,Le.__webglFramebuffer),ne.copy(D.viewport),le.copy(D.scissor),Ae=D.scissorTest,x.viewport(ne),x.scissor(le),x.setScissorTest(Ae),Y=-1;return}else if(Le.__webglFramebuffer===void 0)C.setupRenderTarget(D);else if(Le.__hasExternalTextures)C.rebindTextures(D,B.get(D.texture).__webglTexture,B.get(D.depthTexture).__webglTexture);else if(D.depthBuffer){let st=D.depthTexture;if(Le.__boundDepthTexture!==st){if(st!==null&&B.has(st)&&(D.width!==st.image.width||D.height!==st.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");C.setupDepthRenderbuffer(D)}}let qe=D.texture;(qe.isData3DTexture||qe.isDataArrayTexture||qe.isCompressedArrayTexture)&&(De=!0);let Ze=B.get(D).__webglFramebuffer;D.isWebGLCubeRenderTarget?(Array.isArray(Ze[ee])?se=Ze[ee][ae]:se=Ze[ee],re=!0):D.samples>0&&C.useMultisampledRTT(D)===!1?se=B.get(D).__webglMultisampledFramebuffer:Array.isArray(Ze)?se=Ze[ae]:se=Ze,ne.copy(D.viewport),le.copy(D.scissor),Ae=D.scissorTest}else ne.copy(he).multiplyScalar(ce).floor(),le.copy(H).multiplyScalar(ce).floor(),Ae=Z;if(ae!==0&&(se=X),x.bindFramebuffer(q.FRAMEBUFFER,se)&&x.drawBuffers(D,se),x.viewport(ne),x.scissor(le),x.setScissorTest(Ae),re){let Le=B.get(D.texture);q.framebufferTexture2D(q.FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_CUBE_MAP_POSITIVE_X+ee,Le.__webglTexture,ae)}else if(De){let Le=ee;for(let qe=0;qe<D.textures.length;qe++){let Ze=B.get(D.textures[qe]);q.framebufferTextureLayer(q.FRAMEBUFFER,q.COLOR_ATTACHMENT0+qe,Ze.__webglTexture,ae,Le)}}else if(D!==null&&ae!==0){let Le=B.get(D.texture);q.framebufferTexture2D(q.FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_2D,Le.__webglTexture,ae)}Y=-1},this.readRenderTargetPixels=function(D,ee,ae,se,re,De,He,Le=0){if(!(D&&D.isWebGLRenderTarget)){Qe("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let qe=B.get(D).__webglFramebuffer;if(D.isWebGLCubeRenderTarget&&He!==void 0&&(qe=qe[He]),qe){x.bindFramebuffer(q.FRAMEBUFFER,qe);try{let Ze=D.textures[Le],st=Ze.format,ft=Ze.type;if(D.textures.length>1&&q.readBuffer(q.COLOR_ATTACHMENT0+Le),!b.textureFormatReadable(st)){Qe("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(!b.textureTypeReadable(ft)){Qe("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}ee>=0&&ee<=D.width-se&&ae>=0&&ae<=D.height-re&&q.readPixels(ee,ae,se,re,Se.convert(st),Se.convert(ft),De)}finally{let Ze=V!==null?B.get(V).__webglFramebuffer:null;x.bindFramebuffer(q.FRAMEBUFFER,Ze)}}},this.readRenderTargetPixelsAsync=async function(D,ee,ae,se,re,De,He,Le=0){if(!(D&&D.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let qe=B.get(D).__webglFramebuffer;if(D.isWebGLCubeRenderTarget&&He!==void 0&&(qe=qe[He]),qe)if(ee>=0&&ee<=D.width-se&&ae>=0&&ae<=D.height-re){x.bindFramebuffer(q.FRAMEBUFFER,qe);let Ze=D.textures[Le],st=Ze.format,ft=Ze.type;if(D.textures.length>1&&q.readBuffer(q.COLOR_ATTACHMENT0+Le),!b.textureFormatReadable(st))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(!b.textureTypeReadable(ft))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let Ke=q.createBuffer();q.bindBuffer(q.PIXEL_PACK_BUFFER,Ke),q.bufferData(q.PIXEL_PACK_BUFFER,De.byteLength,q.STREAM_READ),q.readPixels(ee,ae,se,re,Se.convert(st),Se.convert(ft),0);let At=V!==null?B.get(V).__webglFramebuffer:null;x.bindFramebuffer(q.FRAMEBUFFER,At);let qt=q.fenceSync(q.SYNC_GPU_COMMANDS_COMPLETE,0);return q.flush(),await sf(q,qt,4),q.bindBuffer(q.PIXEL_PACK_BUFFER,Ke),q.getBufferSubData(q.PIXEL_PACK_BUFFER,0,De),q.deleteBuffer(Ke),q.deleteSync(qt),De}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(D,ee=null,ae=0){let se=Math.pow(2,-ae),re=Math.floor(D.image.width*se),De=Math.floor(D.image.height*se),He=ee!==null?ee.x:0,Le=ee!==null?ee.y:0;C.setTexture2D(D,0),q.copyTexSubImage2D(q.TEXTURE_2D,ae,0,0,He,Le,re,De),x.unbindTexture()},this.copyTextureToTexture=function(D,ee,ae=null,se=null,re=0,De=0){let He,Le,qe,Ze,st,ft,Ke,At,qt,Gt=D.isCompressedTexture?D.mipmaps[De]:D.image;if(ae!==null)He=ae.max.x-ae.min.x,Le=ae.max.y-ae.min.y,qe=ae.isBox3?ae.max.z-ae.min.z:1,Ze=ae.min.x,st=ae.min.y,ft=ae.isBox3?ae.min.z:0;else{let Yt=Math.pow(2,-re);He=Math.floor(Gt.width*Yt),Le=Math.floor(Gt.height*Yt),D.isDataArrayTexture?qe=Gt.depth:D.isData3DTexture?qe=Math.floor(Gt.depth*Yt):qe=1,Ze=0,st=0,ft=0}se!==null?(Ke=se.x,At=se.y,qt=se.z):(Ke=0,At=0,qt=0);let Lt=Se.convert(ee.format),cn=Se.convert(ee.type),ke;ee.isData3DTexture?(C.setTexture3D(ee,0),ke=q.TEXTURE_3D):ee.isDataArrayTexture||ee.isCompressedArrayTexture?(C.setTexture2DArray(ee,0),ke=q.TEXTURE_2D_ARRAY):(C.setTexture2D(ee,0),ke=q.TEXTURE_2D),x.activeTexture(q.TEXTURE0),x.pixelStorei(q.UNPACK_FLIP_Y_WEBGL,ee.flipY),x.pixelStorei(q.UNPACK_PREMULTIPLY_ALPHA_WEBGL,ee.premultiplyAlpha),x.pixelStorei(q.UNPACK_ALIGNMENT,ee.unpackAlignment);let Rn=x.getParameter(q.UNPACK_ROW_LENGTH),_t=x.getParameter(q.UNPACK_IMAGE_HEIGHT),Bn=x.getParameter(q.UNPACK_SKIP_PIXELS),ai=x.getParameter(q.UNPACK_SKIP_ROWS),Hi=x.getParameter(q.UNPACK_SKIP_IMAGES);x.pixelStorei(q.UNPACK_ROW_LENGTH,Gt.width),x.pixelStorei(q.UNPACK_IMAGE_HEIGHT,Gt.height),x.pixelStorei(q.UNPACK_SKIP_PIXELS,Ze),x.pixelStorei(q.UNPACK_SKIP_ROWS,st),x.pixelStorei(q.UNPACK_SKIP_IMAGES,ft);let Vs=D.isDataArrayTexture||D.isData3DTexture,Dt=ee.isDataArrayTexture||ee.isData3DTexture;if(D.isDepthTexture){let Yt=B.get(D),Vi=B.get(ee),Ot=B.get(Yt.__renderTarget),Gi=B.get(Vi.__renderTarget);x.bindFramebuffer(q.READ_FRAMEBUFFER,Ot.__webglFramebuffer),x.bindFramebuffer(q.DRAW_FRAMEBUFFER,Gi.__webglFramebuffer);for(let Gs=0;Gs<qe;Gs++)Vs&&(q.framebufferTextureLayer(q.READ_FRAMEBUFFER,q.COLOR_ATTACHMENT0,B.get(D).__webglTexture,re,ft+Gs),q.framebufferTextureLayer(q.DRAW_FRAMEBUFFER,q.COLOR_ATTACHMENT0,B.get(ee).__webglTexture,De,qt+Gs)),q.blitFramebuffer(Ze,st,He,Le,Ke,At,He,Le,q.DEPTH_BUFFER_BIT,q.NEAREST);x.bindFramebuffer(q.READ_FRAMEBUFFER,null),x.bindFramebuffer(q.DRAW_FRAMEBUFFER,null)}else if(re!==0||D.isRenderTargetTexture||B.has(D)){let Yt=B.get(D),Vi=B.get(ee);x.bindFramebuffer(q.READ_FRAMEBUFFER,P),x.bindFramebuffer(q.DRAW_FRAMEBUFFER,N);for(let Ot=0;Ot<qe;Ot++)Vs?q.framebufferTextureLayer(q.READ_FRAMEBUFFER,q.COLOR_ATTACHMENT0,Yt.__webglTexture,re,ft+Ot):q.framebufferTexture2D(q.READ_FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_2D,Yt.__webglTexture,re),Dt?q.framebufferTextureLayer(q.DRAW_FRAMEBUFFER,q.COLOR_ATTACHMENT0,Vi.__webglTexture,De,qt+Ot):q.framebufferTexture2D(q.DRAW_FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_2D,Vi.__webglTexture,De),re!==0?q.blitFramebuffer(Ze,st,He,Le,Ke,At,He,Le,q.COLOR_BUFFER_BIT,q.NEAREST):Dt?q.copyTexSubImage3D(ke,De,Ke,At,qt+Ot,Ze,st,He,Le):q.copyTexSubImage2D(ke,De,Ke,At,Ze,st,He,Le);x.bindFramebuffer(q.READ_FRAMEBUFFER,null),x.bindFramebuffer(q.DRAW_FRAMEBUFFER,null)}else Dt?D.isDataTexture||D.isData3DTexture?q.texSubImage3D(ke,De,Ke,At,qt,He,Le,qe,Lt,cn,Gt.data):ee.isCompressedArrayTexture?q.compressedTexSubImage3D(ke,De,Ke,At,qt,He,Le,qe,Lt,Gt.data):q.texSubImage3D(ke,De,Ke,At,qt,He,Le,qe,Lt,cn,Gt):D.isDataTexture?q.texSubImage2D(q.TEXTURE_2D,De,Ke,At,He,Le,Lt,cn,Gt.data):D.isCompressedTexture?q.compressedTexSubImage2D(q.TEXTURE_2D,De,Ke,At,Gt.width,Gt.height,Lt,Gt.data):q.texSubImage2D(q.TEXTURE_2D,De,Ke,At,He,Le,Lt,cn,Gt);x.pixelStorei(q.UNPACK_ROW_LENGTH,Rn),x.pixelStorei(q.UNPACK_IMAGE_HEIGHT,_t),x.pixelStorei(q.UNPACK_SKIP_PIXELS,Bn),x.pixelStorei(q.UNPACK_SKIP_ROWS,ai),x.pixelStorei(q.UNPACK_SKIP_IMAGES,Hi),De===0&&ee.generateMipmaps&&q.generateMipmap(ke),x.unbindTexture()},this.initRenderTarget=function(D){B.get(D).__webglFramebuffer===void 0&&C.setupRenderTarget(D)},this.initTexture=function(D){D.isCubeTexture?C.setTextureCube(D,0):D.isData3DTexture?C.setTexture3D(D,0):D.isDataArrayTexture||D.isCompressedArrayTexture?C.setTexture2DArray(D,0):C.setTexture2D(D,0),x.unbindTexture()},this.resetState=function(){O=0,A=0,V=null,x.reset(),Ee.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return Jn}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=rt._getDrawingBufferColorSpace(e),t.unpackColorSpace=rt._getUnpackColorSpace()}};function Vh(s,e){if(e===vh)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Cr||e===Vo){let t=s.getIndex();if(t===null){let o=[],a=s.getAttribute("position");if(a!==void 0){for(let l=0;l<a.count;l++)o.push(l);s.setIndex(o),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Cr)for(let o=1;o<=n;o++)i.push(t.getX(0)),i.push(t.getX(o)),i.push(t.getX(o+1));else for(let o=0;o<n;o++)o%2===0?(i.push(t.getX(o)),i.push(t.getX(o+1)),i.push(t.getX(o+2))):(i.push(t.getX(o+2)),i.push(t.getX(o+1)),i.push(t.getX(o)));i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles.");let r=s.clone();return r.setIndex(i),r.clearGroups(),r}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function Of(s){let e=new Map,t=new Map,n=s.clone();return Bf(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,o=e.get(i),a=o.skeleton.bones;r.skeleton=o.skeleton.clone(),r.bindMatrix.copy(o.bindMatrix),r.skeleton.bones=a.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function Bf(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)Bf(s.children[n],e.children[n],t)}var hs=class extends gi{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new Kh(t)}),this.register(function(t){return new jh(t)}),this.register(function(t){return new ru(t)}),this.register(function(t){return new ou(t)}),this.register(function(t){return new au(t)}),this.register(function(t){return new $h(t)}),this.register(function(t){return new Qh(t)}),this.register(function(t){return new eu(t)}),this.register(function(t){return new tu(t)}),this.register(function(t){return new Zh(t)}),this.register(function(t){return new nu(t)}),this.register(function(t){return new Jh(t)}),this.register(function(t){return new su(t)}),this.register(function(t){return new iu(t)}),this.register(function(t){return new qh(t)}),this.register(function(t){return new rc(t,gt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new rc(t,gt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new lu(t)})}load(e,t,n,i){let r=this,o;if(this.resourcePath!=="")o=this.resourcePath;else if(this.path!==""){let c=Oi.extractUrlBase(e);o=Oi.resolveURL(c,this.path)}else o=Oi.extractUrlBase(e);this.manager.itemStart(e);let a=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new Sr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,o,function(h){t(h),r.manager.itemEnd(e)},a)}catch(h){a(h)}},n,a)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,o={},a={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===Gf){try{o[gt.KHR_BINARY_GLTF]=new cu(e)}catch(u){i&&i(u);return}r=JSON.parse(o[gt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new gu(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),a[u.name]=u,o[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],d=r.extensionsRequired||[];switch(u){case gt.KHR_MATERIALS_UNLIT:o[u]=new Yh;break;case gt.KHR_DRACO_MESH_COMPRESSION:o[u]=new hu(r,this.dracoLoader);break;case gt.KHR_TEXTURE_TRANSFORM:o[u]=new uu;break;case gt.KHR_MESH_QUANTIZATION:o[u]=new du;break;default:d.indexOf(u)>=0&&a[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(o),c.setPlugins(a),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function Pv(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function jt(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var gt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},qh=class{constructor(e){this.parser=e,this.name=gt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new xe(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],mn);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new ns(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new ei(h),c.distance=u;break;case"spot":c=new bo(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),yi(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],a=(r.extensions&&r.extensions[this.name]||{}).light;return a===void 0?null:this._loadLight(a).then(function(l){return n._getNodeRef(t.cache,a,l)})}},Yh=class{constructor(){this.name=gt.KHR_MATERIALS_UNLIT}getMaterialType(){return bt}extendParams(e,t,n){let i=[];e.color=new xe(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let o=r.baseColorFactor;e.color.setRGB(o[0],o[1],o[2],mn),e.opacity=o[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,Ht))}return Promise.all(i)}},Zh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},Kh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new ve(r,r)}return Promise.all(i)}},jh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},Jh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},$h=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_SHEEN}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new xe(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],mn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,Ht)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},Qh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},eu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_VOLUME}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new xe().setRGB(r[0],r[1],r[2],mn),Promise.all(i)}},tu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_IOR}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},nu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new xe().setRGB(r[0],r[1],r[2],mn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,Ht)),Promise.all(i)}},iu=class{constructor(e){this.parser=e,this.name=gt.EXT_MATERIALS_BUMP}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},su=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},ru=class{constructor(e){this.parser=e,this.name=gt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],o=t.options.ktx2Loader;if(!o){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,o)}},ou=class{constructor(e){this.parser=e,this.name=gt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},au=class{constructor(e){this.parser=e,this.name=gt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},rc=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),o=this.parser.options.meshoptDecoder;if(!o||!o.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(a){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,d=new Uint8Array(a,l,c);return o.decodeGltfBufferAsync?o.decodeGltfBufferAsync(h,u,d,i.mode,i.filter).then(function(f){return f.buffer}):o.ready.then(function(){let f=new ArrayBuffer(h*u);return o.decodeGltfBuffer(new Uint8Array(f),h,u,d,i.mode,i.filter),f})})}else return null}},lu=class{constructor(e){this.name=gt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==Xn.TRIANGLES&&c.mode!==Xn.TRIANGLE_STRIP&&c.mode!==Xn.TRIANGLE_FAN&&c.mode!==void 0)return null;let o=n.extensions[this.name].attributes,a=[],l={};for(let c in o)a.push(this.parser.getDependency("accessor",o[c]).then(h=>(l[c]=h,l[c])));return a.length<1?null:(a.push(this.parser.createNodeMesh(e)),Promise.all(a).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],d=c[0].count,f=[];for(let g of u){let M=new et,m=new z,p=new Zt,v=new z(1,1,1),S=new vn(g.geometry,g.material,d);for(let _=0;_<d;_++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,_),l.ROTATION&&p.fromBufferAttribute(l.ROTATION,_),l.SCALE&&v.fromBufferAttribute(l.SCALE,_),S.setMatrixAt(_,M.compose(m,p,v));for(let _ in l)if(_==="_COLOR_0"){let w=l[_];S.instanceColor=new Qi(w.array,w.itemSize,w.normalized)}else _!=="TRANSLATION"&&_!=="ROTATION"&&_!=="SCALE"&&g.geometry.setAttribute(_,l[_]);St.prototype.copy.call(S,g),this.parser.assignFinalMaterial(S),f.push(S)}return h.isGroup?(h.clear(),h.add(...f),h):f[0]}))}},Gf="glTF",Yo=12,zf={JSON:1313821514,BIN:5130562},cu=class{constructor(e){this.name=gt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,Yo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==Gf)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-Yo,r=new DataView(e,Yo),o=0;for(;o<i;){let a=r.getUint32(o,!0);o+=4;let l=r.getUint32(o,!0);if(o+=4,l===zf.JSON){let c=new Uint8Array(e,Yo+o,a);this.content=n.decode(c)}else if(l===zf.BIN){let c=Yo+o;this.body=e.slice(c,c+a)}o+=a}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},hu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=gt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,o=e.extensions[this.name].attributes,a={},l={},c={};for(let h in o){let u=pu[h]||h.toLowerCase();a[u]=o[h]}for(let h in e.attributes){let u=pu[h]||h.toLowerCase();if(o[h]!==void 0){let d=n.accessors[e.attributes[h]],f=Nr[d.componentType];c[u]=f.name,l[u]=d.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,d){i.decodeDracoFile(h,function(f){for(let g in f.attributes){let M=f.attributes[g],m=l[g];m!==void 0&&(M.normalized=m)}u(f)},a,c,mn,d)})})}},uu=class{constructor(){this.name=gt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){return(t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0||(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),e.needsUpdate=!0),e}},du=class{constructor(){this.name=gt.KHR_MESH_QUANTIZATION}},oc=class extends mi{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let o=0;o!==i;o++)t[o]=n[r+o];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=a*2,c=a*3,h=i-t,u=(n-t)/h,d=u*u,f=d*u,g=e*c,M=g-c,m=-2*f+3*d,p=f-d,v=1-m,S=p-d+u;for(let _=0;_!==a;_++){let w=o[M+_+a],T=o[M+_+l]*h,L=o[g+_+a],y=o[g+_]*h;r[_]=v*w+S*T+m*L+p*y}return r}},Iv=new Zt,fu=class extends oc{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return Iv.fromArray(r).normalize().toArray(r),r}},Xn={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Nr={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},kf={9728:Kt,9729:Vt,9984:ul,9985:wr,9986:Fs,9987:ni},Hf={33071:Hn,33648:rr,10497:$i},Gh={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},pu={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},cs={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},Lv={CUBICSPLINE:void 0,LINEAR:Es,STEP:Ts},Wh={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function Dv(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new yn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:In})),s.DefaultMaterial}function zs(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function yi(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function Nv(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let o=[],a=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let d=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;o.push(d)}if(i){let d=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;a.push(d)}if(r){let d=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(d)}}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],d=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=d),s.morphTargetsRelative=!0,s})}function Uv(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function Fv(s){let e,t=s.extensions&&s.extensions[gt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+Xh(t.attributes):e=s.indices+":"+Xh(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+Xh(s.targets[n]);return e}function Xh(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function mu(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function Ov(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var Bv=new et,gu=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new Pv,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,o=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let a=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(a)===!0;let l=a.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=a.indexOf("Firefox")>-1,o=r?a.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&o<98?this.textureLoader=new _o(this.options.manager):this.textureLoader=new So(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new Sr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(o){return o._markDefs&&o._markDefs()}),Promise.all(this._invokeAll(function(o){return o.beforeRoot&&o.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(o){let a={scene:o[0][i.scene||0],scenes:o[0],animations:o[1],cameras:o[2],asset:i.asset,parser:n,userData:{}};return zs(r,a,i),yi(a,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(a)})).then(function(){for(let l of a.scenes)l.updateMatrixWorld();e(a)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let o=t[i].joints;for(let a=0,l=o.length;a<l;a++)e[o[a]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let o=e[i];o.mesh!==void 0&&(this._addNodeRef(this.meshCache,o.mesh),o.skin!==void 0&&(n[o.mesh].isSkinnedMesh=!0)),o.camera!==void 0&&this._addNodeRef(this.cameraCache,o.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(o,a)=>{let l=this.associations.get(o);l!=null&&this.associations.set(a,l);for(let[c,h]of o.children.entries())r(h,a.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,o){return n.getDependency(e,o)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[gt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,o){n.load(Oi.resolveURL(t.uri,i.path),r,void 0,function(){o(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let o=Gh[i.type],a=Nr[i.componentType],l=i.normalized===!0,c=new a(i.count*o);return Promise.resolve(new xt(c,o,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(o){let a=o[0],l=Gh[i.type],c=Nr[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,d=i.byteOffset||0,f=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,g=i.normalized===!0,M,m;if(f&&f!==u){let p=Math.floor(d/f),v="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+p+":"+i.count,S=t.cache.get(v);S||(M=new c(a,p*f,i.count*f/h),S=new dr(M,f/h),t.cache.add(v,S)),m=new fr(S,l,d%f/h,g)}else a===null?M=new c(i.count*l):M=new c(a,d,i.count*l),m=new xt(M,l,g);if(i.sparse!==void 0){let p=Gh.SCALAR,v=Nr[i.sparse.indices.componentType],S=i.sparse.indices.byteOffset||0,_=i.sparse.values.byteOffset||0,w=new v(o[1],S,i.sparse.count*p),T=new c(o[2],_,i.sparse.count*l);a!==null&&(m=new xt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let L=0,y=w.length;L<y;L++){let I=w[L];if(m.setX(I,T[L*l]),l>=2&&m.setY(I,T[L*l+1]),l>=3&&m.setZ(I,T[L*l+2]),l>=4&&m.setW(I,T[L*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=g}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,o=t.images[r],a=this.textureLoader;if(o.uri){let l=n.manager.getHandler(o.uri);l!==null&&(a=l)}return this.loadTextureImage(e,r,a)}loadTextureImage(e,t,n){let i=this,r=this.json,o=r.textures[e],a=r.images[t],l=(a.uri||a.bufferView)+":"+o.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=o.name||a.name||"",h.name===""&&typeof a.uri=="string"&&a.uri.startsWith("data:image/")===!1&&(h.name=a.uri);let d=(r.samplers||{})[o.sampler]||{};return h.magFilter=kf[d.magFilter]||Vt,h.minFilter=kf[d.minFilter]||ni,h.wrapS=Hf[d.wrapS]||$i,h.wrapT=Hf[d.wrapT]||$i,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==Kt&&h.minFilter!==Vt,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let o=i.images[e],a=self.URL||self.webkitURL,l=o.uri||"",c=!1;if(o.bufferView!==void 0)l=n.getDependency("bufferView",o.bufferView).then(function(u){c=!0;let d=new Blob([u],{type:o.mimeType});return l=a.createObjectURL(d),l});else if(o.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(d,f){let g=d;t.isImageBitmapLoader===!0&&(g=function(M){let m=new Qt(M);m.needsUpdate=!0,d(m)}),t.load(Oi.resolveURL(u,r.path),g,void 0,f)})}).then(function(u){return c===!0&&a.revokeObjectURL(l),yi(u,o),u.userData.mimeType=o.mimeType||Ov(o.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(o){if(!o)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(o=o.clone(),o.channel=n.texCoord),r.extensions[gt.KHR_TEXTURE_TRANSFORM]){let a=n.extensions!==void 0?n.extensions[gt.KHR_TEXTURE_TRANSFORM]:void 0;if(a){let l=r.associations.get(o);o=r.extensions[gt.KHR_TEXTURE_TRANSFORM].extendTexture(o,a),r.associations.set(o,l)}}return i!==void 0&&(o.colorSpace=i),e[t]=o,o})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,o=t.attributes.normal===void 0;if(e.isPoints){let a="PointsMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new xr,gn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(a,l)),n=l}else if(e.isLine){let a="LineBasicMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new di,gn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(a,l)),n=l}if(i||r||o){let a="ClonedMaterial:"+n.uuid+":";i&&(a+="derivative-tangents:"),r&&(a+="vertex-colors:"),o&&(a+="flat-shading:");let l=this.cache.get(a);l||(l=n.clone(),r&&(l.vertexColors=!0),o&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(a,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return yn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],o,a={},l=r.extensions||{},c=[];if(l[gt.KHR_MATERIALS_UNLIT]){let u=i[gt.KHR_MATERIALS_UNLIT];o=u.getMaterialType(),c.push(u.extendParams(a,r,t))}else{let u=r.pbrMetallicRoughness||{};if(a.color=new xe(1,1,1),a.opacity=1,Array.isArray(u.baseColorFactor)){let d=u.baseColorFactor;a.color.setRGB(d[0],d[1],d[2],mn),a.opacity=d[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(a,"map",u.baseColorTexture,Ht)),a.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,a.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(a,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(a,"roughnessMap",u.metallicRoughnessTexture))),o=this._invokeOne(function(d){return d.getMaterialType&&d.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(d){return d.extendMaterialParams&&d.extendMaterialParams(e,a)})))}r.doubleSided===!0&&(a.side=Nt);let h=r.alphaMode||Wh.OPAQUE;if(h===Wh.BLEND?(a.transparent=!0,a.depthWrite=!1):(a.transparent=!1,h===Wh.MASK&&(a.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&o!==bt&&(c.push(t.assignTexture(a,"normalMap",r.normalTexture)),a.normalScale=new ve(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;a.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&o!==bt&&(c.push(t.assignTexture(a,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(a.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&o!==bt){let u=r.emissiveFactor;a.emissive=new xe().setRGB(u[0],u[1],u[2],mn)}return r.emissiveTexture!==void 0&&o!==bt&&c.push(t.assignTexture(a,"emissiveMap",r.emissiveTexture,Ht)),Promise.all(c).then(function(){let u=new o(a);return r.name&&(u.name=r.name),yi(u,r),t.associations.set(u,{materials:e}),r.extensions&&zs(i,u,r),u})}createUniqueName(e){let t=Rt.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(a){return n[gt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(a,t).then(function(l){return Vf(l,a,t)})}let o=[];for(let a=0,l=e.length;a<l;a++){let c=e[a],h=Fv(c),u=i[h];if(u)o.push(u.promise);else{let d;c.extensions&&c.extensions[gt.KHR_DRACO_MESH_COMPRESSION]?d=r(c):d=Vf(new pt,c,t),i[h]={primitive:c,promise:d},o.push(d)}}return Promise.all(o)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],o=r.primitives,a=[];for(let l=0,c=o.length;l<c;l++){let h=o[l].material===void 0?Dv(this.cache):this.getDependency("material",o[l].material);a.push(h)}return a.push(t.loadGeometries(o)),Promise.all(a).then(function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let f=0,g=h.length;f<g;f++){let M=h[f],m=o[f],p,v=c[f];if(m.mode===Xn.TRIANGLES||m.mode===Xn.TRIANGLE_STRIP||m.mode===Xn.TRIANGLE_FAN||m.mode===void 0)p=r.isSkinnedMesh===!0?new ro(M,v):new je(M,v),p.isSkinnedMesh===!0&&p.normalizeSkinWeights(),m.mode===Xn.TRIANGLE_STRIP?p.geometry=Vh(p.geometry,Vo):m.mode===Xn.TRIANGLE_FAN&&(p.geometry=Vh(p.geometry,Cr));else if(m.mode===Xn.LINES)p=new Rs(M,v);else if(m.mode===Xn.LINE_STRIP)p=new Ii(M,v);else if(m.mode===Xn.LINE_LOOP)p=new ao(M,v);else if(m.mode===Xn.POINTS)p=new xn(M,v);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(p.geometry.morphAttributes).length>0&&Uv(p,r),p.name=t.createUniqueName(r.name||"mesh_"+e),yi(p,r),m.extensions&&zs(i,p,m),t.assignFinalMaterial(p),u.push(p)}for(let f=0,g=u.length;f<g;f++)t.associations.set(u[f],{meshes:e,primitives:f});if(u.length===1)return r.extensions&&zs(i,u[0],r),u[0];let d=new lt;r.extensions&&zs(i,d,r),t.associations.set(d,{meshes:e});for(let f=0,g=u.length;f<g;f++)d.add(u[f]);return d})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new $t(Ct.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new xi(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),yi(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),o=i,a=[],l=[];for(let c=0,h=o.length;c<h;c++){let u=o[c];if(u){a.push(u);let d=new et;r!==null&&d.fromArray(r.array,c*16),l.push(d)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new oo(a,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,o=[],a=[],l=[],c=[],h=[];for(let u=0,d=i.channels.length;u<d;u++){let f=i.channels[u],g=i.samplers[f.sampler],M=f.target,m=M.node,p=i.parameters!==void 0?i.parameters[g.input]:g.input,v=i.parameters!==void 0?i.parameters[g.output]:g.output;M.node!==void 0&&(o.push(this.getDependency("node",m)),a.push(this.getDependency("accessor",p)),l.push(this.getDependency("accessor",v)),c.push(g),h.push(M))}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let d=u[0],f=u[1],g=u[2],M=u[3],m=u[4],p=[];for(let S=0,_=d.length;S<_;S++){let w=d[S],T=f[S],L=g[S],y=M[S],I=m[S];if(w===void 0)continue;w.updateMatrix&&w.updateMatrix();let E=n._createAnimationTracks(w,T,L,y,I);if(E)for(let R=0;R<E.length;R++)p.push(E[R])}let v=new Ls(r,void 0,p);return yi(v,i),v})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let o=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&o.traverse(function(a){if(a.isMesh)for(let l=0,c=i.weights.length;l<c;l++)a.morphTargetInfluences[l]=i.weights[l]}),o})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),o=[],a=i.children||[];for(let c=0,h=a.length;c<h;c++)o.push(n.getDependency("node",a[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(o),l]).then(function(c){let h=c[0],u=c[1],d=c[2];d!==null&&h.traverse(function(f){f.isSkinnedMesh&&f.bind(d,Bv)});for(let f=0,g=u.length;f<g;f++)h.add(u[f]);if(h.userData.pivot!==void 0&&u.length>0){let f=h.userData.pivot,g=u[0];h.pivot=new z().fromArray(f),h.position.x-=f[0],h.position.y-=f[1],h.position.z-=f[2],g.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],o=r.name?i.createUniqueName(r.name):"",a=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&a.push(l),r.camera!==void 0&&a.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){a.push(c)}),this.nodeCache[e]=Promise.all(a).then(function(c){let h;if(r.isBone===!0?h=new pr:c.length>1?h=new lt:c.length===1?h=c[0]:h=new St,h!==c[0])for(let u=0,d=c.length;u<d;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=o),yi(h,r),r.extensions&&zs(n,h,r),r.matrix!==void 0){let u=new et;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new lt;n.name&&(r.name=i.createUniqueName(n.name)),yi(r,n),n.extensions&&zs(t,r,n);let o=n.nodes||[],a=[];for(let l=0,c=o.length;l<c;l++)a.push(i.getDependency("node",o[l]));return Promise.all(a).then(function(l){for(let h=0,u=l.length;h<u;h++){let d=l[h];d.parent!==null?r.add(Of(d)):r.add(d)}let c=h=>{let u=new Map;for(let[d,f]of i.associations)(d instanceof gn||d instanceof Qt)&&u.set(d,f);return h.traverse(d=>{let f=i.associations.get(d);f!=null&&u.set(d,f)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let o=[],a=e.name?e.name:e.uuid,l=[];function c(f){f.morphTargetInfluences&&l.push(f.name?f.name:f.uuid)}cs[r.path]===cs.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(a);let h;switch(cs[r.path]){case cs.weights:h=Ni;break;case cs.rotation:h=Ui;break;case cs.translation:case cs.scale:h=ts;break;default:switch(n.itemSize){case 1:h=Ni;break;case 2:case 3:default:h=ts;break}break}let u=i.interpolation!==void 0?Lv[i.interpolation]:Es,d=this._getArrayFromAccessor(n);for(let f=0,g=l.length;f<g;f++){let M=new h(l[f]+"."+cs[r.path],t.array,d,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(M),o.push(M)}return o}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=mu(t.constructor),i=new Float32Array(t.length);for(let r=0,o=t.length;r<o;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof Ui?fu:oc;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function zv(s,e,t){let n=e.attributes,i=new en;if(n.POSITION!==void 0){let a=t.json.accessors[n.POSITION],l=a.min,c=a.max;if(l!==void 0&&c!==void 0){if(i.set(new z(l[0],l[1],l[2]),new z(c[0],c[1],c[2])),a.normalized){let h=mu(Nr[a.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let a=new z,l=new z;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let d=t.json.accessors[u.POSITION],f=d.min,g=d.max;if(f!==void 0&&g!==void 0){if(l.setX(Math.max(Math.abs(f[0]),Math.abs(g[0]))),l.setY(Math.max(Math.abs(f[1]),Math.abs(g[1]))),l.setZ(Math.max(Math.abs(f[2]),Math.abs(g[2]))),d.normalized){let M=mu(Nr[d.componentType]);l.multiplyScalar(M)}a.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(a)}s.boundingBox=i;let o=new dn;i.getCenter(o.center),o.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=o}function Vf(s,e,t){let n=e.attributes,i=[];function r(o,a){return t.getDependency("accessor",o).then(function(l){s.setAttribute(a,l)})}for(let o in n){let a=pu[o]||o.toLowerCase();a in s.attributes||i.push(r(n[o],a))}if(e.indices!==void 0&&!s.index){let o=t.getDependency("accessor",e.indices).then(function(a){s.setIndex(a)});i.push(o)}return rt.workingColorSpace!==mn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${rt.workingColorSpace}" not supported.`),yi(s,e),zv(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?Nv(s,e.targets,t):s})}var Wf={type:"change"},_u={type:"start"},qf={type:"end"},ac=new ui,Xf=new kn,kv=Math.cos(70*Ct.DEG2RAD),on=new z,Tn=2*Math.PI,Pt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},xu=1e-6,lc=class extends Ao{constructor(e,t=null){super(e,t),this.state=Pt.NONE,this.target=new z,this.cursor=new z,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:is.ROTATE,MIDDLE:is.DOLLY,RIGHT:is.PAN},this.touches={ONE:ss.ROTATE,TWO:ss.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new z,this._lastQuaternion=new Zt,this._lastTargetPosition=new z,this._quat=new Zt().setFromUnitVectors(e.up,new z(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new Tr,this._sphericalDelta=new Tr,this._scale=1,this._panOffset=new z,this._rotateStart=new ve,this._rotateEnd=new ve,this._rotateDelta=new ve,this._panStart=new ve,this._panEnd=new ve,this._panDelta=new ve,this._dollyStart=new ve,this._dollyEnd=new ve,this._dollyDelta=new ve,this._dollyDirection=new z,this._mouse=new ve,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=Vv.bind(this),this._onPointerDown=Hv.bind(this),this._onPointerUp=Gv.bind(this),this._onContextMenu=jv.bind(this),this._onMouseWheel=qv.bind(this),this._onKeyDown=Yv.bind(this),this._onTouchStart=Zv.bind(this),this._onTouchMove=Kv.bind(this),this._onMouseDown=Wv.bind(this),this._onMouseMove=Xv.bind(this),this._interceptControlDown=Jv.bind(this),this._interceptControlUp=$v.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents(),this.domElement.getRootNode().removeEventListener("keydown",this._interceptControlDown,{capture:!0}),this.domElement.style.touchAction=""}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(Wf),this.update(),this.state=Pt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;on.copy(t).sub(this.target),on.applyQuaternion(this._quat),this._spherical.setFromVector3(on),this.autoRotate&&this.state===Pt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=Tn:n>Math.PI&&(n-=Tn),i<-Math.PI?i+=Tn:i>Math.PI&&(i-=Tn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let o=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=o!=this._spherical.radius}if(on.setFromSpherical(this._spherical),on.applyQuaternion(this._quatInverse),t.copy(this.target).add(on),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let o=null;if(this.object.isPerspectiveCamera){let a=on.length();o=this._clampDistance(a*this._scale);let l=a-o;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let a=new z(this._mouse.x,this._mouse.y,0);a.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new z(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(a),this.object.updateMatrixWorld(),o=on.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;o!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(o).add(this.object.position):(ac.origin.copy(this.object.position),ac.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(ac.direction))<kv?this.object.lookAt(this.target):(Xf.setFromNormalAndCoplanarPoint(this.object.up,this.target),ac.intersectPlane(Xf,this.target))))}else if(this.object.isOrthographicCamera){let o=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),o!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>xu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>xu||this._lastTargetPosition.distanceToSquared(this.target)>xu?(this.dispatchEvent(Wf),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?Tn/60*this.autoRotateSpeed*e:Tn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){on.setFromMatrixColumn(t,0),on.multiplyScalar(-e),this._panOffset.add(on)}_panUp(e,t){this.screenSpacePanning===!0?on.setFromMatrixColumn(t,1):(on.setFromMatrixColumn(t,0),on.crossVectors(this.object.up,on)),on.multiplyScalar(e),this._panOffset.add(on)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;on.copy(i).sub(this.target);let r=on.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,o=n.width,a=n.height;this._mouse.x=i/o*2-1,this._mouse.y=-(r/a)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Tn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Tn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(Tn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-Tn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(Tn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-Tn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(Tn*this._rotateDelta.x/t.clientHeight),this._rotateUp(Tn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let o=(e.pageX+t.x)*.5,a=(e.pageY+t.y)*.5;this._updateZoomParameters(o,a)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new ve,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function Hv(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function Vv(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function Gv(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(qf),this.state=Pt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function Wv(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case is.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Pt.DOLLY;break;case is.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Pt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Pt.ROTATE}break;case is.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Pt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Pt.PAN}break;default:this.state=Pt.NONE}this.state!==Pt.NONE&&this.dispatchEvent(_u)}function Xv(s){switch(this.state){case Pt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Pt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Pt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function qv(s){this.enabled===!1||this.enableZoom===!1||this.state!==Pt.NONE||(s.preventDefault(),this.dispatchEvent(_u),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(qf))}function Yv(s){this.enabled!==!1&&this._handleKeyDown(s)}function Zv(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case ss.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Pt.TOUCH_ROTATE;break;case ss.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Pt.TOUCH_PAN;break;default:this.state=Pt.NONE}break;case 2:switch(this.touches.TWO){case ss.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Pt.TOUCH_DOLLY_PAN;break;case ss.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Pt.TOUCH_DOLLY_ROTATE;break;default:this.state=Pt.NONE}break;default:this.state=Pt.NONE}this.state!==Pt.NONE&&this.dispatchEvent(_u)}function Kv(s){switch(this._trackPointer(s),this.state){case Pt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Pt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Pt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Pt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Pt.NONE}}function jv(s){this.enabled!==!1&&s.preventDefault()}function Jv(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function $v(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var cc=class extends As{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new fi;e.deleteAttribute("uv");let t=new yn({side:tn}),n=new yn,i=new ei(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new je(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let o=new vn(e,n,6),a=new St;a.position.set(-10.906,2.009,1.846),a.rotation.set(0,-.195,0),a.scale.set(2.328,7.905,4.651),a.updateMatrix(),o.setMatrixAt(0,a.matrix),a.position.set(-5.607,-.754,-.758),a.rotation.set(0,.994,0),a.scale.set(1.97,1.534,3.955),a.updateMatrix(),o.setMatrixAt(1,a.matrix),a.position.set(6.167,.857,7.803),a.rotation.set(0,.561,0),a.scale.set(3.927,6.285,3.687),a.updateMatrix(),o.setMatrixAt(2,a.matrix),a.position.set(-2.017,.018,6.124),a.rotation.set(0,.333,0),a.scale.set(2.002,4.566,2.064),a.updateMatrix(),o.setMatrixAt(3,a.matrix),a.position.set(2.291,-.756,-2.621),a.rotation.set(0,-.286,0),a.scale.set(1.546,1.552,1.496),a.updateMatrix(),o.setMatrixAt(4,a.matrix),a.position.set(-2.193,-.369,-5.547),a.rotation.set(0,.516,0),a.scale.set(3.875,3.487,2.986),a.updateMatrix(),o.setMatrixAt(5,a.matrix),this.add(o);let l=new je(e,Ur(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new je(e,Ur(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new je(e,Ur(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new je(e,Ur(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let d=new je(e,Ur(20));d.position.set(3.235,11.486,-12.541),d.scale.set(2.5,2,.1),this.add(d);let f=new je(e,Ur(100));f.position.set(0,20,0),f.scale.set(1,.1,1),this.add(f)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Ur(s){return new mo({color:0,emissive:16777215,emissiveIntensity:s})}var zi={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var Fn=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},Qv=new xi(-1,1,1,-1,0,1),vu=class extends pt{constructor(){super(),this.setAttribute("position",new nt([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new nt([0,2,0,0,2,0],2))}},ey=new vu,Mi=class{constructor(e){this._mesh=new je(ey,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,Qv)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Fr=class extends Fn{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof mt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=si.clone(e.uniforms),this.material=new mt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new Mi(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Zo=class extends Fn{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},hc=class extends Fn{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var uc=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new ve);this._width=n.width,this._height=n.height,t=new Wt(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:nn}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Fr(zi),this.copyPass.material.blending=Wn,this.timer=new To}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let o=this.passes[i];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),o.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),o.needsSwap){if(n){let a=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}Zo!==void 0&&(o instanceof Zo?n=!0:o instanceof hc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new ve);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var Yf={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new xe(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Or=class s extends Fn{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new ve(e.x,e.y):new ve(256,256),this.clearColor=new xe(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new Wt(r,o,{type:nn}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new Wt(r,o,{type:nn});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let d=new Wt(r,o,{type:nn});d.texture.name="UnrealBloomPass.v"+h,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),r=Math.round(r/2),o=Math.round(o/2)}let a=Yf;this.highPassUniforms=si.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new mt({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new ve(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new z(1,1,1),new z(1,1,1),new z(1,1,1),new z(1,1,1),new z(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=si.clone(zi.uniforms),this.blendMaterial=new mt({uniforms:this.copyUniforms,vertexShader:zi.vertexShader,fragmentShader:zi.fragmentShader,premultipliedAlpha:!0,blending:Xt,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new xe,this._oldClearAlpha=1,this._basic=new bt,this._fsQuad=new Mi(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new ve(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let o=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let a=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),a=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=o}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let i=0;i<e;i++)t.push(.39894*Math.exp(-.5*i*i/(n*n))/n);return new mt({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new ve(.5,.5)},direction:{value:new ve(.5,.5)},gaussianCoefficients:{value:t}},vertexShader:`

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

				}`})}};Or.BlurDirectionX=new ve(1,0);Or.BlurDirectionY=new ve(0,1);var Ko={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var dc=class extends Fn{constructor(){super(),this.isOutputPass=!0,this.uniforms=si.clone(Ko.uniforms),this.material=new br({name:Ko.name,uniforms:this.uniforms,vertexShader:Ko.vertexShader,fragmentShader:Ko.fragmentShader}),this._fsQuad=new Mi(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},rt.getTransfer(this._outputColorSpace)===yt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Co?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Po?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Io?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Ns?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Do?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===No?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Lo&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var oi=512,ty=7.5;function yu(s,e=ty){let t=new uo,n=s.map(([a,l])=>new z(a,0,l)),i=[],r=[],o=n.length;for(let a=0;a<o;a++){let l=n[a],c=n[(a+o-1)%o],h=n[(a+1)%o],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let a=0;a<o;a++)t.add(new Qn(i[a],n[a],r[a])),t.add(new yr(r[a],i[(a+1)%o]));return t}function ny(s){let e=yu(s),t=e.getSpacedPoints(oi),n=[];for(let i=0;i<oi;i++){let r=t[(i+oi-1)%oi],o=t[(i+1)%oi];n.push(new z(o.x-r.x,0,o.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function Zf(s){let e=[],t=(n,i,r,o)=>{let a=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*a+r*l,z:n.z-i*l+r*a,r:o,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function Kf({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:o=()=>!1}){let a=s.map(ny),l=s.map((E,R)=>({route:R,s:a[R].length*(R*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[R]??5.1+R*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new z,h=new z,u=new z;function d(E,R,F,X,P){let N=(R/E.length%1+1)%1*oi,O=Math.floor(N)%oi,A=N-Math.floor(N);X.lerpVectors(E.points[O],E.points[(O+1)%oi],A),P.lerpVectors(E.tangents[O],E.tangents[(O+1)%oi],A).normalize().multiplyScalar(F)}let f=E=>Math.atan2(Math.sin(E),Math.cos(E));function g(E){d(a[E.route],E.s,E.dir,c,h),E.fx=h.x,E.fz=h.z,E.rx=-h.z,E.rz=h.x,E.x=c.x+E.rx*E.lane,E.z=c.z+E.rz*E.lane}function M(E,R){for(let F of e){let X=F.r+t;if((E-F.x)**2+(R-F.z)**2<X*X)return!0}return!1}function m(E,R){let F=a[E.route];for(let X of[-2.5,0,1,3,6,9,12,15,18])if(d(F,E.s+E.dir*X,E.dir,c,h),u.set(c.x-h.z*R,0,c.z+h.x*R),M(u.x,u.z)||X>=0&&o(E,u.x,u.z))return!1;return!0}function p(E){E.dir=-E.dir,E.lane=-E.lane,E.laneTarget=i,E.laneVel=0,E.state="turn",E.cooldown=4,E.wait=0,E.target=0,E.turns++}function v(E,R){if(E.held){E.target=0,E.speed=0,E.laneVel=0;return}if(E.cooldown>0&&(E.cooldown-=R),E.state==="turn"){if(E.target=0,E.speed<.05){let N=Math.atan2(E.fx,E.fz),O=f(N-E.heading);E.heading+=O*Math.min(1,R*3.2),Math.abs(O)<.1&&(E.heading=N,E.state="cruise")}return}let F=null,X=[...n].sort((N,O)=>Math.abs(N-i)-Math.abs(O-i)||O-N);for(let N of X)if(m(E,N)){F=N;break}if(F===null){E.cooldown<=0?p(E):E.target=0;return}E.laneTarget=F,E.target=E.cruise,Math.abs(E.laneTarget-E.lane)>.5&&!m(E,E.lane)&&(E.target=Math.min(E.target,2.4));let P=!1;for(let N of l){if(N===E)continue;let O=L(E,E.laneTarget,N);if(!O)continue;P=!0;let A=N.fx*E.fx+N.fz*E.fz,V=N.speed<.3;if(V||A<-.2){let Y=_.get(N)[S.length-1],G=V?0:Math.sign((Y.x-E.x)*E.rx+(Y.z-E.z)*E.rz)||-1,ne=null;for(let le of X)if(!(G&&Math.sign(le-E.lane)===G)&&m(E,le)&&!L(E,le,N)){ne=le;break}ne!==null?E.laneTarget=ne:E.target=0}else A>.5?O.da>=O.db&&(E.target=Math.min(E.target,O.da<3.8?0:N.speed*.9)):(O.da>O.db||O.da===O.db&&E.route>N.route)&&(E.target=Math.min(E.target,O.da<=4?0:2))}P&&E.target===0?E.wait+=R:E.wait=0,E.wait>2.5&&E.cooldown<=0&&p(E)}let S=[0,2,4,6,8,10,12],_=new Map,w=new z;function T(E,R,F,X){d(a[E.route],E.s+E.dir*R,E.dir,c,h),X.set(c.x-h.z*F,0,c.z+h.x*F)}function L(E,R,F){let X=_.get(F),P=(t*2+.6)**2,N=F.speed<.3;for(let O=0;O<S.length;O++){T(E,S[O],R,w);let A=S[O]/Math.max(E.speed,1.5);for(let V=0;V<S.length&&!(N&&V>0);V++){let Y=S[V]/Math.max(F.speed,1.5);if(!(S[O]>4&&Math.abs(A-Y)>1.6)&&w.distanceToSquared(X[V])<P)return{da:S[O],db:S[V]}}}return null}function y(E){if(E>0){for(let R of l){g(R),_.has(R)||_.set(R,S.map(()=>new z));let F=_.get(R);S.forEach((X,P)=>T(R,R.state==="cruise"?X:0,R.lane,F[P]))}for(let R of l)v(R,E);for(let R=0;R<l.length;R++)for(let F=R+1;F<l.length;F++){let X=l[R],P=l[F],N=P.x-X.x,O=P.z-X.z,A=Math.hypot(N,O),V=t*2+.2;if(A>=V)continue;let Y=X.state==="turn"||X.speed<.3,G=P.state==="turn"||P.speed<.3,ne=Y===G?.5:Y?0:1,le=Math.min(V-Math.max(A,.01),2.5*E),Ae=N*X.rx+O*X.rz,Pe=-(N*P.rx+O*P.rz);X.lane=Ct.clamp(X.lane-Math.sign(Ae||1)*le*ne,-4.2,4.2),P.lane=Ct.clamp(P.lane-Math.sign(Pe||1)*le*(1-ne),-4.2,4.2),X.target=Math.min(X.target,.5),P.target=Math.min(P.target,.5)}for(let R of l){let F=R.x,X=R.z,P=R.target>R.speed?6:9;if(R.held){R.vx=R.vz=0;continue}R.speed+=Ct.clamp(R.target-R.speed,-P*E,P*E),R.speed<0&&(R.speed=0),R.s+=R.dir*R.speed*E;let N=R.laneTarget-R.lane,O=R.state==="turn"?0:Math.min(3,R.speed*.6),A=Math.sign(N)*Math.min(O,Math.sqrt(8*Math.abs(N)),Math.abs(N)/E);if(R.laneVel+=Ct.clamp(A-R.laneVel,-4*E,4*E),R.lane+=R.laneVel*E,g(R),R.vx=(R.x-F)/E,R.vz=(R.z-X)/E,R.state==="cruise"){let V=R.speed>.25?Math.atan2(R.vx,R.vz):Math.atan2(R.fx,R.fz);R.heading+=f(V-R.heading)*Math.min(1,E*(R.speed>.25?8:3.2))}}}}let I=[];for(let E of l){for(let R=0;R<600&&(g(E),!(I.every(X=>Math.hypot(E.x-X.x,E.z-X.z)>t*2+6)&&m(E,E.lane)));R++)E.s+=1;E.heading=Math.atan2(E.fx,E.fz),I.push(E)}return{agents:l,step:y,hitsObstacle:M,seek(E,R){E.s=R,g(E),E.heading=Math.atan2(E.fx,E.fz)},adopt(E,R){let F=a[E.route],X=0,P=1/0;for(let N=0;N<oi;N++){let O=F.points[N],A=Math.hypot(O.x-R.x,O.z-R.z);A<P&&(P=A,X=N)}E.s=X/oi*F.length,d(F,E.s,E.dir,c,h),E.lane=(R.x-c.x)*-h.z+(R.z-c.z)*h.x,E.laneTarget=i,E.speed=0,E.laneVel=0,E.state="cruise",E.heading=R.heading,g(E)},stats:()=>({states:l.map(E=>E.state),turns:l.reduce((E,R)=>E+R.turns,0),lanes:l.map(E=>Math.round(E.lane*10)/10)})}}var iy=(s,e,t)=>Math.max(e,Math.min(t,s)),mc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),jo=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function ki(s,e=1){let t=s.min.map(d=>d*e),n=s.max.map(d=>d*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,o=i>r,a=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(a/Math.max(.3,l)))),h=a/c,u=[];for(let d=0;d<c;d++){let f=-a+h+d*h*2;u.push({x:(t[0]+n[0])/2+(o?f:0),z:(t[2]+n[2])/2+(o?0:f),r:Math.hypot(l,h)})}return{circles:u,minY:t[1],maxY:n[1],reach:Math.max(...u.map(d=>Math.hypot(d.x,d.z)+d.r))}}function Br(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function pc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let o=(t-s)/r,a=(n-s)/r;return i[0]=Math.max(i[0],Math.min(o,a)),i[1]=Math.min(i[1],Math.max(o,a)),i[0]<=i[1]}function sy(s,e,t,n,i,r){let o=[0,1];return pc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,o)?o:null}function fc(s,e,t,n,i,r){let o=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>o||Math.min(i.x,r.x)-Math.max(e.x,t.x)>o||Math.min(e.z,t.z)-Math.max(i.z,r.z)>o||Math.min(i.z,r.z)-Math.max(e.z,t.z)>o)return!1;let a=sy(s,e,t,n,i,r);if(!a)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(mc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(mc(i.heading,r.heading))*Math.hypot(c.x,c.z),u=Br(l,e),d=Br(l,t),f=Br(c,i),g=Br(c,r),M=u.x-f.x,m=u.z-f.z,p=d.x-g.x-M,v=d.z-g.z-m,S=iy(-(M*p+m*v)/Math.max(1e-12,p*p+v*v),...a),_=l.r+c.r+h;if((M+p*S)**2+(m+v*S)**2<_*_)return!0}return!1}function Mu(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),o=Math.sin(n.heading||0),a=l=>({x:(l.x-n.x)*r-(l.z-n.z)*o,z:(l.x-n.x)*o+(l.z-n.z)*r});for(let l of s.circles){let c=a(Br(l,e)),h=a(Br(l,t)),u=l.r+.06+Math.abs(mc(e.heading,t.heading))*Math.hypot(l.x,l.z),d=[0,1];if(pc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,d)&&pc(c.x,h.x,n.min[0]-u,n.max[0]+u,d)&&pc(c.z,h.z,n.min[2]-u,n.max[2]+u,d))return!0}return!1}function gc(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,o=0,a=(g,M)=>g.id===M.id||g.ignore===M.id||M.ignore===g.id||g.enabled===!1||M.enabled===!1;function l(g,M,m,p=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let v of e.values())if(v.enabled!==!1&&!(v.owner&&(v.owner===g.ignore||v.owner===g.id))&&Mu(g,M,m,v))return v.id;if(p){for(let v of s.values())if(!a(g,v)&&fc(g,M,m,v,v,v))return v.id}return null}let c=(g,M,m,p=!0)=>!l(g,M,m,p);function h(g,M,m,p=0){if(s.has(g))return s.get(g);let v={id:g,...M,...jo(m),priority:p,enabled:!0,blocked:0};return c(v,v,v)?(s.set(g,v),v):null}function u(g,M,m=12){let p={...g,enabled:!0};for(let v=0;v<=m;v+=.5)for(let S=0;S<(v?24:1);S++){let _={...jo(M),x:M.x+Math.cos(S*Math.PI/12)*v,z:M.z+Math.sin(S*Math.PI/12)*v};if(c(p,_,_))return _}return null}function d(g,M,m=12){let p=u(g,M,m);return p?(Object.assign(g,p),!0):!1}function f(g){let M=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let p=t.get(m.id),v=p?.next||jo(m);return{body:m,from:jo(m),next:v,request:p,accepted:c(m,m,v,!1)}});for(let m=0;m<=M.length;m++){let p=!1;for(let v of M)v.accepted&&v.body.follow&&M.some(S=>S.body.id===v.body.follow&&!S.accepted)&&(v.accepted=!1,p=!0);for(let v=0;v<M.length;v++)for(let S=v+1;S<M.length;S++){let _=M[v],w=M[S];if(a(_.body,w.body)||!_.accepted&&!w.accepted)continue;let T=_.accepted?_.next:_.from,L=w.accepted?w.next:w.from;if(!fc(_.body,_.from,T,w.body,w.from,L))continue;let y=Y=>Y.request&&Y.accepted&&(Math.hypot(Y.next.x-Y.from.x,Y.next.y-Y.from.y,Y.next.z-Y.from.z)>1e-8||Math.abs(mc(Y.from.heading,Y.next.heading))>1e-8),I=y(_),E=y(w);if(!I&&!E)continue;let R=_.next.x-_.from.x,F=_.next.z-_.from.z,X=w.next.x-w.from.x,P=w.next.z-w.from.z,N=R*X+F*P>.8*Math.hypot(R,F)*Math.hypot(X,P),O=I&&!fc(_.body,_.from,_.next,w.body,w.from,w.from),A=E&&!fc(_.body,_.from,_.from,w.body,w.from,w.next),V=I?E?O&&!A?w:A&&!O?_:_.body.priority!==w.body.priority?_.body.priority<w.body.priority?_:w:N?(w.from.x-_.from.x)*R+(w.from.z-_.from.z)*F>0?_:w:_.body.id>w.body.id?_:w:_:w;V.accepted=!1,p=!0}if(!p)break}for(let m of M)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=g,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of M)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=g}return{register:h,relocate:d,findFree:u,clear:c,obstruction:l,solve:f,begin(){t.clear()},propose(g,M,m){g?.enabled!==!1&&g&&t.set(g.id,{next:jo(M),commit:m})},remove(g){s.delete(g),t.delete(g)},solid(g,M){let m=e.get(g),p={id:g,x:0,z:0,heading:0,...M};if(m&&m.x===p.x&&m.z===p.z&&m.heading===p.heading&&m.enabled===p.enabled&&m.min.every((T,L)=>T===p.min[L])&&m.max.every((T,L)=>T===p.max[L]))return;let v=Math.cos(p.heading),S=Math.sin(p.heading),_=[],w=[];for(let T of[p.min[0],p.max[0]])for(let L of[p.min[2],p.max[2]])_.push(p.x+T*v+L*S),w.push(p.z-T*S+L*v);p.world=[Math.min(..._),Math.min(...w),Math.max(..._),Math.max(...w)],e.set(g,p),o++},removeSolid(g){e.delete(g)&&o++},removeOwner(g){for(let[M,m]of e)m.owner===g&&(e.delete(M),o++)},revision:()=>o,solidClear(g){return[...s.values()].every(M=>M.enabled===!1||g.owner&&M.ignore===g.owner||!Mu(M,M,M,{x:0,z:0,heading:0,...g}))},ceiling(g,M,m){let p=0;for(let v of e.values())v.enabled!==!1&&Mu(g,{x:M,y:v.min[1],z:m,heading:0},{x:M,y:v.min[1],z:m,heading:0},v)&&(p=Math.max(p,v.max[1]-g.minY+1));return p},body:g=>s.get(g),neighbors(g,M=12){return[...s.values()].filter(m=>!a(g,m)&&Math.abs(m.y-g.y)<4&&Math.hypot(m.x-g.x,m.z-g.z)<M)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(g=>g.enabled!==!1).map(g=>({id:g.id,x:g.x,y:g.y,z:g.z,heading:g.heading,blocked:g.blocked,obstruction:g.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function jf(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var Jf=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function $f(s,e,t){let n=!1,i=0,r=!1,o=!1,a=0,l=!1,c=new lt;c.name="city-life",s.add(c);let h=t.traffic,u=[],d=Kf({routes:Jf,obstacles:t.obstacles||[],blocked:(ue,he,H)=>{let Z=u[ue.route],W=Z?{...Z,x:he,z:H}:null;return Z?!h.clear(Z,W,W):!1}}),f=new Set,g=new Set,M=new Set,m=[],p=new AbortController,v=[],S=[],_=ue=>(f.add(ue),ue),w=ue=>(g.add(ue),ue),T=_(new Dn(1,.013,5,64));for(let ue of e){let he=w(new bt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),H=new je(T,he);H.rotation.x=Math.PI/2,H.position.set(ue.x,.6,ue.z),H.scale.setScalar(ue.radius*1.12),c.add(H),S.push({id:ue.id,ring:H,material:he,state:"unknown",eventUntil:0,color:new xe(7438733)})}let L=new Map,y=[],I=new Map,E=Date.now(),R=3e3,F=e.find(ue=>ue.id==="agent"),X=new z(0,0,1),P=new z,N={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(F){for(let ue of e)if(ue!==F){let he=new z(F.x,F.height+3,F.z),H=new z(ue.x,ue.height+4,ue.z),Z=he.clone().lerp(H,.5);Z.y=Math.max(he.y,H.y)+12;let W=new Qn(he,Z,H),J=w(new di({color:N[ue.id],transparent:!0,opacity:0,depthWrite:!1,blending:Xt,toneMapped:!1})),te=new Ii(_(new pt().setFromPoints(W.getPoints(48))),J);te.visible=!1,c.add(te),L.set(ue.id,{curve:W,line:te})}}let O=_(new Dn(1,.06,5,40,Math.PI*.95));for(let ue=0;ue<12;ue++){let he=[];for(let H=0;H<4;H++){let Z=w(new bt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:Xt,toneMapped:!1})),W=new je(H===3?T:O,Z);W.visible=!1,c.add(W),he.push(W)}y.push({at:-1/0,waves:he,link:null})}function A(ue,he){if(!l||t.active?.()===!1||ue.at<E||he-ue.at>R||ue.at>he||!["started","succeeded","failed","sanitized","progress"].includes(ue.state))return;let H=ue.to==="agent",Z=H?ue.from:ue.to;if(!H&&ue.from!=="agent"||!L.has(Z))return;let W=Z+":"+H;if(ue.at-(I.get(W)??-1/0)<450)return;I.set(W,ue.at);let J=y.reduce((te,fe)=>te.at<fe.at?te:fe);Object.assign(J,{at:ue.at,link:L.get(Z),incoming:H,from:ue.from,to:ue.to,state:ue.state}),J.waves.forEach(te=>te.material.color.setHex(ue.state==="failed"?16737881:N[Z]))}function V(ue){let he=Date.now();L.forEach(H=>{H.line.visible=!1});for(let H of y){(!ue||t.active?.()===!1)&&(H.at=-1/0);let Z=(he-H.at)/R,W=Z>=0&&Z<1;if(H.waves.forEach(te=>{te.visible=!1}),!W||!H.link)continue;H.link.line.visible=!0,H.link.line.material.opacity=.12*Math.sin(Z*Math.PI);for(let te=0;te<3;te++){let fe=(Z-te*.075)/.75;if(fe<0||fe>1)continue;let Me=H.incoming?1-fe:fe,Oe=H.waves[te];H.link.curve.getPoint(Me,Oe.position),H.link.curve.getTangent(Me,P),H.incoming&&P.negate(),Oe.quaternion.setFromUnitVectors(X,P),Oe.rotateZ(Math.PI*.525),Oe.scale.setScalar(1.2+Math.sin(fe*Math.PI)*3.5),Oe.material.opacity=Math.sin(fe*Math.PI)*.8,Oe.visible=!0}let J=(Z-.74)/.26;if(J>0){let te=H.waves[3];H.link.curve.getPoint(H.incoming?0:1,te.position),te.rotation.set(-Math.PI/2,0,0),te.scale.setScalar(1+J*8),te.material.opacity=(1-J)*.6,te.visible=!0}}}let Y=_(new ln(7,7)),G=w(new mt({transparent:!0,depthWrite:!1,blending:Xt,uniforms:{color:{value:new xe(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),ne=_(new Ps(.42,1.3,12,1,!0)),le=w(new bt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:Xt}));Jf.forEach((ue,he)=>{let H=new lt;H.name="city-white-robot-"+(he+1);let Z=new lt;H.add(Z),c.add(H);let W=new je(Y,G);W.rotation.x=-Math.PI/2,W.position.y=.58,c.add(W);let J=new je(ne,le);J.rotation.z=Math.PI,J.position.y=-.45,H.add(J),H.visible=!1,W.visible=!1,v.push({root:H,body:Z,glow:W,jet:J,agent:d.agents[he]})});function Ae(ue){ue.traverse(he=>{if(he.isMesh){f.add(he.geometry);for(let H of Array.isArray(he.material)?he.material:[he.material]){g.add(H);for(let Z of Object.values(H))Z?.isTexture&&M.add(Z)}}})}function Pe(){f.forEach(he=>he.dispose()),g.forEach(he=>he.dispose());let ue=new Set;M.forEach(he=>{he.image&&ue.add(he.image),he.dispose()}),ue.forEach(he=>he.close?.()),f.clear(),g.clear(),M.clear()}async function Re(){let ue=setTimeout(()=>p.abort(),15e3);try{let he=await fetch(t.robotURL,{signal:p.signal});if(!he.ok)throw Error("Robot unavailable");let H=await he.arrayBuffer();if(n)return;let{scene:Z}=await new hs().parseAsync(H,"");if(Ae(Z),n){Pe();return}let W=new en().setFromObject(Z),J=W.getSize(new z),te=Math.max(J.x,J.y,J.z);if(!Number.isFinite(te)||te<=0)throw Error("Invalid robot bounds");Z.scale.setScalar(6/te),Z.updateMatrixWorld(!0),W.setFromObject(Z);let fe=W.getCenter(new z);Z.position.set(-fe.x,-W.min.y,-fe.z),Z.rotation.y=-Math.PI/2,Z.updateMatrixWorld(!0),W.setFromObject(Z),W.expandByScalar(.4);let Me=ki({min:W.min.toArray(),max:W.max.toArray()});Z.traverse(Oe=>{Oe.isMesh&&(Oe.castShadow=!1,Oe.receiveShadow=!0)}),v.forEach((Oe,We)=>{if(Oe.body.add(Z.clone(!0)),Oe.root.visible=!0,Oe.glow.visible=!0,h){let Ye=Oe.agent,Je=null;for(let q=0;q<600&&!Je;q++)Je=h.register("patrol-"+We,Me,{x:Ye.x,y:1.6,z:Ye.z,heading:Ye.heading},1),Je||d.seek(Ye,Ye.s+1);Je?(u[We]=Je,t.society?.patrol(Je.id,Je,Oe.root,(q,Be)=>{Ye.held=q,Be&&d.adopt(Ye,Be)})):(Oe.root.visible=!1,Oe.glow.visible=!1,Ye.held=!0)}}),r=!0,ce(0,!1)}catch(he){n||(o=!0,t.onError?.(he))}finally{clearTimeout(ue)}}function oe(ue){m.forEach(H=>{H.dispose(),g.delete(H)}),m.length=0,ue.children.find(H=>H.userData.district==="operations")?.getObjectByName("signal")?.traverse(H=>{if(!H.isMesh)return;let Z=W=>{let J=w(W.clone());return m.push(J),J};H.material=Array.isArray(H.material)?H.material.map(Z):Z(H.material)})}function pe(ue,he=[]){for(let Z of S){let W=ue.find(J=>J.id===Z.id);Z.state=!W||W.stale?"unknown":W.state,Z.color.setHex(Z.state==="error"?16730430:Z.state==="running"?7400403:Z.state==="unknown"?7438733:7452878),Z.material.color.copy(Z.color)}let H=Date.now();for(let Z of[...he].reverse())if(Z.id>a&&(A(Z,H),l&&Z.at>=E&&t.active?.()!==!1&&t.society?.event(Z),H-Z.at<6e3)){let W=S.find(J=>J.id===Z.district);W&&(W.eventUntil=Z.at+5e3)}a=Math.max(a,...he.map(Z=>Z.id))}function ce(ue,he){if(n)return;l=he,V(he);let H=he?Math.min(.1,Math.max(0,ue)):0,Z=h?d.agents.map(W=>({...W})):null;he&&H>0&&(i+=H,d.step(H));for(let[W,J]of v.entries()){let te=J.agent,fe=Math.hypot(te.vx,te.vz),Me=(!he||te.held)&&u[W]?u[W]:te;J.root.position.set(Me.x,1.6+Math.sin(i*1.6+W*1.9)*.18,Me.z),J.root.rotation.y=Me.heading;let Oe=(te.vx*te.rx+te.vz*te.rz)/Math.max(1,fe);if(J.body.rotation.z=Math.sin(i*.85+W)*.035-Oe*.08,J.body.rotation.x=-.035+Math.sin(i*1.1+W)*.018-Math.min(.09,fe*.015),J.glow.position.x=Me.x,J.glow.position.z=Me.z,J.jet.scale.y=1+Math.sin(i*4+W)*.12+fe*.04,he&&u[W]&&!te.held){let We=u[W],Ye={x:te.x,y:1.6,z:te.z,heading:te.heading};J.escape&&i<J.escape.until?(Ye.x=We.x+J.escape.x*H*1.8,Ye.z=We.z+J.escape.z*H*1.8,Ye.heading=We.heading):J.escape=null,!h.clear(We,We,Ye,!1)&&h.clear(We,We,{...Ye,heading:We.heading},!1)&&(Ye.heading=We.heading);let Je=Math.hypot(Ye.x-We.x,Ye.z-We.z);h.propose(We,Ye,(q,Be)=>{if(J.wait=q&&Je>.002?0:(J.wait||0)+H,J.escape&&q&&d.adopt(te,Be),J.wait>2.5){let $e=Math.sin(Be.heading),b=Math.cos(Be.heading);for(let[x,U]of[[b,-$e],[-b,$e],[$e,b],[-$e,-b]])if(h.clear(Be,Be,{...Be,x:Be.x+x*2.5,z:Be.z+U*2.5})){J.escape={x,z:U,until:i+2};break}J.wait=0}if(q)te.trafficWait=0,te.heading=Be.heading;else{let $e=(te.trafficWait||0)+H;Object.assign(te,Z[W]),te.speed=0,te.vx=te.vz=0,te.trafficWait=$e,$e>2.5&&(te.state!=="turn"?(te.dir=-te.dir,te.lane=-te.lane,te.laneTarget=2.2,te.state="turn",te.turns++):te.state="cruise",te.trafficWait=0)}J.root.position.x=Be.x,J.root.position.z=Be.z,J.root.rotation.y=Be.heading,J.glow.position.x=Be.x,J.glow.position.z=Be.z})}}for(let W of S){let J=he?.5+.5*Math.sin(i*(W.state==="error"?3.2:1.8)):.5,te=W.state==="error"||W.state==="running"||W.eventUntil>Date.now();if(W.material.opacity=W.state==="unknown"?.03:te?.2+J*.46:.065,W.ring.scale.setScalar(e.find(fe=>fe.id===W.id).radius*1.12*(1+(te&&he?J*.035:0))),W.id==="operations")for(let fe of m)fe.color.copy(W.color),fe.emissive?.copy(W.color),fe.emissiveIntensity=W.state==="error"?1.1+J*2.2:W.state==="unknown"?.08:.55}}function Te(){n||(n=!0,p.abort(),t.signal?.removeEventListener("abort",Te),c.removeFromParent(),Pe(),v.length=0,m.length=0,u.forEach(ue=>h.remove(ue.id)))}return t.signal?.addEventListener("abort",Te,{once:!0}),t.signal?.aborted?Te():Re(),{update:ce,setData:pe,attachLandmarks:oe,dispose:Te,stats:()=>({robots:r?v.filter(ue=>ue.root.visible).length:0,robotError:o,time:i,navigation:d.stats(),positions:v.map(ue=>ue.root.position.toArray()),transmissions:y.filter(ue=>ue.waves.some(he=>he.visible)).map(ue=>({from:ue.from,to:ue.to,state:ue.state,age:Date.now()-ue.at})),signals:S.map(ue=>({id:ue.id,state:ue.state,intensity:ue.material.opacity}))})}}var Jo='Geist, "Segoe UI", system-ui, sans-serif',xc=13,ry=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},bu={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},oy={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},ay={vertex:bu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},Qf={vertex:bu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},ly={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function zr(s,e,t={}){return new mt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:Xt,side:Nt,...t})}function cy(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function ep(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,o="";for(let c of e.split(" ")){if(i.length>=n)break;let h=o?o+" "+c:c;if(r(h)){o=h;continue}if(o&&(i.push(o),o="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let d=u.length-1;for(;d>1&&!r(u.slice(0,d));)d--;i.push(u.slice(0,d)),u=u.slice(d)}o=u}o&&i.length<n&&(i.push(o),o="");let a=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(a<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function tp(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new lt;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let o=[],a=[],l=[],c=(H,Z)=>(o.push(H),a.push(Z),new je(H,Z)),h=new xe(6544639),u=new xe(10980351),d=new xe(14218751),f={time:{value:0}},g=!1,M=0,m=[],p=[],v=0,S="none",_=!1,w=c(new ln(11,11),zr(ay,{time:f.time,strength:{value:.9},color:{value:h}}));w.rotation.x=-Math.PI/2,w.position.y=.18,r.add(w);let T=c(new es(7.4,2.3,xc,56,1,!0),zr(oy,{time:f.time,strength:{value:.32},base:{value:h},top:{value:u}}));T.position.y=xc/2+.3,r.add(T);let L=c(new Dn(7.5,.055,6,128),zr(Qf,{time:f.time,strength:{value:1.6},color:{value:h}}));L.rotation.x=Math.PI/2,L.position.y=xc+.3,r.add(L);let y=c(new Dn(2.6,.05,6,96),zr(Qf,{time:f.time,strength:{value:2},color:{value:d}}));y.rotation.x=Math.PI/2,y.position.y=.55,r.add(y);let I=new Mr(1.35,1),E=new po(I),R=new di({color:10217983,transparent:!0,opacity:.75,blending:Xt,depthWrite:!1,toneMapped:!1}),F=new Rs(E,R);F.position.y=4.6,r.add(F),o.push(I,E),a.push(R);let X=new bt({color:4175871,transparent:!0,opacity:.12,blending:Xt,depthWrite:!1,toneMapped:!1}),P=c(new Mr(1.1,1),X);F.add(P);let N=160,O=new Float32Array(N);for(let H=0;H<N;H++)O[H]=H*.618033988749895%1;let A=new pt;A.setAttribute("position",new xt(new Float32Array(N*3),3)),A.setAttribute("seed",new xt(O,1)),A.boundingSphere=new dn(new z(0,xc/2,0),12);let V=zr(ly,{time:f.time,pointScale:{value:800},color:{value:d},strength:{value:.9}}),Y=new xn(A,V);Y.frustumCulled=!1,r.add(Y),o.push(A),a.push(V);let G=new ei(6478079,26,46,2);G.position.y=6,r.add(G);function ne(H,Z,W,J){let te=document.createElement("canvas");te.width=W,te.height=J;let fe=te.getContext("2d"),Me=new Cs(te);Me.colorSpace=Ht,Me.generateMipmaps=!1,Me.minFilter=Vt,l.push(Me);let Oe=zr(bu,{map:{value:Me},time:f.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new xe(16777215)}},{side:In}),We=c(new ln(H,Z),Oe);return{canvas:te,context:fe,texture:Me,material:Oe,mesh:We,text:"",targetFade:1,next:0}}let le=ne(17.5,8.55,1024,500),Ae=[ne(8.6,2.1,640,156),ne(8.6,2.1,640,156)];le.mesh.position.y=10.4,r.add(le.mesh),Ae.forEach((H,Z)=>{H.mesh.position.y=Z?14.4:5.9,H.orbit=Z?-.22:.27,H.angle=Z*2.3,r.add(H.mesh)});function Pe(H,Z,W){let{context:J,canvas:te}=le,fe=te.width,Me=te.height;J.clearRect(0,0,fe,Me),J.fillStyle="rgba(48,150,214,0.14)",J.beginPath(),J.roundRect(14,14,fe-28,Me-28,22),J.fill(),J.strokeStyle="rgba(150,230,255,0.85)",J.lineWidth=3;for(let[Oe,We,Ye,Je]of[[18,18,1,1],[fe-18,18,-1,1],[18,Me-18,1,-1],[fe-18,Me-18,-1,-1]])J.beginPath(),J.moveTo(Oe,We+Je*42),J.lineTo(Oe,We),J.lineTo(Oe+Ye*42,We),J.stroke();J.fillStyle="rgba(150,230,255,0.55)",J.fillRect(48,108,fe-96,2),J.font="600 27px "+Jo,J.textBaseline="middle",J.fillStyle="rgba(160,232,255,0.92)",J.textAlign="left",J.fillText("\u258C "+i+(W?"  \xB7  "+String(Z+1).padStart(2,"0")+" / "+String(W).padStart(2,"0"):""),50,72),J.textAlign="right",J.font="500 25px "+Jo,J.fillStyle="rgba(190,150,255,0.85)",J.fillText("0x"+ry(H||i).toUpperCase(),fe-52,72),J.textAlign="left",J.shadowColor="rgba(120,225,255,0.9)",J.shadowBlur=16,H?(J.font="600 54px "+Jo,J.fillStyle="rgba(232,250,255,0.97)",ep(J,H,fe-110,4).forEach((We,Ye)=>J.fillText(We,54,172+Ye*72))):(J.font="600 40px "+Jo,J.fillStyle="rgba(180,235,255,0.7)",J.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),J.shadowBlur=0,le.texture.needsUpdate=!0}function Re(H,Z){let{context:W,canvas:J}=H,te=J.width,fe=J.height;W.clearRect(0,0,te,fe),W.fillStyle="rgba(48,150,214,0.12)",W.beginPath(),W.roundRect(6,6,te-12,fe-12,14),W.fill(),W.fillStyle="rgba(190,150,255,0.8)",W.fillRect(20,26,6,fe-52),W.font="600 42px "+Jo,W.textBaseline="middle",W.textAlign="left",W.shadowColor="rgba(160,140,255,0.9)",W.shadowBlur=12,W.fillStyle="rgba(236,240,255,0.95)",W.fillText(Z?ep(W,Z,te-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,fe/2),W.shadowBlur=0,H.texture.needsUpdate=!0}function oe(){return m.length?(v>=p.length&&(p=m.map((H,Z)=>Z).sort(()=>Math.random()-.5),v=0),m[p[v++]]):""}let pe=.8,ce=[3.5,6.5];function Te(H){let Z=oe();le.text=Z,M++,Pe(Z,m.indexOf(Z),m.length),le.material.uniforms.glitch.value=H?1:0,le.material.uniforms.fade.value=H?.35:1,pe=g?12:4.5+Math.random()*3}function ue(H,Z){let W=Ae[H],J=oe();W.text=J,Re(W,J.length>46?J.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":J),W.material.uniforms.glitch.value=Z?.7:0,W.material.uniforms.fade.value=Z?.3:1,ce[H]=g?15:6+Math.random()*4}Pe("",0,0),Ae.forEach(H=>Re(H,"")),document.fonts?.ready?.then(()=>{_||(Pe(le.text,m.indexOf(le.text),m.length),Ae.forEach(H=>Re(H,H.text)))});let he=new z;return{group:r,setTexts(H,Z="live"){let W=cy(H),J=W.length!==m.length||W.some((te,fe)=>te!==m[fe]);m=W,S=W.length?Z:"none",J&&(p=[],v=0,(!le.text||!m.includes(le.text))&&(pe=Math.min(pe,.6)))},setPointScale(H){V.uniforms.pointScale.value=H},setReducedMotion(H){g=!!H},update(H,Z,W){if(_)return;let J=W&&!g,te=J?Math.min(.1,Math.max(0,H)):0;f.time.value+=te,pe-=H,pe<=0&&Te(J),ce.forEach((fe,Me)=>{ce[Me]=fe-H,ce[Me]<=0&&ue(Me,J)});for(let fe of[le,...Ae]){let Me=fe.material.uniforms;Me.glitch.value=Math.max(0,Me.glitch.value-H*2.4),Me.fade.value=Math.min(1,Me.fade.value+H*1.8),fe.mesh.quaternion.copy(Z.quaternion)}Ae.forEach(fe=>{fe.angle+=fe.orbit*te,fe.mesh.position.x=Math.cos(fe.angle)*6.4,fe.mesh.position.z=Math.sin(fe.angle)*6.4,he.copy(fe.mesh.position).add(r.position).sub(Z.position).normalize();let Me=-(he.x*Math.cos(fe.angle)+he.z*Math.sin(fe.angle));fe.material.uniforms.gain.value=1.35*Ct.clamp(.35+Me*.9,.15,1.2)}),le.mesh.position.y=9.6+Math.sin(f.time.value*.9)*.22,F.rotation.y+=te*.7,F.rotation.x+=te*.31,P.rotation.y-=te*1.1,L.rotation.z+=te*.18,y.rotation.z-=te*.42,G.intensity=26+Math.sin(f.time.value*2.1)*6+le.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:S,switches:M,shown:le.text?m.indexOf(le.text):-1}),dispose(){_||(_=!0,r.removeFromParent(),o.forEach(H=>H.dispose()),a.forEach(H=>H.dispose()),l.forEach(H=>H.dispose()),G.dispose())}}}var hy=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function np(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new ve(.5,.5)},shaftColor:{value:new xe(12376319)},shadowTint:{value:new xe(6686)},highlightTint:{value:new xe(1444864)}},t=new Fr(new mt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:hy}));t.enabled=!1;let n=new z,i=new xe(16761992),r=new xe(12376319),o=!1;return{pass:t,setCinematic(a){o=!!a},update(a,l,c,{day:h,dusk:u,cloud:d,animated:f}){e.aspect.value=l.aspect;let g=o?1:0;e.letterbox.value=f?e.letterbox.value+(g-e.letterbox.value)*Math.min(1,a*2.2):g,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let M=(n.x+1)/2,m=(n.y+1)/2,p=n.z<1?Math.max(0,1-Math.max(0,Math.abs(M-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(M,m),e.shafts.value=p*(.06+u*.14+(1-h)*.03)*(1-d*.75),e.shaftColor.value.copy(r).lerp(i,u),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+u*.03,.01+u*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var Tu=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,Su="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",uy="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",dy=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${Tu}${uy}
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
gl_FragColor=vec4(col,1.0);}`,ip={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},sp={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${Tu}
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
}`},fy=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Tu}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,rp={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},py=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,my=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,op={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},ap={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function lp(s,e={}){let t=[],n=[],i=new lt;i.name="city-atmosphere",s.add(i);let r=(k,$)=>(t.push(k),n.push($),new je(k,$)),o={value:0},a={value:900},l={value:0},c={value:0},h={value:.25},u={value:0},d={value:5},f=new xe(4860440),g=(e.sunDirection||new z(-70,145,85)).clone().normalize(),M=new xe(330010),m=new xe(1323596),p=new xe(8011050),v=new xe(s.fog?.color||528926),S=(k,$,j,Q={})=>new mt({uniforms:j,vertexShader:k,fragmentShader:$,...Q}),_={transparent:!0,depthWrite:!1,blending:Xt},w=r(new pi(1500,48,24),S(Su,dy,{time:o,day:l,dusk:c,cloud:h,flash:u,octaves:d,glow:{value:f},zenith:{value:M},horizon:{value:m},haze:{value:v},warm:{value:p},sunDir:{value:g}},{side:tn,depthWrite:!1,fog:!1}));w.frustumCulled=!1,w.renderOrder=-10,i.add(w);let T=1400,L=new Float32Array(T*3),y=new Float32Array(T),I=new Float32Array(T),E=new Float32Array(T),R=12345,F=()=>(R=Math.imul(R,1664525)+1013904223>>>0)/4294967296;for(let k=0;k<T;k++){let $=F()*Math.PI*2,j=.03+F()*.97,Q=Math.sqrt(1-j*j);L.set([Math.cos($)*Q*1450,j*1450,Math.sin($)*Q*1450],k*3),y[k]=F()*Math.PI*2,I[k]=.4+F()*1.6,E[k]=8e-4+F()*F()*.0022}let X=new pt;X.setAttribute("position",new xt(L,3)),X.setAttribute("phase",new xt(y,1)),X.setAttribute("speed",new xt(I,1)),X.setAttribute("size",new xt(E,1));let P=S(ip.vertex,ip.fragment,{time:o,pointScale:a,cloud:h},{..._,fog:!1}),N=new xn(X,P);N.frustumCulled=!1,N.renderOrder=-9,i.add(N),t.push(X),n.push(P);let O=r(new ln(1800,1800),S(sp.vertex,sp.fragment,{time:o,day:l,glow:{value:f},island:{value:new Mt(0,-7,85,87)},deep:{value:new xe(398368)},shallow:{value:new xe(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:g},sunColor:{value:new xe(14676223)},fogColor:{value:new xe},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));O.rotation.x=-Math.PI/2,O.position.y=-3.2,i.add(O);let A=r(new ln(1900,1900),S(Su,fy,{time:o,strength:{value:.55},color:{value:new xe(1717320)},island:{value:new Mt(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));A.rotation.x=-Math.PI/2,A.position.y=-2.4,i.add(A);let V=500,Y=new Float32Array(V*3),G=new Float32Array(V*3);for(let k=0;k<V;k++)Y.set([(F()-.5)*180,2+F()*F()*75,-95+F()*170],k*3),G.set([F(),F(),F()],k*3);let ne=new pt;ne.setAttribute("position",new xt(Y,3)),ne.setAttribute("seed",new xt(G,3));let le=S(rp.vertex,rp.fragment,{time:o,pointScale:a,color:{value:new xe(10475775)},strength:{value:.32}},_),Ae=new xn(ne,le);Ae.frustumCulled=!1,i.add(Ae),t.push(ne),n.push(le);let Pe=new lt;Pe.position.set(0,86.5,-12),i.add(Pe);let Re=S(Su,py,{strength:{value:.45},color:{value:new xe(9430783)}},{..._,side:Nt}),oe=new ln(100,5.2);oe.translate(50,0,0),t.push(oe),n.push(Re);let pe=new je(oe,Re),ce=new je(oe,Re);ce.rotation.x=Math.PI/2;let Te=new lt;Te.add(pe,ce),Te.rotation.z=-.07,Pe.add(Te);let ue=new bt({color:12579583,toneMapped:!1}),he=r(new pi(.9,16,12),ue);Pe.add(he);let H=(e.lamps||[]).slice(0,64),Z=S("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",my,{color:{value:new xe(16767392)},strength:{value:.3}},{..._,side:Nt}),W=new Ps(2.7,6.1,18,1,!0),J=new vn(W,Z,Math.max(1,H.length));t.push(W),n.push(Z);let te=new St;H.forEach((k,$)=>{te.position.set(k.x+1.9*Math.cos(k.angle||0),3.35,k.z-1.9*Math.sin(k.angle||0)),te.updateMatrix(),J.setMatrixAt($,te.matrix)}),J.count=H.length,J.instanceMatrix.needsUpdate=!0,i.add(J);let fe=S(op.vertex,op.fragment,{color:{value:new xe(11128063)},strength:{value:0}},{..._,side:Nt,fog:!1}),Me=new es(11,.7,280,20,1,!0);Me.translate(0,140,0),t.push(Me),n.push(fe);let Oe=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([k,$,j])=>{let Q=new je(Me,fe);return Q.position.set(k,-3,$),Q.rotation.order="YXZ",Q.userData.offset=j,Q.frustumCulled=!1,i.add(Q),Q}),We=96,Ye=new Float32Array(We*3),Je=new Float32Array(We);for(let k=0;k<We;k++)Je[k]=F()*Math.PI*2;let q=new pt;q.setAttribute("position",new xt(Ye,3)),q.setAttribute("phase",new xt(Je,1)),q.setDrawRange(0,0);let Be=S(ap.vertex,ap.fragment,{time:o,pointScale:a},{..._,fog:!1}),$e=new xn(q,Be);$e.frustumCulled=!1,i.add($e),t.push(q),n.push(Be);let b=np(o),x=b.pass,U=!1,B="high",C=0;return{post:x,sea:O,setLighting(k,$,j){g.copy(j).normalize(),l.value=k,c.value=Math.min(1,$)*(1-k),M.set(330010).lerp(new xe(3108776),k).lerp(new xe(1909062),c.value*.8),m.set(1323596).lerp(new xe(11126484),k).lerp(new xe(12609598),c.value*.85),p.set($>.1?16747077:8011050),f.set(4860440).lerp(new xe(7027234),c.value),v.copy(s.fog.color),N.visible=k<.35,O.material.uniforms.sky.value.copy(m).lerp(M,c.value*.65).multiplyScalar(.7),O.material.uniforms.deep.value.set(398368).lerp(new xe(801368),k),O.material.uniforms.shallow.value.set(930640).lerp(new xe(2321036),k),A.material.uniforms.strength.value=.55*(1-k*.55),O.material.uniforms.sunColor.value.set(14676223).lerp(new xe(16752736),c.value)},setWeather(k){h.value=k==="rain"?.95:k==="fog"?.7:.25},setFlash(k){u.value=Math.max(0,Math.min(1,k))},setCinematic(k){b.setCinematic(k)},setAviation(k){let $=Math.min(We,k.length);for(let j=0;j<$;j++)Ye.set([k[j].x,k[j].y,k[j].z],j*3);q.attributes.position.needsUpdate=!0,q.setDrawRange(0,$)},setTier(k){B=k;let $=B!=="low";A.visible=$,Ae.visible=$,J.visible=$,Oe.forEach(j=>{j.visible=$}),d.value=B==="low"?3:B==="medium"?4:5,x.enabled=B==="high"||B==="ultra"},setBusy(k){U=!!k},setPointScale(k){a.value=k},update(k,$,j,Q){let me=Q?Math.min(.1,Math.max(0,k)):0;o.value+=me,s.fog&&(O.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(O.material.uniforms.fogDensity.value=s.fog.density)),O.position.x=j.position.x,O.position.z=j.position.z;let Ie=U?1.15:.32,be=Pe.userData.rate??Ie;Pe.userData.rate=be+(Ie-be)*Math.min(1,me*1.5),Te.rotation.y+=me*Pe.userData.rate,Re.uniforms.strength.value=U?.85:.45,he.scale.setScalar(1+Math.sin(o.value*(U?6:2.2))*.18),C+=me;let ye=1-l.value;fe.uniforms.strength.value=ye*ye*.075*(1-c.value*.7)*(1-h.value*.45),Oe.forEach((Fe,we)=>{let ze=Fe.userData.offset;Fe.rotation.y=Math.PI+Math.sin(C*(.11+we*.025)+ze)*.95,Fe.rotation.x=.22+.14*Math.sin(C*.13+ze*1.7)}),x.enabled&&b.update(me,j,g,{day:l.value,dusk:c.value,cloud:h.value,animated:Q})},stats:()=>({tier:B,busy:U,stars:T,dust:V,lamps:H.length,post:x.enabled,clouds:+h.value.toFixed(2),searchlights:Oe.length,aviation:q.drawRange.count,cinema:b.stats()}),dispose(){i.removeFromParent(),J.dispose(),t.forEach(k=>k.dispose()),n.forEach(k=>k.dispose()),b.dispose()}}}var cp=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function hp(s,e={}){let t=new lt;t.name="city-drones",s.add(t);let n=[],i=[],r=[],o=new z,a=new z,l=new z,c=new pi(.16,8,6);n.push(c);let h=v=>{let S=new bt({color:v,toneMapped:!1});return i.push(S),S},u=h(16730684),d=h(5046154),f=h(16777215);[...cp,...cp].forEach((v,S)=>{let _=new vr(v.points.map(y=>new z(y[0],y[1]+(S>=3?8:0),y[2])),!0,"centripetal",.6),w=new lt;w.name="city-drone-"+(S+1),w.visible=!1;let T=new lt;w.add(T);let L=[new je(c,u),new je(c,d),new je(c,f)];L[0].position.set(-2.6,.8,0),L[1].position.set(2.6,.8,0),L[2].position.set(0,2.1,-.4),w.add(...L),t.add(w),r.push({root:w,body:T,lights:L,curve:_,length:_.getLength(),speed:v.speed,t:S*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let g=!0,M="high",m=!1;function p(v,S){let{curve:_,root:w}=v,T=v.t;if(v.t=(v.t+S*v.speed*v.dir/v.length+1)%1,_.getPointAt(v.t,w.position),_.getTangentAt(v.t,o),o.multiplyScalar(v.dir),v.collider&&e.traffic){let I=v.collider,E=e.traffic;_.getPointAt((v.t+v.dir*20/v.length+1)%1,l),v.avoidTime=Math.max(0,(v.avoidTime||0)-S);let R=Math.max(w.position.y,E.ceiling(I,w.position.x,w.position.z),E.ceiling(I,l.x,l.z),v.avoidTime?v.avoidHeight:0);w.position.y=I.y+Ct.clamp(R-I.y,-S*5,S*7);let F=Math.atan2(o.x,o.z),X=I.heading+Math.atan2(Math.sin(F-I.heading),Math.cos(F-I.heading))*Math.min(1,S*3),P={...w.position,heading:X};if(v.obstruction=E.obstruction(I,I,P),v.obstruction){let N=E.body(v.obstruction),O=v.obstruction.startsWith("drone-")&&(Math.abs(I.y-N.y)>1?I.y<N.y:I.id<N.id);O||(v.avoidHeight=Math.min(170,I.y+8),v.avoidTime=8),P={...I,y:O?I.y:Math.min(170,I.y+S*5)},v.t=T}E.propose(I,P,(N,O)=>{N?v.waiting=0:(v.t=T,v.waiting+=S,v.waiting>2.5&&(v.dir=-v.dir,v.waiting=0)),w.position.set(O.x,O.y,O.z),w.rotation.set(0,O.heading,0)})}_.getTangentAt((v.t+.015)%1,a),v.collider||w.rotation.set(0,Math.atan2(o.x,o.z),0);let L=Math.atan2(a.x,a.z)-Math.atan2(o.x,o.z),y=Math.atan2(Math.sin(L),Math.cos(L));v.roll+=(Ct.clamp(y*12,-.55,.55)-v.roll)*(S>0?Math.min(1,S*3):1),v.body.rotation.z=v.roll}return{setTier(v){M=v},setTemplate(v){if(v){m=!0;for(let[S,_]of r.entries()){_.body.clear(),_.rotors.length=0;let w=v.clone(!0);w.scale.setScalar(1.9),w.updateMatrixWorld(!0);let T=new en().setFromObject(w);if(T.expandByScalar(.5),T.min.y-=1,T.max.y+=1,w.traverse(L=>{L.isMesh&&(L.castShadow=!1,L.receiveShadow=!1),/^rotor_/.test(L.name)&&_.rotors.push(L)}),_.body.add(w),_.root.visible=S<(M==="low"?2:M==="medium"?3:6),e.traffic&&!_.collider){p(_,0);let L=ki({min:T.min.toArray(),max:T.max.toArray()});_.root.position.y=Math.max(_.root.position.y,e.traffic.ceiling(L,_.root.position.x,_.root.position.z)),_.collider=e.traffic.register("drone-"+S,L,{..._.root.position,heading:_.root.rotation.y},0),_.collider||(_.root.visible=!1)}}}},update(v,S,_){g=_;let w=_?Math.min(.1,Math.max(0,v)):0;r.forEach((T,L)=>{if(T.root.visible=m&&L<(M==="low"?2:M==="medium"?3:6)&&(!e.traffic||!!T.collider),T.collider&&(T.root.visible&&!T.collider.enabled&&!e.traffic.relocate(T.collider,T.collider)&&(T.root.visible=!1),T.collider.enabled=T.root.visible),!T.root.visible)return;p(T,w),T.rotors.forEach((I,E)=>{I.rotation.y+=w*(E%2?-46:46)});let y=Math.sin(S*5+L*1.7);T.lights[0].visible=T.lights[1].visible=!_||y>-.2,T.lights[2].visible=!_||y>.93,T.lights[2].scale.setScalar(_?1.6:1)})},stats:()=>({drones:r.filter(v=>v.root.visible).length,animated:g,positions:r.map(v=>v.root.position.toArray().map(S=>Math.round(S))),obstructions:r.map(v=>v.obstruction)}),dispose(){t.removeFromParent(),n.forEach(v=>v.dispose()),i.forEach(v=>v.dispose()),r.forEach(v=>{v.collider&&e.traffic.remove(v.collider.id)}),r.length=0}}}var En={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80},ht={ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16},_n=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),qn=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],Eu=[[-67,-77],[-67,59],[67,59],[67,-77]],bi={x:78,z:30},_c=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function up(s,e,t=En.halfWidth){return s>=En.minX&&s<=En.maxX&&e>=En.minZ&&e<=En.maxZ&&(En.xs.some(n=>Math.abs(s-n)<=t)||En.zs.some(n=>Math.abs(e-n)<=t))}function dp(s,e){return up(s,e)?up(s,e,En.laneHalfWidth)?ht.road:ht.pavement:ht.ground}function gy(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function fp(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function pp(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,o)=>{r>n&&o>i&&s.push({x:(n+r)/2,z:(i+o)/2,sx:(r-n)/4,sz:(o-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,o,a,l]=[n-2,i-2,n+2,i+2],[c,h,u,d]=[Math.max(r,e[0]),Math.max(o,e[1]),Math.min(a,e[2]),Math.min(l,e[3])];if(c>=u||h>=d){t(r,o,a,l);continue}t(r,o,c,l),t(u,o,a,l),t(c,o,u,h),t(c,d,u,l)}return s}function wu(s,e,t,n){return gy(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Au={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},xy={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]};function mp(s,e,t){for(let n of t){let i=xy[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function _y(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Au[n.id]||[]).some(([o,a,l,c])=>i>o&&i<l&&r>a&&r<c))return!0}return _c.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function ks(s,e){return _n.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function us(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(ks(s,e))return ht.room;for(let t of qn){let n=!!t.angle&&Math.abs(t.angle)!==Math.PI;if(Math.abs(s-t.platformX)<(n?2:4.5)&&Math.abs(e-t.platformZ)<(n?4.5:2))return ht.pavement+.3}return Math.max(dp(s,e),e>=74&&e<=80&&s>=-80&&s<=80?ht.quay:ht.ground)}function gp(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=ks(s,e),o=ks(t.x,t.z);if(r||o){let a=r||o;return!(r?.id!==o?.id&&(Math.abs(s-a.x)>1.15||Math.abs(t.x-a.x)>1.15||Math.min((e-a.doorZ)*a.front,(t.z-a.doorZ)*a.front)<-.9||!n(a.id))||r&&(Math.abs(s-a.x)>5.55||(e-a.z)*a.front<-5.55))}return!_y(s,e,i)}function vc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var ds=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}];function xp(s,e){let t=[],n=new Map,i=-1;for(let o of[-73,-61,-24,-12,12,24,61,73])for(let a of[-83,-71,-38,-26,7,19,53,65])t.push({x:o,z:a,y:e(o,a),heading:0});function r(o,a){i!==s.revision()&&(n.clear(),i=s.revision());let l=o.circles.map(v=>[v.x,v.z,v.r].join(",")).join(";")+":"+o.maxY;if(!n.has(l)){let v=t.filter(_=>s.clear(o,_,_,!1)),S=v.map(()=>[]);for(let _=0;_<v.length;_++)for(let w=_+1;w<v.length;w++){let T=v[_],L=v[w],y=Math.hypot(T.x-L.x,T.z-L.z);y<48&&s.clear(o,T,L,!1)&&(S[_].push([w,y]),S[w].push([_,y]))}n.set(l,{nodes:v,edges:S})}let c=n.get(l),h=[...c.nodes,{x:o.x,y:o.y,z:o.z,heading:o.heading},{...a,heading:0}],u=h.length-2,d=h.length-1,f=c.edges.map(v=>[...v]);f.push([],[]);for(let v of[u,d])for(let S=0;S<v;S++){let _=h[v],w=h[S],T=Math.hypot(_.x-w.x,_.z-w.z);(T<48||S===u)&&s.clear(o,_,w,!1)&&(f[v].push([S,T]),f[S].push([v,T]))}let g=h.map(()=>1/0),M=h.map(()=>-1),m=new Set(h.map((v,S)=>S));for(g[u]=0;m.size;){let v=-1;for(let S of m)(v<0||g[S]<g[v])&&(v=S);if(!Number.isFinite(g[v])||v===d)break;m.delete(v);for(let[S,_]of f[v])g[v]+_<g[S]&&(g[S]=g[v]+_,M[S]=v)}if(!Number.isFinite(g[d]))return[];let p=[];for(let v=d;v!==u;v=M[v]){if(v<0)return[];p.unshift(h[v])}return p}return{route:r,points:t}}function _p(s){let e=new lt;e.name="city-conversations",s.add(e);let t=new Dn(1,.025,5,40,Math.PI*1.45),n=new Is(.92,1,48),i=new z(0,0,1),r=new z,o=[],a=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let u=new bt({color:8445392,transparent:!0,opacity:0,blending:Xt,depthWrite:!1,side:Nt,toneMapped:!1}),d=new je(h===3?n:t,u);d.visible=!1,e.add(d),c.push(d)}o.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let u=o.find(d=>d.age>=2.4);return u?(Object.assign(u,{from:l,to:c,age:0,source:h}),a++,u.meshes.forEach(d=>d.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of o){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let u=h.from,d=h.to,f=d.x-u.x,g=d.z-u.z;r.set(f,d.y+Math.min(2,d.maxY*.7)-(u.y+Math.min(2,u.maxY*.7)),g).normalize();for(let m=0;m<3;m++){let p=(h.age-m*.18)/1.55;if(p<0||p>1)continue;let v=h.meshes[m];v.visible=!0,v.position.set(u.x+f*p,Ct.lerp(u.y+Math.min(2,u.maxY*.7),d.y+Math.min(2,d.maxY*.7),p)+Math.sin(p*Math.PI)*.65,u.z+g*p),v.quaternion.setFromUnitVectors(i,r),v.rotateZ(m*.65),v.scale.setScalar(.25+Math.sin(p*Math.PI)*.65),v.material.opacity=Math.sin(Math.PI*p)*.75}let M=(h.age-1.55)/.85;if(M>0){let m=h.meshes[3];m.visible=!0,m.position.set(d.x,d.y+Math.min(2,d.maxY*.7),d.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-M)+.12),m.material.opacity=Math.sin(M*Math.PI)*.85}}},clear(){o.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:a,active:o.filter(l=>l.age<2.4).length,sources:o.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),o.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var Ru=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5);function vp(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:o}){let a=[],l=new Map,c=[],h=[],u=xp(e,n),d=_p(s),f=0,g=null,M=!1,m="high",p=!1,v=0,S="",_=()=>m==="low"?3:m==="medium"?10:19;function w(P){P.slot&&(P.slot.owner=null,P.slot=null),P.path=[],P.goal=null}function T(P){return P?{id:P.id,role:P.role,state:P.state,source:P.source||"ambient",guided:!!P.guide}:null}function L(P){g=P?.id||null,S=P?.state||"",i?.(T(P))}function y(P,N,O){let A="resident-"+N,V=ki(O,1.15),Y=["courier","technician","archivist"][N%3],G=null;V.circles=[{x:0,z:0,r:V.reach}];let le=[...c.filter(Pe=>Pe.role===Y).map(Pe=>({x:Pe.x,z:Pe.z+2.2,y:n(Pe.x,Pe.z+2.2),heading:Math.PI})),...u.points.map((Pe,Re)=>u.points[(N*7+Re)%u.points.length])];for(let Pe of le)if(G=e.register(A,V,Pe,1),G)break;if(!G)return P.node.visible=!1,null;let Ae={...P,id:A,index:N,body:G,role:Y,state:"idle",path:[],until:N*.3,trip:N,cooldown:8+N*.7,source:"ambient",carrying:!1,parcels:[]};return P.node.traverse(Pe=>{Pe.isMesh&&/parcel/.test(Pe.name)&&(Ae.parcels.push(Pe),Pe.visible=!1)}),a.push(Ae),P.node.position.set(G.x,G.y,G.z),Ae}function I(P,N,O,A){let V={id:P,body:N,node:O,pause:A,role:"patrol",state:"patrol",cooldown:7+a.length,path:[]};return a.push(V),V}function E(P){w(P),P.source="ambient",P.state="idle",P.until=f+2;let N=c.filter(O=>!O.owner&&(P.role==="courier"?O.place===(P.carrying?"meeting-charge":"parcel-sorter"):O.role===P.role||O.place==="meeting-charge"));for(let O=0;O<N.length;O++){let A=N[(P.trip+O)%N.length],V=u.route(P.body,A);if(V.length){A.owner=P.id,P.slot=A,P.path=V,P.goal=A,P.trip++,P.state=P.carrying?"carry":"walk";return}}for(let O=0;O<8;O++){let A=u.points[(P.trip++*13+P.index)%u.points.length],V=u.route(P.body,A);if(V.length){P.path=V,P.state="walk";return}}}function R(P){for(let N of[P.a,P.b])w(N),N.meeting=null,N.cooldown=f+12+(N.index||0),N.pause?.(!1),N.state=N.pause?"patrol":"idle",N.until=f+1}function F(){h.forEach(R),h.length=0,d.clear(),g=null,i?.(null);for(let P of a)P.guide=null,P.pause?.(!1,P.body),P.trafficYield=!1,P.yieldTram=null,P.pause||(w(P),P.state="idle",P.until=f+1)}function X(P,N){if(p)return;N&&(f+=P);for(let A of a){let V=A.pause||A.index<_();if(V&&!A.body.enabled){if(!e.relocate(A.body,A.body)){A.node.visible=!1;continue}A.body.enabled=!0,A.node.position.set(A.body.x,A.body.y,A.body.z)}if(!V&&A.body.enabled&&(w(A),A.body.enabled=!1,A.meeting&&R(A.meeting)),A.node.visible=!!V,!V||!N)continue;let Y=e.neighbors(A.body,22).sort((G,ne)=>Math.hypot(G.x-A.body.x,G.z-A.body.z)-Math.hypot(ne.x-A.body.x,ne.z-A.body.z)).find(G=>{if(!G.id.startsWith("tram-"))return!1;let ne=A.body.x-G.x,le=A.body.z-G.z,Ae=Math.cos(G.heading),Pe=Math.sin(G.heading);return Math.abs(ne*Ae-le*Pe)<A.body.reach+(A.trafficYield?4:2.3)&&ne*Pe+le*Ae>-8-A.body.reach&&ne*Pe+le*Ae<14});if(!Y&&A.yieldTram){let G=e.body(A.yieldTram),ne=G?A.body.x-G.x:0,le=G?A.body.z-G.z:0;if(G?.enabled&&Math.hypot(ne,le)<26&&ne*Math.sin(G.heading)+le*Math.cos(G.heading)>-G.reach-A.body.reach-1){A.pause?.(!0),A.play?.("idle");continue}A.yieldTram=null}if(Y){if(A.trafficYield=!0,A.yieldTram=Y.id,A.meeting){let ce=A.meeting;R(ce),h.splice(h.indexOf(ce),1)}A.pause?.(!0);let G=Math.cos(Y.heading),ne=Math.sin(Y.heading),le=Math.sign((A.body.x-Y.x)*G-(A.body.z-Y.z)*ne)||1,Ae=(A.body.x-Y.x)*G-(A.body.z-Y.z)*ne,Pe=Math.sign((A.body.x-Y.x)*ne+(A.body.z-Y.z)*G)||1,Re=[[G*le,-ne*le,A.body.reach+4.5-Math.abs(Ae)],[-G*le,ne*le,A.body.reach+4.5+Math.abs(Ae)],[ne*Pe,G*Pe,1.2]],oe=Array.from({length:16},(ce,Te)=>[Math.cos(Te*Math.PI/8),Math.sin(Te*Math.PI/8),2]),pe=([ce,Te])=>ce*(A.body.x-Y.x)+Te*(A.body.z-Y.z);Re.push(...oe.filter(ce=>pe(ce)>.2).sort((ce,Te)=>pe(Te)-pe(ce)));for(let[ce,Te,ue]of Re){let he={x:A.body.x+ce*ue,y:A.body.y,z:A.body.z+Te*ue,heading:A.body.heading};if(!e.clear(A.body,A.body,he))continue;let H={...he,x:A.body.x+ce*P*1.8,z:A.body.z+Te*P*1.8};H.y=A.pause?A.body.y:n(H.x,H.z),e.propose(A.body,H,(Z,W)=>{A.node.position.set(W.x,W.y,W.z),A.play?.("walk")}),A.state="yield",A.cooldown=f+5;break}continue}if(A.trafficYield&&(A.trafficYield=!1,A.pause?.(!1,A.body),A.state=A.pause?"patrol":A.path.length?"walk":"idle"),!A.meeting){if(A.pause){A.state==="greet"&&f>A.until&&(A.pause(!1),A.state="patrol");continue}if(A.guide&&Math.hypot(t.position.x-A.body.x,t.position.z-A.body.z)>9){A.play("idle");continue}if(A.path.length){let G=A.path[0],ne=G.x-A.body.x,le=G.z-A.body.z,Ae=Math.hypot(ne,le),Pe=A.guide?1.9:1.65;if(Ae<.18){A.path.shift();continue}let Re=Math.min(Ae,Pe*P),oe={x:ne/Ae,z:le/Ae},pe=A.body.x+oe.x*Re,ce=A.body.z+oe.z*Re,Te=Ru(A.body.heading,Math.atan2(ne,le),P),ue={x:A.body.x+oe.x*1.8,y:n(pe,ce),z:A.body.z+oe.z*1.8,heading:Te};if(!e.clear(A.body,A.body,ue)){let H=!1;for(let[Z,W]of[[oe.z,-oe.x],[-oe.z,oe.x],[-oe.x,-oe.z]]){let J={x:A.body.x+Z*1.3,y:A.body.y,z:A.body.z+W*1.3,heading:A.body.heading};if(e.clear(A.body,A.body,J)){pe=A.body.x+Z*Re,ce=A.body.z+W*Re,Te=Ru(A.body.heading,Math.atan2(Z,W),P),H=!0;break}}H||(pe=A.body.x,ce=A.body.z)}e.clear(A.body,A.body,{x:pe,y:n(pe,ce),z:ce,heading:Te},!1)||(Te=A.body.heading);let he=Math.hypot(pe-A.body.x,ce-A.body.z)>.001;e.propose(A.body,{x:pe,y:n(pe,ce),z:ce,heading:Te},(H,Z)=>{A.node.position.set(Z.x,Z.y,Z.z),A.node.rotation.y=Z.heading,!H||!he?(A.wait=(A.wait||0)+P,A.play("idle"),A.state="yield"):(A.wait=0,A.state=A.guide?"guide":A.carrying?"carry":"walk",A.play(A.carrying?"carry":"walk")),A.wait>2.5&&(A.wait=0,w(A),A.state=A.guide?"guide":"idle",A.until=f+.5,A.guide&&(A.path=u.route(A.body,A.guide)))})}else if(A.state==="walk"||A.state==="carry"||A.state==="guide"||A.state==="yield"){if(A.guide&&Math.hypot(A.body.x-A.guide.x,A.body.z-A.guide.z)>.5){A.play("idle"),f>A.until&&(A.path=u.route(A.body,A.guide),A.until=f+2.5);continue}A.guide=null,A.state=A.slot?.place==="meeting-charge"?"charge":A.slot?"work":"idle",A.until=f+5+A.index%4,A.play(A.state==="work"?"work":"idle"),A.slot&&r?.(A.slot.place,5),v++}else f>A.until&&(A.role==="courier"&&A.slot&&(A.state==="work"||A.state==="charge")&&(A.carrying=A.slot.place==="parcel-sorter",A.parcels.forEach(G=>{G.visible=A.carrying})),E(A))}}if(!N){d.update(0,!1);return}for(let A of a){if(!A.body.enabled||A.trafficYield||A.meeting||A.guide||A.state==="greet"||f<A.cooldown)continue;let V=a.find(G=>G!==A&&G.body.enabled&&!G.trafficYield&&G.state!=="greet"&&!G.meeting&&!G.guide&&f>=G.cooldown&&Math.abs(A.body.y-G.body.y)<2&&Math.hypot(A.body.x-G.body.x,A.body.z-G.body.z)>2.8&&Math.hypot(A.body.x-G.body.x,A.body.z-G.body.z)<6);if(!V)continue;let Y={a:A,b:V,at:f,sent:!1,replied:!1};h.push(Y);for(let G of[A,V])G.meeting=Y,G.state="exchange",G.pause?.(!0),G.play?.("greet")}for(let A=h.length-1;A>=0;A--){let V=h[A],Y=f-V.at;if(Y>4.8||!V.a.body.enabled||!V.b.body.enabled){R(V),h.splice(A,1);continue}for(let[G,ne]of[[V.a,V.b],[V.b,V.a]])e.propose(G.body,{...G.body,heading:Ru(G.body.heading,Math.atan2(ne.body.x-G.body.x,ne.body.z-G.body.z),P)},(le,Ae)=>{G.node.rotation.y=Ae.heading});Y>.6&&!V.sent&&(d.send(V.a.body,V.b.body),V.sent=!0),Y>2.4&&!V.replied&&(d.send(V.b.body,V.a.body),V.replied=!0)}d.update(P,!0);let O=a.find(A=>A.id===g);O&&O.state!==S&&(S=O.state,i?.({...T(O),refresh:!0}))}return{add:y,patrol:I,update:X,suspend:F,addPlace(P,N){if(!l.has(P.id)){l.set(P.id,P);for(let O of N.navigation.workpoints||[])c.push({id:P.id+":"+O.id,place:P.id,role:P.role,x:P.x+O.position[0],y:n(P.x,P.z),z:P.z+O.position[2],owner:null})}},nearby(){let P=[];for(let N of a)if(N.body.enabled){let O=Math.hypot(t.position.x-N.body.x,t.position.z-N.body.z);O<5&&Math.abs(t.position.y-N.body.y)<8&&P.push({kind:"resident",id:N.id,distance:O,x:N.body.x,z:N.body.z})}for(let N of l.values()){let O=Math.hypot(t.position.x-N.x,t.position.z-N.z);O<7&&P.push({kind:"demonstrate",id:N.id,distance:O,x:N.x,z:N.z})}return P},inspect(P){let N=a.find(O=>O.id===P);return L(N),T(N)},action(P,N){let O=a.find(A=>A.id===g);if(!O)return null;if(O.meeting){let A=O.meeting,V=h.indexOf(A);R(A),V>=0&&h.splice(V,1)}if(P==="greet"&&(w(O),O.guide=null,O.pause?.(!0),O.state="greet",O.until=f+3,O.cooldown=f+5,O.play?.("greet")),P==="guide"&&!O.pause&&N){w(O);for(let A of[0,1.5,3]){for(let V=0;V<(A?8:1)&&!O.path.length;V++){let Y=N.x+Math.cos(V*Math.PI/4)*A,G=N.z+Math.sin(V*Math.PI/4)*A;O.path=u.route(O.body,{x:Y,z:G,y:n(Y,G)})}if(O.path.length)break}O.guide=O.path.length?O.path[O.path.length-1]:null,O.state=O.guide?"guide":"idle"}return P==="cancel"&&(w(O),O.guide=null,O.pause?.(!1),O.state=O.pause?"patrol":"idle",O.until=f+1),L(O),T(O)},demonstrate(P,N=!1){l.has(P)&&r?.(P,N?0:6)},setTier(P){m=P},setReplay(P){M!==P&&(M=P,F())},event(P){if(M||!o()||!["started","succeeded","failed","sanitized","progress"].includes(P.state)||!Number.isFinite(P.at)||Date.now()-P.at>2500||P.at>Date.now())return;let N=ds.find(O=>O.district===(P.to==="agent"?P.from:P.to));N&&l.has(N.id)&&r?.(N.id,2,P.state)},stats:()=>({completed:v,meetings:h.length,reservations:c.filter(P=>P.owner).length,signals:d.stats(),residents:a.filter(P=>P.body.enabled).map(P=>({...T(P),x:P.body.x,z:P.body.z})),places:[...l.keys()]}),dispose(){p=!0,F(),a.forEach(P=>e.remove(P.id)),a.length=0,l.clear(),c.length=0,d.dispose()}}}function yp(s){let e=new lt;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,o="high",a=S=>(t.add(S),S),l=S=>(n.add(S),S),c=a(new Dn(1,.025,6,64)),h=a(new _r(1,48)),u=a(new pt),d=new Float32Array(288),f=new Float32Array(96);for(let S=0;S<96;S++)d[S*3]=Math.cos(S*2.399)*Math.sqrt(S/96),d[S*3+2]=Math.sin(S*2.399)*Math.sqrt(S/96),f[S]=S/96;u.setAttribute("position",new xt(d,3)),u.setAttribute("phase",new xt(f,1));for(let S of ds){let _=new lt;_.name=S.id,_.position.set(S.x,.12,S.z),_.visible=!1,e.add(_);let w=l(new bt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:Xt,toneMapped:!1})),T=[];for(let I=0;I<3;I++){let E=new je(c,l(w.clone()));E.rotation.x=-Math.PI/2,_.add(E),T.push(E)}let L=new je(h,l(w.clone()));L.rotation.x=-Math.PI/2,L.scale.setScalar(2.5),L.position.y=.2,_.add(L);let y=new xn(u,l(new mt({transparent:!0,depthWrite:!1,blending:Xt,uniforms:{time:{value:0},strength:{value:0},color:{value:new xe(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));_.add(y),i.set(S.id,{group:_,waves:T,base:L,motes:y,district:S.district,started:0,until:0,source:"ambient",model:null})}let g=new je(a(new ln(3.9,2.3)),l(new mt({transparent:!0,depthWrite:!1,side:Nt,blending:Xt,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));g.position.set(0,1.4,.8),i.get("repair-bay").group.add(g);let M=i.get("kinetic-fountain"),m=l(new mt({transparent:!0,depthWrite:!1,side:Nt,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let S=0;S<8;S++){let _=S*Math.PI/4,w=new z(Math.cos(_)*2.3,.5,Math.sin(_)*2.3),T=new z(Math.cos(_+.5)*.7,.55,Math.sin(_+.5)*.7),L=new Qn(w,new z(Math.cos(_)*1.5,3.7,Math.sin(_)*1.5),T);M.group.add(new je(a(new fo(L,24,.045,5,!1)),m))}let p=l(new mt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),v=new je(a(new _r(2.58,64)),p);return v.rotation.x=-Math.PI/2,v.position.y=.3,M.group.add(v),{attach(S,_){let w=i.get(S);w&&(w.model=_,w.group.visible=!0)},demonstrate(S,_=6,w=null){let T=i.get(S);T&&(T.until<=r&&(T.started=r),T.until=r+_,T.source=w?"live":"ambient",T.status=w,T.model?.play("operate"))},clear(){i.forEach(S=>{S.until=0,S.source="ambient",S.status=null})},setTier(S){o=S},update(S,_){_&&(r+=S),m.uniforms.time.value=p.uniforms.time.value=g.material.uniforms.time.value=r;for(let[w,T]of i){let L=T.until>r,y=w==="kinetic-fountain",I=L?Ct.smoothstep(r-T.started,0,.35)*Ct.smoothstep(T.until-r,0,.65):0,E=y?Math.max(.25,I):I;L||(T.source="ambient",T.status=null);let R=T.status==="failed"?16737881:T.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[T.district]:7529695;if(T.waves.forEach((F,X)=>{F.visible=L;let P=(r*.6+X/3)%1;F.position.y=.4+P*2,F.scale.setScalar(.4+P*2.6),F.material.opacity=(1-P)*.22*I,F.material.color.setHex(R)}),w==="repair-bay"&&(g.visible=L,g.material.uniforms.strength.value=I),w==="relay-mast"&&L){let F=T.model?.node.getObjectByName("antenna");F&&(F.rotation.y=Ct.damp(F.rotation.y,Math.atan2(-T.group.position.x,-12-T.group.position.z),3,_?S:0))}T.base.material.opacity=.08*I,T.motes.visible=o!=="low"&&E>0,T.motes.material.uniforms.time.value=r,T.motes.material.uniforms.strength.value=E,T.model?.mixer&&(T.model.mixer.timeScale=_&&(L||y)?.65:0)}},stats:()=>({places:[...i].filter(([,S])=>S.model).map(([S,_])=>({id:S,operating:_.until>r,source:_.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(S=>S.dispose()),n.forEach(S=>S.dispose()),i.clear()}}}function Mp(s,e){let t=new lt;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,o=[],a=[],l=[],c=new Map,h=new Map,u=new Map,d=new Map,f=new Set,g=[],M=new Set,m=[],p=new Map,v=new AbortController,S=[],_=0,w=!1,T=!1,L=e.tier||"high",y=null,I=0,E=0,R=null,F=null,X=0,P=new Set,N=null,O=0,A=new z,V=new z,Y=new z,G=e.camera,ne=e.traffic||gc(),le=yp(s),Ae=new Map;function Pe(b,x){let U=us(b,x);for(let B of ds)for(let C of y?.assets.find(k=>k.id===B.id)?.navigation.surfaces||[]){let[k,$,j,Q]=C.rect;b>=B.x+k&&b<=B.x+j&&x>=B.z+$&&x<=B.z+Q&&(U=Math.max(U,us(B.x,B.z)+C.height))}return U}let Re=vp(s,{traffic:ne,camera:G,floor:Pe,onSelect:e.onSociety,onDemonstrate:le.demonstrate,active:e.active}),oe=0,pe=0;try{P=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(b=>qn.some(x=>x.id===b)))}catch{}let ce=yu(Eu),Te=ce.getLength(),ue=qn.map(b=>{let x=1/0,U=0;for(let B=0;B<3e3;B++){ce.getPointAt(B/3e3,A);let C=Math.hypot(A.x-b.x,A.z-b.z);C<x&&(x=C,U=B/3e3)}return{...b,u:U}}),he=(b,x)=>e.onSound?.(b,x?.x||0,x?.y||0,x?.z||0,!!N),H=new yn({color:11834208,metalness:.8,roughness:.35});r.add(H);for(let b of[-1,1]){let x=[],U=[];for(let k=0;k<=512;k++){ce.getPointAt(k/512,A),ce.getTangentAt(k/512,V);for(let $ of[-.045,.045]){let j=b*.85+$;x.push(A.x-V.z*j,.21,A.z+V.x*j)}if(k<512){let $=k*2;U.push($,$+1,$+2,$+1,$+3,$+2)}}let C=new pt;C.setAttribute("position",new nt(x,3)),C.setIndex(U),C.computeVertexNormals(),i.add(C),t.add(new je(C,H))}async function Z(b,x=L==="low"?2:L==="medium"?1:0){let U=b+":"+x;return n.has(U)||n.set(U,(async()=>{let B=y.assets.find(Q=>Q.id===b),C=B?.lods.find(Q=>Q.level===x);if(!C||!/^[\w-]+\.lod[0-2]\.glb$/.test(C.file))throw Error("Invalid world asset");let k=await fetch(e.assetURL(C.file),{signal:v.signal});if(!k.ok)throw Error("World asset unavailable");let $=await k.arrayBuffer();if(T)throw Error("Disposed");I+=$.byteLength;let j=await new hs().parseAsync($,"");if(j.scene.traverse(Q=>{if(Q.isMesh){i.add(Q.geometry);for(let me of Array.isArray(Q.material)?Q.material:[Q.material])r.add(me);Q.castShadow=!0,Q.receiveShadow=!0}}),T)throw i.forEach(Q=>Q.dispose()),r.forEach(Q=>Q.dispose()),Error("Disposed");return f.add(b),j})()),n.get(U)}async function W(b,x,U,B=0,C=0,k=t,$=[1,1,1],j){let Q=await Z(b,j);if(T)return null;let me=Q.scene.clone(!0);me.position.set(x,B,U),me.rotation.y=C,me.scale.set(...$),k.add(me),me.traverse(we=>{we.isMesh&&(we.userData.worldAsset=b,we.userData.worldPart=we.name,g.push(we))});let Ie=y.assets.find(we=>we.id===b),be="furnishing:"+oe++;for(let[we,ze]of(Ie.navigation.colliders||[]).entries())ne.solid(be+":"+we,{owner:be,x,z:U,heading:C,min:[ze[0]*$[0],B+ze[1]*$[1],ze[2]*$[2]],max:[ze[3]*$[0],B+ze[4]*$[1],ze[5]*$[2]]});let ye=Q.animations.length?new Eo(me):null;ye&&o.push(ye);let Fe=null;return{node:me,mixer:ye,owner:be,bounds:Ie.motion_bounds||Ie.lods[0].bounds,clips:Q.animations,play(we,ze=!1){if(!ye)return;let K=Q.animations.find(de=>de.name===we);if(!K)return;let _e=ye.clipAction(K);return Fe===_e||(_e.reset(),ze&&(_e.setLoop(Kl,1),_e.clampWhenFinished=!0),_e.play(),Fe&&_e.crossFadeFrom(Fe,.25,!1),Fe=_e),_e}}}function J(b){b.updateMatrixWorld(!0);let x=new Map;b.traverse(B=>{if(!B.isMesh)return;let C=B.geometry.uuid+":"+(Array.isArray(B.material)?B.material.map(k=>k.uuid).join(","):B.material.uuid);x.has(C)||x.set(C,{n:B,matrices:[]}),x.get(C).matrices.push(B.matrixWorld.clone())});let U=new lt;t.add(U);for(let{n:B,matrices:C}of x.values()){let k=new vn(B.geometry,B.material,C.length);C.forEach(($,j)=>k.setMatrixAt(j,$)),k.castShadow=!0,k.receiveShadow=!0,k.userData={...B.userData},g.push(k),U.add(k)}return b.traverse(B=>{let C=g.indexOf(B);C>=0&&g.splice(C,1)}),b.removeFromParent(),u.set(U.uuid,U),U}function te(b){if(d.has(b.id))return d.get(b.id);let x=(async()=>{let U=new lt;t.add(U);let B=[];for(let C of pp())B.push(W("floor",b.x+C.x,b.z+C.z,ht.room,0,U,[C.sx,1,C.sz]));for(let C=-4;C<=4;C+=4)for(let k=-4;k<=4;k+=4)B.push(W("ceiling",b.x+C,b.z+k,8.15+ht.room,0,U));for(let C=-4;C<=4;C+=4)B.push(W("wall",b.x+C,b.z+6,ht.room,0,U));for(let C=-4;C<=4;C+=4)for(let k of[-1,1])for(let $ of[0,4])B.push(W("window",b.x+k*6,b.z+C,ht.room+$,Math.PI/2,U));for(let C=-4;C<=4;C+=4)for(let k of[-1,1])B.push(W("window",b.x+C,b.z+k*6,ht.gallery,0,U));for(let C of[-4,4])B.push(W("wall",b.x+C,b.doorZ,ht.room,0,U));for(let[C,k]of[[-4,1],[.2,1.1]])B.push(W("floor",b.x+C,b.z+3,ht.gallery,0,U,[k,1,1.5])),B.push(W("railing",b.x+C,b.z,ht.gallery,0,U,[k,1,1]));for(let C of[.7,5.3])B.push(W("railing",b.x+2.4,b.z+C,ht.gallery,Math.PI/2,U,[.35,1,1]));for(let C of[-5.6,1.9])B.push(W("wall",b.x+C,b.z+.15,ht.room,0,U,[.05,1,.6]));if(await Promise.all(B),!T)return J(U)})();return d.set(b.id,x),x}async function fe(b){if(!(c.has(b.id)||T)){c.set(b.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),O++;try{await te(b);let x=new lt;t.add(x);let U=[];for(let C of[-3,0])U.push(W(b.id==="missions"?"cargo":b.id==="memory"?"archive-shelf":"console",b.x+C,b.z+3,ht.room,Math.PI,x));if(U.push(W("bench",b.x-4,b.z-1,ht.room,Math.PI/2,x)),U.push(W("bench",b.x-4,b.z+4.8,ht.gallery,0,x)),U.push(W("console",b.x,b.z+4.8,ht.gallery,Math.PI,x)),await Promise.all(U),T)return;J(x);let B=c.get(b.id);B.hologram=await W("hologram",b.x-1,b.z,ht.room),B.hologram?.play("operate"),b.id==="memory"&&We(),B.lift=await W("lift",b.liftX,b.liftZ,ht.room),B.liftAction=B.lift?.play("operate",!0),B.liftAction&&(B.liftAction.paused=!0),B.ready=!0}catch(x){T||e.onError?.(x)}finally{O--}}}async function Me(){try{let b=await fetch(e.assetURL("manifest.json"),{signal:v.signal});if(!b.ok)throw Error("World manifest unavailable");y=await b.json();let x=new lt;t.add(x);let U=[];for(let C of qn)U.push(W("station",C.platformX,C.platformZ,ht.pavement,C.angle,x));for(let C of _n){U.push(te(C));let k=await W("door",C.x,C.doorZ,ht.room,Math.PI);if(T)return;let $=k.play("open",!0);$.paused=!0,h.set(C.id,{...k,action:$,value:0,open:!1})}for(let C=-76;C<=76;C+=8)U.push(W("quay",C,77,ht.quay,0,x));for(let[C,k]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])U.push(W("garden",C,k,0,0,x)),U.push(W("bench",C+3,k,0,Math.PI/2,x));for(let[C,k]of[[-30,23],[30,23],[-28,72]])U.push(W("arcade",C,k,0,0,x));U.push(W("bridge",-78.5,40,2,0,x,[1.75,1,1]),W("ramp",-80,49,0,Math.PI,x),W("stairs",-76.8,49,0,Math.PI,x),W("ramp",-78.5,31,0,0,x)),U.push(W("pad",bi.x,bi.z,0,0,x));for(let[C,k]of[[-60,-56],[-60,-46],[51,49]])U.push(W("charger",C,k,0,0,x));if(await Promise.all(U),T)return;J(x);for(let[C,k]of[[-57,-61],[-57,-52]])(await W("cooler",C,k))?.play("operate");for(let C of ds){let k=await W(C.id,C.x,C.z,us(C.x,C.z),0,t,[1,1,1],2);if(T)return;k&&(k.level=2,Ae.set(C.id,k),le.attach(C.id,k),Re.addPlace(C,y.assets.find($=>$.id===C.id)),k.play("operate"))}let B=await W("service-cart",55,49,ht.ground,Math.PI/2);B&&B.play("open",!0);for(let C=0;C<19;C++){let k=["courier","technician","archivist"][C%3],$=await W("robot-"+k,0,0);if(!$)return;$.node.scale.setScalar(1.15),a.push($),Re.add($,C,$.bounds),$.play("idle")}for(let C=0;C<2;C++){let k=await W("tram",0,0);if(!k)return;let $=k.play("open",!0);$.paused=!0;let j=ue[C?3:0].u;ce.getPointAt(j,A),ce.getTangentAt(j,V);let Q=ne.register("tram-"+C,ki(k.bounds),{x:A.x,y:ht.road+.03,z:A.z,heading:Math.atan2(V.x,V.z)},3);l.push({...k,body:Q,u:j,speed:0,dwell:5,stop:C?3:0,door:$})}for(let C=0;C<3;C++){let k=await W("cargo",43,70);k&&(ne.removeOwner(k.owner),k.node.visible=!1,m.push({...k,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(b){T||e.onError?.(b)}}Me();async function Oe(b,x,U){if(!(x.level===U||x.requested===U)){x.requested=U;try{let B=await Z(b,U);if(T||x.requested!==U)return;let C=new Map;B.scene.traverse(k=>{k.isMesh&&C.set(k.name,k)}),x.node.traverse(k=>{let $=C.get(k.name);k.isMesh&&$&&(k.geometry=$.geometry,k.material=$.material)}),x.level=U,x.requested=null,e.onReady?.()}catch(B){x.requested=null,T||e.onError?.(B)}}}function We(){let b=c.get("memory");if(!b?.hologram||!b.text&&!S.length)return;if(!b.text){let k=document.createElement("canvas");k.width=1024,k.height=512;let $=new Cs(k);$.colorSpace=Ht,M.add($);let j=new ln(4.8,2.4),Q=new bt({map:$,transparent:!0,depthWrite:!1,side:Nt,toneMapped:!1});i.add(j),r.add(Q);let me=new je(j,Q),Ie=_n.find(be=>be.id==="memory");me.position.set(Ie.x-1,3.6+ht.room,Ie.z+.3),me.rotation.y=Math.PI,t.add(me),b.text={canvas:k,texture:$,mesh:me}}let{canvas:x,texture:U,mesh:B}=b.text,C=x.getContext("2d");C.clearRect(0,0,1024,512),B.visible=S.length>0,C.fillStyle="rgba(5,24,32,.87)",C.fillRect(0,0,1024,512),C.fillStyle="#b9f4ef",C.font="28px sans-serif",S.slice(0,4).forEach((k,$)=>{let j=Array.from(k).slice(0,96);C.fillText(j.slice(0,48).join(""),30,55+$*118),C.fillText(j.slice(48).join(""),30,94+$*118)}),U.needsUpdate=!0}function Ye(){if(R){if(R.kind==="tram"&&l[R.index].dwell<=1){R.exitRequested=!0;return}Je();return}if(F){if(F.kind==="resident"){Re.inspect(F.id);return}if(F.kind==="demonstrate"){Re.demonstrate(F.id,e.reduced()),e.onSociety?.({id:F.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(F.kind==="door"){let b=h.get(F.id);if(b){b.open=!b.open,he("door",b.node.position);let x=_n.find(U=>U.id===F.id);fe(x)}}if(F.kind==="discover"){P.add(F.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...P]))}catch{}e.onDiscover?.(F.id),he("discover",G.position)}if(F.kind==="tram"&&!e.reduced()&&(R={kind:"waiting",station:F.id},e.onRide?.("waiting")),F.kind==="terminal"&&e.onTerminal?.(F.id),F.kind==="drone"&&!e.reduced()&&(R={kind:"drone",elapsed:0,origin:G.position.clone()},e.onRide?.("drone"),he("tram",G.position)),F.kind==="lift"){let b=c.get(F.id);if(b){let x=G.position.y>5?4:0;if(Math.abs(b.liftValue-x)>.02){b.liftTarget=x;return}b.liftTarget=x?0:4,R={kind:"lift",id:F.id},e.onRide?.("lift"),he("lift",G.position)}}}}function Je(){if(R){if(R.kind==="lift"){let b=c.get(R.id),x=_n.find(U=>U.id===R.id);b.liftTarget=b.liftValue<2?0:4,G.position.set(x.x+1.5,2.4+ht.room+b.liftTarget,x.liftZ)}if(R.kind==="tram"){let b=qn.find(x=>x.id===R.station)||qn[0];G.position.set(b.platformX,2.4+ht.pavement+.3,b.platformZ)}R.kind==="drone"&&G.position.copy(R.origin),R=null,e.onRide?.(null)}}function q(){if(R)return[{kind:"exit",id:R.kind,distance:0}];G.getWorldDirection(A);let b=Re.nearby().filter(U=>(U.x-G.position.x)*A.x+(U.z-G.position.z)*A.z>U.distance*.3),x=(U,B,C,k,$)=>{let j=Math.hypot(G.position.x-C,G.position.z-k);j<$&&b.push({kind:U,id:B,distance:j})};for(let U of _n)G.position.y<5&&x("door",U.id,U.x,U.doorZ,3),c.get(U.id)?.ready&&(x("lift",U.id,U.liftX,U.liftZ,2.3),G.position.y<5&&x("terminal",U.id,U.x-2,U.z+3,2.5));if(G.position.y<5)for(let U of qn)x("tram",U.id,U.platformX,U.platformZ,4),P.has(U.id)||x("discover",U.id,U.platformX+(U.angle===0?5:0),U.platformZ+(U.angle===0?0:5),3);return x("drone","drone",bi.x,bi.z,5),b.sort((U,B)=>U.distance-B.distance)}function Be(b,x,U){if(T||!y)return;let B=x?Math.min(b,.05):0;if(E+=B,N=ks(G.position.x,G.position.z)?.id||null,pe+=b,pe>.5){pe=0;for(let[C,k]of Ae){let $=G.position.distanceTo(k.node.position),j=L==="low"||$>55?2:L==="medium"||$>32?1:0;Oe(C,k,j)}}e.traffic||ne.begin();for(let C of _n)Math.hypot(G.position.x-C.x,G.position.z-C.z)<32&&fe(C);o.forEach(C=>{C.getRoot().visible&&C.update(B)});for(let C of h.values()){let k=Ct.damp(C.value,C.open?1:0,5,Math.min(b,.1));Math.abs(k-(C.open?1:0))<.001&&(k=C.open?1:0);let $=j=>[-1,1].map(Q=>({owner:C.owner,x:C.node.position.x,z:C.node.position.z,min:[Q*.8+Q*1.6*j-.8,ht.room,-.13],max:[Q*.8+Q*1.6*j+.8,ht.room+3.6,.13]}));(C.open||$(k).every(j=>ne.solidClear(j)))&&(C.value=k),$(C.value).forEach((j,Q)=>ne.solid(C.owner+":panel:"+Q,j)),C.action.time=C.value*C.action.getClip().duration,C.mixer.update(0)}for(let[C,k]of c)if(k.liftAction&&(k.liftValue=Ct.damp(k.liftValue,k.liftTarget,1.8,Math.min(b,.1)),Math.abs(k.liftValue-k.liftTarget)<.02&&(k.liftValue=k.liftTarget),k.liftAction.time=k.liftValue/4*k.liftAction.getClip().duration,k.lift.mixer.update(0),R?.kind==="lift"&&R.id===C)){let $=_n.find(j=>j.id===C);G.position.set($.liftX,2.4+ht.room+k.liftValue,$.liftZ),k.liftValue===k.liftTarget&&(R=null,e.onRide?.(null))}if(Re.update(B,x),le.update(B,x),l.forEach((C,k)=>{if(C.node.visible=!!C.body&&k<(L==="low"?1:2),C.body&&(C.node.visible&&!C.body.enabled&&!ne.clear({...C.body,enabled:!0},C.body,C.body)&&(C.node.visible=!1),C.body.enabled=C.node.visible),!C.node.visible)return;let $={u:C.u,dwell:C.dwell,stop:C.stop},j=!1;if(C.dwell>0)C.dwell-=B,C.speed=0;else{let Q=we=>(we.u-C.u+1)%1*Te,me=ue.reduce((we,ze)=>Q(ze)>1e-5&&Q(ze)<Q(we)?ze:we,ue.find(we=>Q(we)>1e-5)||ue[0]),Ie=Math.min(11,Math.sqrt(Q(me)*8));for(let we=1;we<=3;we++){let ze=(C.u+(1.2+C.speed*.7)*we/3/Te)%1;ce.getPointAt(ze,A),ce.getTangentAt(ze,V);let K={x:A.x,y:ht.road+.03,z:A.z,heading:Math.atan2(V.x,V.z)};if(!ne.clear(C.body,K,K)){Ie=0;break}}C.speed+=Ct.clamp(Ie-C.speed,-B*8,B*3.5);let be=B*C.speed/Te,ye=C.u;C.u=(C.u+be)%1;let Fe=ue.findIndex(we=>(we.u-ye+1)%1>0&&(we.u-ye+1)%1<=be+1e-5);Fe>=0&&(C.stop=Fe,C.u=ue[Fe].u,C.dwell=6,C.speed=0,j=!0)}if(ce.getPointAt(C.u,C.node.position),C.node.position.y=ht.road+.03,ce.getTangentAt(C.u,V),C.node.rotation.y=Math.atan2(V.x,V.z),C.body&&(C.body.enabled=C.node.visible,ne.propose(C.body,{...C.node.position,heading:C.node.rotation.y},(Q,me)=>{Q||(Object.assign(C,$),C.speed=0),C.node.position.set(me.x,me.y,me.z),C.node.rotation.y=me.heading,Q&&j&&he("tram",C.node.position)})),C.door.time=(C.dwell>1?1:0)*C.door.getClip().duration,C.mixer.update(0),R?.kind==="waiting"&&ue[C.stop].id===R.station&&C.dwell>1&&(R={kind:"tram",index:k,station:R.station,offset:0},e.onRide?.("tram")),R?.kind==="tram"&&R.index===k){if(C.dwell>1&&(R.station=ue[C.stop].id,R.exitRequested)){Je();return}G.position.copy(C.node.position).addScaledVector(V,R.offset),G.position.y+=.9+1.4,G.lookAt(C.node.position.x+V.x*16,G.position.y,C.node.position.z+V.z*16)}}),R?.kind==="drone"){R.elapsed+=Math.min(b,.1);let C=R.elapsed/40*Math.PI*2;G.position.set(Math.cos(C)*100,45+Math.sin(C*2)*12,Math.sin(C)*95-10),G.lookAt(0,22,-12),R.elapsed>=40&&Je()}if(F=U==="street"&&q()[0]||null,U==="street"&&!R){let C=G.position.distanceTo(Y);C<3&&(X+=C),X>1.8&&(he(N?"step_inside":"step",G.position),X=0)}Y.copy(G.position),e.onEnvironment?.(!!N);for(let[C,k]of m.entries()){if(k.elapsed+=B,k.node.visible=x&&k.elapsed<8,!k.node.visible){k.body&&ne.remove(k.body.id),k.body=null;continue}let $=k.direction>0?k.elapsed/8:1-k.elapsed/8,j={x:40+$*3,y:ht.room,z:70,heading:0};if(k.body||(k.body=ne.register("freight-"+C,ki(k.bounds),j,0)),!k.body){k.elapsed=9,k.node.visible=!1;continue}ne.propose(k.body,j,(Q,me)=>{Q||(k.elapsed-=B),k.node.position.set(me.x,me.y,me.z)})}e.traffic||ne.solve(B),e.onInteraction?.(F,P.size,N)}async function $e(b){if(L=b,!y)return;let x=b==="low"?2:b==="medium"?1:0;Re.setTier(b),le.setTier(b);try{let U=new Map(await Promise.all([...f].filter(B=>!Ae.has(B)).map(async B=>{let C=await Z(B,x),k=new Map;return C.scene.traverse($=>{$.isMesh&&k.set($.name,$)}),[B,k]})));if(T||L!==b)return;for(let B of g){let C=U.get(B.userData.worldAsset)?.get(B.userData.worldPart);C&&(B.geometry=C.geometry,B.material=C.material)}e.onReady?.()}catch(U){T||e.onError?.(U)}}return{update:Be,interact:Ye,endRide:Je,setTier:$e,society:Re,suspend(){Re.suspend(),le.clear(),w=!1;for(let b of m)b.elapsed=9,b.node.visible=!1,b.body&&ne.remove(b.body.id),b.body=null},syncRide(){if(R?.kind==="tram"){let b=l[R.index];ce.getTangentAt(b.u,V),G.position.copy(b.node.position).addScaledVector(V,R.offset),G.position.y+=2.3,G.lookAt(b.node.position.x+V.x*16,G.position.y,b.node.position.z+V.z*16)}},socialAction(b,x){let U=qn.find(B=>B.id===x);return Re.action(e.reduced()&&b==="guide"?"cancel":b,U?{x:U.platformX,z:U.platformZ}:null)},setMemory(b){S=(Array.isArray(b)?b:[]).filter(x=>typeof x=="string").slice(0,8),We()},setWorld(b,x=!1){x?(Re.setReplay(!0),le.clear()):Re.setReplay(!1);let U=b?.entities?.filter(C=>C.kind==="mission")||[],B=new Set;for(let C of U){B.add(C.id);let k=p.get(C.id);if(!x&&e.active()&&w&&k!=null&&k!==C.state&&["running","completed","failed","cancelled"].includes(C.state)&&Date.now()-C.at<3e4){let $=m.find(j=>j.elapsed>=8);$&&($.elapsed=0,$.direction=C.state==="running"?1:-1,_++)}p.set(C.id,C.state)}for(let C of p.keys())B.has(C)||p.delete(C);if(w=!x,x)for(let C of m)C.elapsed=9,C.node.visible=!1},walkRide(b,x){return R?(R.kind==="tram"&&(R.offset=Ct.clamp(R.offset+b*x*3,-2.5,2.5)),!0):!1},move(b,x,U){if(R)return!1;let B=ks(U.x,U.z);return B&&U.y>5?Math.abs(x-B.z-4.8)<.85&&(Math.abs(b-B.x)<1.1||Math.abs(b-B.x+4)<1.75)?!1:wu(B,b,x,c.get(B.id)?.liftValue===4):B&&U.y<5&&([-5.6,1.9].some(C=>Math.abs(b-B.x-C)<.25&&Math.abs(x-B.z-.15)<.25)||fp(B,b,x)&&!(b<B.liftX+1.25&&Math.abs(x-B.liftZ)<1.25&&c.get(B.id)?.liftValue===0)||Math.hypot(b-(B.x-1),x-B.z)<1.55||[-3,0].some(C=>Math.abs(b-B.x-C)<1.1&&Math.abs(x-B.z-3)<.8))?!1:gp(b,x,U,C=>h.get(C)?.value>.85&&!!c.get(C)?.ready,e.districts)},floor(b,x){let U=ks(b,x);return U&&G.position.y>5&&wu(U,b,x,c.get(U.id)?.liftValue===4)?ht.gallery:Math.max(Pe(b,x),mp(b,x,e.districts))},visit(b){Je(),R=null;let x=ds.find(B=>B.id===b);if(x){let B=y?.assets.find(C=>C.id===b)?.navigation.interaction[0]?.position||[0,1.5,4];G.position.set(x.x,2.4+us(x.x,x.z),x.z+B[2]+1),G.lookAt(x.x,2,x.z);return}if(b==="drone"){G.position.set(bi.x,2.7,bi.z+3),G.lookAt(bi.x,2,bi.z);return}let U=qn.find(B=>B.id===b);if(U){let B=P.has(b)?0:5,C=U.platformX+(U.angle===0?B:0),k=U.platformZ+(U.angle===0?0:B);G.position.set(C,2.4+us(C,k),k),G.lookAt(U.platformX,2,U.platformZ===k?U.platformZ+1:U.platformZ)}},destination(b){let x=_n.find(U=>U.id===b);if(x){Je(),R=null;let U=x.doorZ+x.front*4;G.position.set(x.x,2.4+us(x.x,U),U),G.lookAt(x.x,2.4+us(x.x,U),x.z)}},isRiding:()=>!!R,rideBody:()=>R?.kind==="tram"?"tram-"+R.index:null,interaction:()=>F,stats:()=>({society:Re.stats(),machinery:le.stats(),details:[...Ae].map(([b,x])=>({id:b,level:x.level})),loaded:[...f],bytes:I,freightEvents:_,residents:a.filter(b=>b.node.visible).length,trams:l.filter(b=>b.node.visible).length,rooms:[...c].filter(([,b])=>b.ready).map(([b])=>b),inside:N,ride:R?.kind||null,station:R?.station||null,discovered:[...P],loading:O,interactions:F?{kind:F.kind,id:F.id}:null}),dispose(){T||(T=!0,v.abort(),Re.dispose(),le.dispose(),e.traffic||ne.dispose(),S=[],p.clear(),t.removeFromParent(),o.forEach(b=>{b.stopAllAction(),b.uncacheRoot(b.getRoot())}),t.traverse(b=>{b.isInstancedMesh&&b.dispose()}),i.forEach(b=>b.dispose()),r.forEach(b=>b.dispose()),M.forEach(b=>b.dispose()),n.clear())}}}var vy=s=>.5+(s-6)/12*3.62,bp=new z;function Sp(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,ground:r,onThunder:o}){let a="local",l="clear",c="high",h=!1,u=-100,d=!1,f=n.intensity,g=14,M=-1,m=0,p={value:0},v=new pt,S=[],_=317,w=()=>(_=Math.imul(_,1664525)+1013904223>>>0)/4294967296;for(let P=0;P<1200;P++)S.push((w()-.5)*170,w()*60,(w()-.5)*174-7);v.setAttribute("position",new nt(S,3));let T=new mt({transparent:!0,depthWrite:!1,uniforms:{time:p,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),L=new xn(v,T);L.frustumCulled=!1,s.add(L);let y=new xe(10205129),I=new xe(528926),E=new xe(2762296);function R(){let P=vc(a),N=P.amount,O=vy(P.hour);e.intensity=.8+N*2.6,e.color.set(P.evening>.1?16757370:13164287),e.position.set(Math.cos(P.hour/24*Math.PI*2)*120,40+N*115,85),bp.set(Math.cos(O)*120,40-P.evening*18+N*115,Math.sin(O)*120),t.intensity=.7+P.evening*1.6,f=n.intensity=.5+N*.65,s.fog.color.copy(I).lerp(y,N).lerp(E,P.evening*(1-N)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-N*8e-4,i.setLighting?.(N,P.evening,bp),i.setWeather?.(l),r.material.roughness=l==="rain"?.14:.44,L.visible=l==="rain"&&!h,v.setDrawRange(0,c==="low"?200:1200)}function F(P,N){if(l!=="rain"||h||!N){M>=0&&(M=-1,X(0));return}if(g-=P,M<0&&g<=0&&(M=0,m++,g=9+w()*16,o?.(.6+w()*3.2)),M<0)return;M+=P;let O=M<.09?1:M<.16?.25:M<.24?.75:Math.max(0,1-(M-.24)/.35)*.5;X(O),M>.6&&(M=-1,X(0))}function X(P){n.intensity=f+P*2.4,T.uniforms.flash.value=P,i.setFlash?.(P)}return R(),{set(P={}){["local","day","evening","night"].includes(P.time)&&(a=P.time),["clear","rain","fog"].includes(P.weather)&&(l=P.weather),R()},setTier(P){c=P,R()},setIndoor(P){h!==P&&(h=P,R())},update(P,N,O){if(!d)return L.visible=l==="rain"&&!h&&O,O&&(p.value+=Math.min(P,.1)),F(Math.min(P,.1),O),N-u>30?(u=N,R(),!0):!1},mood(){let P=vc(a);return{day:P.amount,evening:P.evening,weather:l,indoor:h}},stats:()=>({time:a,weather:l,indoor:h,daylight:vc(a).amount,strikes:m}),dispose(){d=!0,L.removeFromParent(),v.dispose(),T.dispose()}}}function Tp(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(Au[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([a,l,c,h],u)=>s.solid("district:"+i.id+":"+u,{x:i.x,z:i.z,min:[a,0,l],max:[c,8,h]})),s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let o=e.get(i.asset)?.lods[0].bounds;if(!o)return;let a=o.min.map((c,h)=>c*i.scale[h]),l=o.max.map((c,h)=>c*i.scale[h]);a[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),a[1]=Math.max(a[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:a,max:l})})}function yy(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let o=0;o<s.numberOfChannels;o++){let a=s.getChannelData(o);for(let l=0;l<a.length;l+=e){let c=0,h=Math.min(l+e,a.length);for(let u=l;u<h;u++){let d=a[u];if(!Number.isFinite(d))throw Error("Invalid voice");n=Math.max(n,Math.abs(d)),c+=d*d}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(o=>o>Math.max(.003,n*.003)).sort((o,a)=>o-a),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function Ep(s,e){let t=!1,n=!1,i=0,r=null,o=null,a=[],l=null,c=null,h=null,u=250,d=0,f=0,g=0,M="idle",m=null,p=()=>.04+.46/(1+(u/110)**2);function v(){let y=s.currentTime;l?.gain.setTargetAtTime(p(),y,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),y,.18),h?.pan.setTargetAtTime(d,y,.18)}function S(){if(o){o.onended=null;try{o.stop()}catch{}o=null}a.forEach(y=>y.disconnect()),a=[],l=c=h=null}function _(){f++,clearTimeout(i),i=0,r?.abort(),r=null,S(),M="idle"}function w(y=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,L()},y))}function T(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let y=42;for(let I=0;I<2;I++){let E=m.getChannelData(I),R=Math.round(s.sampleRate*.015),F=0;for(let P=0;P<E.length;P++){y=Math.imul(y,1664525)+1013904223>>>0;let N=(P-R)/(E.length-R);E[P]=P<R?0:(y/4294967296*2-1)*Math.pow(1-N,2.25),F+=E[P]*E[P]}let X=.65/Math.sqrt(F||1);for(let P=0;P<E.length;P++)E[P]*=X}return m}async function L(){if(!t||n||r||o||s.state!=="running"){w();return}let y=f,I=new AbortController;r=I,M="loading";let E=setTimeout(()=>I.abort(),32e3);try{let R=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:I.signal});if(R.status===204){M="idle";return}if(!R.ok||!R.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(R.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let F=await R.arrayBuffer();if(F.byteLength>8*1024*1024)throw Error("Voice too large");if(y!==f||!t||n)return;let X=await s.decodeAudioData(F);if(y!==f||!t||n)return;if(!Number.isFinite(X.duration)||X.duration<=0||X.duration>25||X.numberOfChannels>2)throw Error("Invalid voice");let P=yy(X),N=oe=>(a.push(oe),oe);o=N(s.createBufferSource()),o.buffer=X;let O=N(s.createGain()),A=N(s.createBiquadFilter());O.gain.value=P,A.type="highpass",A.frequency.value=90,c=N(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let V=N(s.createGain()),Y=N(s.createGain()),G=N(s.createGain()),ne=N(s.createGain());V.gain.value=.76,Y.gain.value=.22,G.gain.value=.28;let le=N(s.createDelay(.2)),Ae=N(s.createConvolver());le.delayTime.value=.085,Ae.normalize=!1,Ae.buffer=T();let Pe=N(s.createWaveShaper()),Re=new Float32Array(1024);for(let oe=0;oe<Re.length;oe++)Re[oe]=Math.max(-.75,Math.min(.75,oe*2/(Re.length-1)-1));Pe.curve=Re,l=N(s.createGain()),l.gain.value=p(),h=N(s.createStereoPanner()),h.pan.value=d,o.connect(O).connect(A).connect(c),c.connect(V).connect(ne),c.connect(le).connect(Y).connect(ne),c.connect(Ae).connect(G).connect(ne),ne.connect(Pe).connect(l).connect(h).connect(e),v(),M="speaking",g++,o.onended=()=>{y!==f||!t||n||(M="tail",i=setTimeout(()=>{S(),M="idle",w()},900))},o.start()}catch{y===f&&!n&&(M=I.signal.aborted?"idle":"unavailable")}finally{clearTimeout(E),r===I&&(r=null),y===f&&!n&&!o&&w(M==="unavailable"?6e4:void 0)}}return{setActive(y){t===y||n||(t=y,t?w(3e3+Math.random()*2e3):_())},setListener(y,I,E,R,F){if(!Number.isFinite(y)||!Number.isFinite(I)||!Number.isFinite(E)||!Number.isFinite(R)||!Number.isFinite(F))return;let X=-y,P=-12-E,N=Math.max(0,Math.hypot(X,P)-13);u=Math.hypot(N,Math.max(0,I-87,-I));let O=Math.hypot(X,P),A=Math.hypot(R,F);d=O&&A?Math.max(-.8,Math.min(.8,(P*R-X*F)/O/A))*.8:0,v()},stats(){return{state:M,phrases:g,distance:u,gain:p(),pan:d,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,_(),m=null)}}}function wp(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,o=!1,a="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let p=0;p<h.length;p++)c=Math.imul(c,1664525)+1013904223|0,h[p]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),d=s.createBiquadFilter(),f=s.createGain();u.buffer=l,u.loop=!0,d.type="lowpass",d.frequency.value=450,f.gain.value=0,u.connect(d).connect(f).connect(t),u.start();let g=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((p,v)=>{let S=v===3?s.createBufferSource():s.createOscillator(),_=s.createGain(),w=s.createStereoPanner(),T=s.createBiquadFilter();return v===3?(S.buffer=l,S.loop=!0):(S.type="sine",S.frequency.value=p.f),T.type="lowpass",T.frequency.value=p.f*2,_.gain.value=0,S.connect(T).connect(_).connect(w).connect(t),S.start(),{...p,source:S,level:_,pan:w,low:T}});function M(){f.gain.setTargetAtTime(r?(a==="rain"?.07:.015)*(o?.12:1):0,s.currentTime,.1),d.frequency.setTargetAtTime(o?280:a==="rain"?2100:600,s.currentTime,.2)}function m(){for(let p of[...n]){try{p.source.stop()}catch{}p.release()}}return{setActive(p){if(r=p,M(),!p){m();for(let v of g)v.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(p,v){o=p,v&&(a=v),M()},listener(p,v,S,_,w){Object.assign(i,{x:p,z:S,fx:_,fz:w});for(let T of g){let L=Math.hypot(p-T.x,S-T.z);T.level.gain.setTargetAtTime(r?T.g/(1+L*L*.025)*(o?.25:1):0,s.currentTime,.1),T.pan.pan.value=Math.max(-1,Math.min(1,((T.x-p)*-w+(T.z-S)*_)/Math.max(1,L)))}},play(p,v=0,S=0,_=0){if(!r||n.size>=16)return;let w=Math.hypot(v-i.x,_-i.z);if(w>90)return;let T={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08]}[p];if(!T)return;let[L,y,I]=T,E=p.startsWith("step")||p==="door"?s.createBufferSource():s.createOscillator();"buffer"in E?E.buffer=l:(E.type="sine",E.frequency.value=y);let R=s.createBiquadFilter(),F=s.createGain(),X=s.createStereoPanner();R.type="lowpass",R.frequency.value=o?y*.7:y,F.gain.setValueAtTime(0,s.currentTime),F.gain.linearRampToValueAtTime(I/(1+w*.08),s.currentTime+.015),F.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+L),X.pan.value=Math.max(-1,Math.min(1,((v-i.x)*-i.fz+(_-i.z)*i.fx)/Math.max(w,1))),E.connect(R).connect(F).connect(X).connect(e);let P={source:E,release(){if(n.delete(P))for(let N of[E,R,F,X])N.disconnect()}};n.add(P),E.onended=P.release,E.start(),E.stop(s.currentTime+L+.02)},stats:()=>({voices:n.size,weather:a,inside:o}),dispose(){r=!1,m(),u.stop();for(let p of g){p.source.stop();for(let v of[p.source,p.level,p.pan,p.low])v.disconnect()}for(let p of[u,d,f])p.disconnect()}}}var Cu=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],Ap=[440,523.25,587.33,659.25,783.99,880,1046.5],Pu=12;function My(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let o=0;o<2;o++){let a=i.getChannelData(o);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,a[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function by(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,o=!1,a=0,l=null,c=[],h=[],u=new Float32Array(256),d=new Set,f=.18,g=null,M=null,m=null,p=null,v=!1,S="clear",_=[],w=null,T=null,L=null,y=null,I=null,E=null,R=0,F=0,X=0,P=0,N={busy:!1,day:0,evening:0,weather:"clear"},O={ambience:1,effects:1,voice:1};try{o=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function A(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let H=b=>(c.push(b),b),Z=b=>(h.push(b),H(b));t=H(e.createGain()),t.gain.value=0,n=H(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),M=H(e.createGain()),M.gain.value=O.ambience,M.connect(t),m=H(e.createGain()),m.gain.value=O.effects,m.connect(t),p=H(e.createGain()),p.gain.value=O.voice,p.connect(t),I=H(e.createConvolver()),I.buffer=My(e,3.4,2.6);let W=H(e.createGain());W.gain.value=.55,I.connect(W).connect(M),l=Ep(e,p),g=wp(e,m,M),g.environment(v,S);for(let[b,x]of[[55,.07],[82.4069,.022],[110,.01]]){let U=Z(e.createOscillator()),B=H(e.createGain());U.frequency.value=b,B.gain.value=x,U.connect(B).connect(M),U.start()}w=H(e.createBiquadFilter()),w.type="lowpass",w.frequency.value=650,w.Q.value=.6,T=H(e.createGain()),T.gain.value=.042;let J=H(e.createGain());J.gain.value=.9,w.connect(T).connect(M),T.connect(J).connect(I),_=Cu[0].map(b=>[-6,6].map(x=>{let U=Z(e.createOscillator()),B=H(e.createGain());return U.type="sawtooth",U.frequency.value=b,U.detune.value=x,B.gain.value=.22,U.connect(B).connect(w),U.start(),U}));let te=Z(e.createOscillator()),fe=H(e.createGain());te.frequency.value=.05,fe.gain.value=180,te.connect(fe).connect(w.frequency),te.start();let Me=e.createBuffer(1,e.sampleRate*4,e.sampleRate),Oe=Me.getChannelData(0),We=0;for(let b=0;b<Oe.length;b++)We=(We+(Math.random()*2-1)*.018)/1.018,Oe[b]=We;let Ye=Oe[Oe.length-1]-Oe[0];for(let b=0;b<Oe.length;b++)Oe[b]-=Ye*b/(Oe.length-1);let Je=Z(e.createBufferSource());L=H(e.createBiquadFilter()),y=H(e.createGain()),Je.buffer=Me,Je.loop=!0,L.type="lowpass",L.frequency.value=680,L.Q.value=.25,y.gain.value=.32,Je.connect(L).connect(y).connect(M),Je.start();let q=Z(e.createOscillator()),Be=H(e.createGain());q.frequency.value=.075,Be.gain.value=160,q.connect(Be).connect(L.frequency),q.start(),E=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let $e=E.getChannelData(0);for(let b=0;b<$e.length;b++)$e[b]=Math.random()*2-1;ue()}let V=()=>!!e&&o&&r&&!i&&f>0;function Y(H){let Z={release(){if(d.delete(Z))for(let W of H){try{W.stop?.()}catch{}try{W.disconnect()}catch{}}}};return d.add(Z),H[0].onended=Z.release,P++,Z}function G(H,Z,W){let J=e.createStereoPanner(),te=e.createGain();return J.pan.value=Z,te.gain.value=W,H.connect(J).connect(M),J.connect(te).connect(I),[J,te]}function ne(){let H=e.currentTime,Z=Ap[Math.floor(Math.random()*Ap.length)],W=e.createOscillator(),J=e.createOscillator(),te=e.createGain(),fe=e.createGain();W.frequency.value=Z,J.frequency.value=Z*2.76,te.gain.value=.22,fe.gain.setValueAtTime(0,H),fe.gain.linearRampToValueAtTime(N.busy?.02:.014,H+.008),fe.gain.exponentialRampToValueAtTime(1e-4,H+2.8),W.connect(fe),J.connect(te).connect(fe),Y([W,J,te,fe,...G(fe,Math.random()*1.4-.7,1.4)]),W.start(H),J.start(H),W.stop(H+2.9),J.stop(H+2.9)}function le(){let H=e.currentTime,Z=e.createBufferSource(),W=e.createBiquadFilter(),J=e.createGain(),te=Math.random()<.5?-1:1;Z.buffer=E,Z.loop=!0,W.type="bandpass",W.Q.value=1.4,W.frequency.setValueAtTime(260,H),W.frequency.exponentialRampToValueAtTime(1100,H+1.6),W.frequency.exponentialRampToValueAtTime(380,H+3.4),J.gain.setValueAtTime(0,H),J.gain.linearRampToValueAtTime(.03,H+1.6),J.gain.exponentialRampToValueAtTime(1e-4,H+3.6),Z.connect(W).connect(J);let[fe,Me]=G(J,-.9*te,.5);fe.pan.setValueAtTime(-.9*te,H),fe.pan.linearRampToValueAtTime(.9*te,H+3.4),Y([Z,W,J,fe,Me]),Z.start(H),Z.stop(H+3.7)}function Ae(){let H=e.currentTime,Z=S==="rain"?.36:.32;L.frequency.cancelScheduledValues(H),y.gain.cancelScheduledValues(H),L.frequency.setTargetAtTime(1200+Math.random()*700,H,.8),L.frequency.setTargetAtTime(680,H+2.6,1.4),y.gain.setTargetAtTime(Z*1.7,H,.7),y.gain.setTargetAtTime(Z,H+2.6,1.5),P++}function Pe(){let H=e.currentTime,Z=e.createBiquadFilter(),W=e.createGain();Z.type="lowpass",Z.frequency.value=420,W.gain.setValueAtTime(0,H),W.gain.linearRampToValueAtTime(.028,H+.9),W.gain.setValueAtTime(.028,H+3.2),W.gain.exponentialRampToValueAtTime(1e-4,H+5.5);let J=[69.3,103.83].map(te=>{let fe=e.createOscillator();return fe.type="sawtooth",fe.frequency.value=te,fe.detune.value=Math.random()*8-4,fe.connect(Z),fe});Z.connect(W),Y([...J,Z,W,...G(W,-.55,1.6)]),J.forEach(te=>{te.start(H),te.stop(H+5.6)})}function Re(){let H=e.currentTime,Z=2+Math.floor(Math.random()*3),W=Math.random()*1.2-.6;for(let J=0;J<Z&&d.size<Pu;J++){let te=H+J*(.12+Math.random()*.1),fe=e.createOscillator(),Me=e.createGain(),Oe=2600+Math.random()*900;fe.frequency.setValueAtTime(Oe,te),fe.frequency.exponentialRampToValueAtTime(Oe*1.5,te+.07),Me.gain.setValueAtTime(0,te),Me.gain.linearRampToValueAtTime(.006,te+.015),Me.gain.exponentialRampToValueAtTime(1e-4,te+.13),fe.connect(Me),Y([fe,Me,...G(Me,W,.6)]),fe.start(te),fe.stop(te+.15)}}function oe(){if(d.size>=Pu)return;let H=Math.random(),Z=1-N.day;H<.34?ne():H<.62?le():H<.8?Ae():Z>.5&&N.weather!=="rain"&&H<.88?Pe():N.day>.4&&N.weather==="clear"?Re():ne()}function pe(){clearTimeout(R),R=0,V()&&(R=setTimeout(()=>{R=0,V()&&(oe(),pe())},(N.busy?2200:4200)+Math.random()*6e3))}function ce(){clearTimeout(F),F=0,V()&&(F=setTimeout(()=>{F=0,V()&&(X=(X+1)%Cu.length,_.forEach((H,Z)=>H.forEach(W=>W.frequency.setTargetAtTime(Cu[X][Z],e.currentTime,1.8))),ce())},16e3))}function Te(){clearTimeout(R),clearTimeout(F),R=F=0;for(let H of[...d])H.release()}function ue(){if(!e)return;let H=e.currentTime;w.frequency.setTargetAtTime(N.busy?1500:650+N.day*450+N.evening*150,H,2.5),T.gain.setTargetAtTime((N.busy?.055:.042)*(N.weather==="rain"?.75:1),H,2)}function he(H=!1){if(clearTimeout(a),!i){if(H&&o)try{A()}catch{o=!1,s(!1);return}e&&(l?.setActive(o&&r&&f>0&&O.voice>0),g?.setActive(o&&r&&f>0),o&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(f,e.currentTime,.25),R||pe(),F||ce()):(Te(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),a=setTimeout(()=>{!i&&(!o||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>o,toggle(){o=!o;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(o))}catch{}he(!0),s(o)},unlock(){he(!0)},setActive(H){r!==H&&(r=H,he())},setVolume(H){Number.isFinite(H)&&(f=Math.max(0,Math.min(.35,H)),he())},setListener(H,Z,W,J,te){l?.setListener(H,Z,W,J,te),g?.listener(H,Z,W,J,te)},setChannel(H,Z){if(!(H in O)||!Number.isFinite(Z))return;O[H]=Math.max(0,Math.min(1,Z));let W={ambience:M,effects:m,voice:p}[H];W&&W.gain.setTargetAtTime(O[H],e.currentTime,.05),he()},effect(H,Z,W,J){g?.play(H,Z,W,J)},setEnvironment(H,Z){v=!!H,["clear","rain","fog"].includes(Z)&&(S=Z),g?.environment(v,S)},setMood(H={}){Object.assign(N,{busy:!!H.busy,day:Number.isFinite(H.day)?H.day:N.day,evening:Number.isFinite(H.evening)?H.evening:N.evening,weather:["clear","rain","fog"].includes(H.weather)?H.weather:N.weather}),ue()},thunder(H=1){if(!V()||v||d.size>=Pu)return;let Z=Math.max(0,Math.min(5,H)),W=e.currentTime+Z,J=Math.min(1,Z/4),te=e.createBufferSource(),fe=e.createBiquadFilter(),Me=e.createGain();te.buffer=E,te.loop=!0,fe.type="lowpass",fe.Q.value=.7,fe.frequency.setValueAtTime(900-J*500,W),fe.frequency.exponentialRampToValueAtTime(70,W+3.5),Me.gain.setValueAtTime(0,W),Me.gain.linearRampToValueAtTime(.09*(1-J*.5),W+.06+J*.25),Me.gain.exponentialRampToValueAtTime(.025,W+1.3),Me.gain.exponentialRampToValueAtTime(1e-4,W+5.5),te.connect(fe).connect(Me),Y([te,fe,Me,...G(Me,Math.random()*.8-.4,.8)]),te.start(W),te.stop(W+5.6)},stats(){let H=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let Z of u)H+=Z*Z}return{voice:l?.stats(),effects:g?.stats(),channels:{...O},enabled:o,active:r,state:e?.state||"uninitialized",volume:f,rms:Math.sqrt(H/u.length),soundscape:{chord:X,events:P,transient:d.size,scheduled:!!R,mood:{...N}}}},dispose(){i||(i=!0,Te(),g?.dispose(),l?.dispose(),clearTimeout(a),h.forEach(H=>{try{H.stop()}catch{}}),c.forEach(H=>H.disconnect()),e&&e.close().catch(()=>{}))}}}var wn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],yc={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},Rp=new z(122,106,183),Iu=class extends Fn{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new Wt(1,1,{type:nn,samples:n}),this.quad=new Mi(new mt({uniforms:si.clone(zi.uniforms),vertexShader:zi.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},Cp=new z(0,22,-12),Hs=[];{let s=(n,i,r,o=0,a=0,l=[1,1,1])=>Hs.push({asset:n,x:i,z:r,y:o,angle:a,scale:l}),{xs:e,zs:t}=En;for(let n of t){for(let r of e)s("street-crossing",r,n);let i=[En.minX,...e,En.maxX];for(let r=0;r<i.length-1;r++){let o=i[r]+(r?6:0),a=i[r+1]-(r<i.length-2?6:0);s("street-tile",(o+a)/2,n,0,0,[(a-o)/16,1,1])}}for(let n of e){let i=[En.minZ,...t,En.maxZ];for(let r=0;r<i.length-1;r++){let o=i[r]+(r?6:0),a=i[r+1]-(r<i.length-2?6:0);s("street-tile",n,(o+a)/2,0,Math.PI/2,[(a-o)/16,1,1])}}for(let n of t)for(let i of[-55,-31,31,55])s("street-lamp",i,n-4.5,.45),s("planter",i+4,n-4.5,.45);for(let{x:n,z:i,scale:r}of _c)s("data-tower-a",n,i,0,0,[1,r,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,.5),s("server-rack",-49,-36.7,.5);for(let n=0;n<48;n++){let i=(n%12-5.5)*22,r=-143-Math.floor(n/12)*29,o=.7+n*17%13/12;s(n%2?"data-tower-a":"data-tower-b",i,r,-3,0,[.8,o,.8])}}var Sy=[...Zf(Hs),..._n.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function O1(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",o=r==="auto"?"high":r,a=0,l=null,c=0,h=0,u=0,d=0,f=0,g=0,M=0,m=null,p=e.reducedMotion,v=1,S=1,_=[],w=!1,T=null,L=null,y=new AbortController,I=new Set,E=new Map,R=new Map,F=new Set,X=new Set,P=new Set,N=[],O=new nc({antialias:!0,alpha:!1});O.toneMapping=Ns,O.toneMappingExposure=.98,O.shadowMap.type=al,O.shadowMap.autoUpdate=!1,O.info.autoReset=!1;let A=O.domElement;A.className="sysworld-gl",A.tabIndex=0,A.setAttribute("aria-label",e.label),s.append(A);let V=new As;V.background=new xe(528926),V.fog=new no(528926,.004);let Y=new $t(43,1,.3,1800);Y.position.copy(Rp);let G=gc(),ne=G.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},Y.position,100),le=new lc(Y,A);le.target.copy(Cp),le.enableDamping=!0,le.dampingFactor=.085,le.minDistance=12,le.maxDistance=410,le.maxPolarAngle=Math.PI*.485,le.update();let Ae=new Lr(O),Pe=new cc,Re=Ae.fromScene(Pe,.05);V.environment=Re.texture,V.environmentIntensity=.48,Pe.dispose(),Ae.dispose();let oe=new vo(12639487,1056813,.8);V.add(oe);let pe=new ns(13164287,3.2);pe.position.set(-70,145,85),pe.castShadow=!0,Object.assign(pe.shadow.camera,{left:-120,right:120,top:120,bottom:-120,near:1,far:360}),pe.shadow.normalBias=.09,pe.shadow.bias=-15e-5,V.add(pe);let ce=new ns(16758130,2.4);ce.position.set(90,80,-120),V.add(ce);let Te=new ei(6479871,0,75,2);Te.position.set(0,34,-12),V.add(Te);let ue=new Wt(1,1,{type:nn}),he=new uc(O,ue);he.addPass(new Iu(V,Y,Math.min(4,O.capabilities.maxSamples)));let H=new Or(new ve(1,1),.28,.55,1.25);he.addPass(H),he.addPass(new dc);let Z=new lt,W=new lt,J=new lt;V.add(Z),Z.add(W,J);function te(ie,Ne){return F.add(ie),X.add(Ne),new je(ie,Ne)}let fe=te(new fi(170,3,174),new yn({color:1451051,metalness:.55,roughness:.38}));fe.position.set(0,-1.5,-7),fe.receiveShadow=!0,Z.add(fe);let Me=lp(V,{sunDirection:pe.position,lamps:Hs.filter(ie=>ie.asset==="street-lamp")});he.addPass(Me.post);let Oe=wn.find(ie=>ie.id==="memory"),We=tp(V,Oe,{roof:21,label:e.memoryLabel}),Ye=hp(V,{traffic:G}),Je=Sp(V,{sun:pe,rim:ce,hemisphere:oe,atmosphere:Me,ground:fe,onThunder:ie=>e.onThunder?.(ie)}),q="",Be=null,$e=te(new Is(1,1.035,64),new bt({color:9365742,transparent:!0,opacity:.85,depthWrite:!1,side:Nt}));$e.rotation.x=-Math.PI/2,$e.visible=!1,Z.add($e);let b=new bt({color:6742230,toneMapped:!1}),x=new vn(new pi(.65,10,6),b,wn.length);F.add(x.geometry),X.add(b),Z.add(x);let U=new St,B=new z,C=new wo,k=new ve;wn.forEach((ie,Ne)=>{U.position.set(ie.x,ie.height+2,ie.z),U.updateMatrix(),x.setMatrixAt(Ne,U.matrix),x.setColorAt(Ne,new xe(8425630))}),e.signal?.addEventListener("abort",fs,{once:!0});let $;try{if(e.signal?.aborted)throw Error("Disposed");let ie=await fetch(e.assetURL("manifest.json"),{signal:y.signal});if(!ie.ok)throw Error("City manifest unavailable");$=await ie.json()}catch(ie){throw fs(),ie}let j=new Map($.assets.map(ie=>[ie.id,ie]));Tp(G,j,wn,Hs),Be=Mp(V,{camera:Y,districts:wn,tier:o,traffic:G,reduced:()=>p,active:()=>n&&!w&&i!=="map"&&!p&&!e.replaying?.(),assetURL:ie=>e.resourceURL("/3d/system-world/v2/"+ie),onInteraction:e.onInteraction,onDiscover:e.onDiscover,onSound:e.onSound,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:ie=>{Je.setIndoor(ie),e.onEnvironment?.(ie)},onError:e.onError,onReady:()=>{O.shadowMap.needsUpdate=!0}});async function Q(ie,Ne){let it=ie+":"+Ne;return E.has(it)||E.set(it,(async()=>{let at=j.get(ie)?.lods.find(wt=>wt.level===Ne);if(!at||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(at.file))throw Error("Invalid city asset");let ut=new AbortController;I.add(ut);let Ut=setTimeout(()=>ut.abort(),15e3);try{let wt=await fetch(e.assetURL(at.file),{signal:ut.signal});if(!wt.ok)throw Error("City asset unavailable");let Bt=await wt.arrayBuffer();if(t)throw Error("Disposed");let vt=await new hs().parseAsync(Bt,"");if(t)throw vt.scene.traverse(Ue=>{Ue.isMesh&&(Ue.geometry.dispose(),Ue.material.dispose())}),Error("Disposed");return f+=Bt.byteLength,vt.scene.traverse(Ue=>{if(!Ue.isMesh)return;F.add(Ue.geometry),Ue.castShadow=!0,Ue.receiveShadow=!0;let It=(Array.isArray(Ue.material)?Ue.material:[Ue.material]).map(dt=>{if(R.has(dt.name)){let kt=dt;dt=R.get(dt.name),kt.dispose()}else R.set(dt.name,dt),X.add(dt);return dt});Ue.material=Array.isArray(Ue.material)?It:It[0]}),vt.scene}finally{clearTimeout(Ut),I.delete(ut)}})().catch(at=>{throw E.delete(it),at})),E.get(it)}function me(ie){ie.traverse(Ne=>{Ne.isInstancedMesh&&Ne.dispose()}),ie.clear()}async function Ie(){let ie=++a,Ne=yc[o],it=Ne.lod,at=[...new Set([...Hs.map(ut=>ut.asset),...wn.map(ut=>ut.asset),"service-drone"])];try{let ut=new Map(await Promise.all(at.map(async Ue=>[Ue,await Q(Ue,it)]))),Ut=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async Ue=>[Ue,await Q(Ue,2)])));if(t||ie!==a)return;me(W),me(J),_=[];let wt=new Map;for(let Ue of Hs){let Ft=(Ue.z<-100?Ut:ut).get(Ue.asset);Ft.updateMatrixWorld(!0);let It=new et().compose(new z(Ue.x,Ue.y,Ue.z),new Zt().setFromAxisAngle(new z(0,1,0),Ue.angle),new z(...Ue.scale));Ft.traverse(dt=>{if(!dt.isMesh)return;let kt=dt.uuid;wt.has(kt)||wt.set(kt,{node:dt,matrices:[]}),wt.get(kt).matrices.push(new et().multiplyMatrices(It,dt.matrixWorld))})}for(let{node:Ue,matrices:Ft}of wt.values()){let It=new vn(Ue.geometry,Ue.material,Ft.length);Ft.forEach((dt,kt)=>It.setMatrixAt(kt,dt)),It.castShadow=!0,It.receiveShadow=!0,W.add(It)}for(let Ue of wn){let Ft=ut.get(Ue.asset).clone(!0);Ft.position.set(Ue.x,0,Ue.z),Ft.userData.district=Ue.id,J.add(Ft),_.push(Ft)}L?.attachLandmarks(J),Ye.setTemplate(ut.get("service-drone"));let Bt=new en,vt=new Map;for(let Ue of["data-tower-a","data-tower-b"])vt.set(Ue,Bt.setFromObject(Ut.get(Ue)).max.y);Me.setAviation(Hs.filter(Ue=>vt.has(Ue.asset)&&(Ue.z<-100||Ue.scale[1]>1.1)).map(Ue=>({x:Ue.x,y:Ue.y+vt.get(Ue.asset)*Ue.scale[1]+.6,z:Ue.z}))),O.shadowMap.needsUpdate=!0,e.onReady?.()}catch(ut){!t&&ie===a&&e.onError?.(ut)}}function be(){if(t)return;v=Math.max(1,s.clientWidth),S=Math.max(1,s.clientHeight);let ie=Math.min(devicePixelRatio||1,yc[o].dpr,Math.sqrt(3840*2160/(v*S)));O.setPixelRatio(ie),O.setSize(v,S,!1),he.setPixelRatio(ie),he.setSize(v,S),Y.aspect=v/S,Y.updateProjectionMatrix(),We.setPointScale(S*ie),Me.setPointScale(S*ie)}function ye(ie){return r=ie in yc||ie==="auto"?ie:"auto",o=r==="auto"?"high":r,Fe(),Ie()}function Fe(){let ie=yc[o];O.shadowMap.enabled=ie.shadow>0,ie.shadow&&pe.shadow.mapSize.x!==ie.shadow&&(pe.shadow.mapSize.set(ie.shadow,ie.shadow),pe.shadow.map?.dispose(),pe.shadow.map=null),O.shadowMap.needsUpdate=!0,H.enabled=ie.bloom,Me.setTier(o),Be.setTier(o),Je.setTier(o),Ye.setTier(o),be(),e.onQuality?.(r,o)}function we(ie,Ne){l=null;let it=G.findFree(ne,ie);if(!it)return;if(ie=new z(it.x,it.y,it.z),p){Y.position.copy(ie),Object.assign(ne,it),le.target.copy(Ne),le.update(),l=null;return}let at=(Bt,vt)=>G.clear(ne,{...Bt,heading:0},{...vt,heading:0},!1),ut=Y.position.clone(),Ut=Math.max(ut.y,ie.y,110),wt=[ut,ie];if(!at(ut,ie)){let Bt=[ut],vt=new z(ie.x,Ut,ie.z),Ue=[-1],Ft=[0],It=-1;if(!at(vt,ie))return;for(let dt of[3,8,16])for(let kt=0;kt<8;kt++)Bt.push(new z(ut.x+Math.cos(kt*Math.PI/4)*dt,ut.y,ut.z+Math.sin(kt*Math.PI/4)*dt));for(let dt of _n)if(Math.hypot(ut.x-dt.x,ut.z-dt.z)<24)for(let kt of[dt.z,dt.doorZ-dt.front*2,dt.doorZ+dt.front*3])Bt.push(new z(dt.x,ut.y,kt));for(;Ft.length;){let dt=Ft.shift(),kt=Bt[dt],$o=new z(kt.x,Ut,kt.z);if(at(kt,$o)&&at($o,vt)){It=dt;break}for(let Si=1;Si<Bt.length;Si++)Ue[Si]===void 0&&kt.distanceTo(Bt[Si])<=20&&at(kt,Bt[Si])&&(Ue[Si]=dt,Ft.push(Si))}if(It<0)return;wt=[];for(let dt=It;dt>=0;dt=Ue[dt])wt.unshift(Bt[dt]);wt.push(new z(Bt[It].x,Ut,Bt[It].z),vt,ie)}l={start:Y.position.clone(),targetStart:le.target.clone(),end:ie.clone(),target:Ne.clone(),time:0,waypoints:wt}}function ze(){G.relocate(ne,Y.position),Y.position.set(ne.x,ne.y,ne.z)}function K(ie){let Ne=wn.find(it=>it.id===ie);Ne&&(m=ie,$e.position.set(Ne.x,.55,Ne.z),$e.scale.setScalar(Ne.radius*1.3),$e.visible=!0,i==="street"?(Y.position.set(Ne.x,2.4,Ne.z+Ne.radius+7),Y.lookAt(Ne.x,Ne.height*.4,Ne.z),ze()):we(new z(Ne.x+Ne.radius*2.7,Ne.height*.7+18,Ne.z+Ne.radius*4),new z(Ne.x,Ne.height*.4,Ne.z)))}function _e(ie){Be.endRide(),Be.society.suspend(),P.clear(),l=null,i=ie,le.enabled=i==="orbit"||i==="tour",ne.minY=i==="street"?-2.3:-.4,ne.ignore=ne.follow=null,ne.circles[0].r=ne.reach=i==="street"?.24:.38,document.pointerLockElement===A&&document.exitPointerLock(),i==="street"?(Y.position.set(18,2.4,57),Y.lookAt(0,26,-12),ze()):i!=="map"&&(ze(),we(Rp.clone().multiplyScalar(Y.aspect<1?1.3:1),Cp)),g=0,M=0,e.onMode?.(i)}let de=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};le.addEventListener("start",de);function Se(ie,Ne,it,at){ie.addEventListener(Ne,it,at),N.push(()=>ie.removeEventListener(Ne,it,at))}let Ee=null,ge=!1,Ve=new Gn(0,0,0,"YXZ");function Ge(ie,Ne){Ve.setFromQuaternion(Y.quaternion),Ve.y-=ie*.0025,Ve.x=Ct.clamp(Ve.x-Ne*.0025,-1.35,1.35),Y.quaternion.setFromEuler(Ve)}Se(A,"pointerdown",ie=>{A.focus({preventScroll:!0}),de(),Ee=[ie.clientX,ie.clientY],ge=!1,i==="street"&&A.setPointerCapture(ie.pointerId)}),Se(A,"pointermove",ie=>{i==="street"&&(document.pointerLockElement===A||Ee)&&Ge(ie.movementX,ie.movementY),Ee&&Math.hypot(ie.clientX-Ee[0],ie.clientY-Ee[1])>5&&(ge=!0)}),Se(A,"pointerup",ie=>{if(Ee&&!ge&&i!=="street"){let Ne=A.getBoundingClientRect();k.set((ie.clientX-Ne.left)/Ne.width*2-1,1-(ie.clientY-Ne.top)/Ne.height*2),C.setFromCamera(k,Y);let it=C.intersectObjects(_,!0)[0];if(it){let at=it.object;for(;at&&!at.userData.district;)at=at.parent;at&&e.onSelect?.(at.userData.district)}}Ee=null}),Se(A,"pointercancel",()=>{Ee=null,P.clear()}),Se(A,"keydown",ie=>{if(ie.key==="Escape"){_e("orbit"),ie.preventDefault();return}if(i==="street"&&ie.code==="KeyE"&&!ie.repeat){Be.interact(),ie.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(ie.code)&&(P.add(ie.code),ie.preventDefault(),ie.stopPropagation())}),Se(A,"keyup",ie=>P.delete(ie.code)),Se(A,"blur",()=>P.clear()),Se(window,"blur",()=>{P.clear(),Ee=null}),Se(A,"webglcontextlost",ie=>{ie.preventDefault(),w=!0,P.clear(),e.onContextLost?.()}),T=new ResizeObserver(be),T.observe(s);function zt(ie){let Ne=(P.has("ShiftLeft")?25:11)*ie,it=Number(P.has("KeyW")||P.has("ArrowUp"))-Number(P.has("KeyS")||P.has("ArrowDown")),at=Number(P.has("KeyD")||P.has("ArrowRight"))-Number(P.has("KeyA")||P.has("ArrowLeft"));if(!it&&!at||Be.walkRide(it,ie))return;let ut=Ne/Math.hypot(it,at);Y.getWorldDirection(B),B.y=0,B.normalize();let Ut=(B.x*it-B.z*at)*ut,wt=(B.z*it+B.x*at)*ut,Bt=(vt,Ue)=>Be.move(vt,Ue,Y.position)&&G.clear(ne,ne,{x:vt,y:2.4+Be.floor(vt,Ue),z:Ue,heading:0});Bt(Y.position.x+Ut,Y.position.z)&&(Y.position.x+=Ut),Bt(Y.position.x,Y.position.z+wt)&&(Y.position.z+=wt),Be.isRiding()||(Y.position.y=2.4+Be.floor(Y.position.x,Y.position.z))}let Tt=0,An=jf(ie=>{G.begin(),Tt+=p?0:ie,i==="street"&&zt(ie),Be.update(ie,!p,i);let Ne=ne.follow;ne.follow=Be.rideBody(),ne.ignore=ne.follow||Ne,L?.update(ie,!p),Ye.update(ie,Tt,!p),G.propose(ne,{...Y.position,heading:0},(it,at)=>Y.position.set(at.x,at.y,at.z)),G.solve(ie),Be.syncRide?.(),ne.follow&&Object.assign(ne,{x:Y.position.x,y:Y.position.y,z:Y.position.z}),ne.ignore=ne.follow});function On(ie,Ne){if(t||!n||w||i==="map")return;let it=null,at=0,ut=null;if(i!=="street"){if(i==="tour"&&(g-=ie,g<=0)){let vt=wn[M++%wn.length].id;K(vt),e.onTourFocus?.(vt),g=7}if(l){it=l,at=l.time,l.time+=ie;let vt=Math.min(1,l.time/(l.waypoints.length===2?1.1:3.2)),Ue=vt*vt*(3-2*vt),Ft=Math.min(l.waypoints.length-1-1e-6,Ue*(l.waypoints.length-1)),It=Math.floor(Ft);Y.position.lerpVectors(l.waypoints[It],l.waypoints[It+1],Ft-It),le.target.lerpVectors(l.targetStart,l.target,Ue),Y.lookAt(le.target),ut=Y.position.clone(),vt===1&&(l=null)}else le.update()}An(Math.min(.1,Math.max(0,ie)))||(G.begin(),G.propose(ne,{...Y.position,heading:0},(vt,Ue)=>Y.position.set(Ue.x,Ue.y,Ue.z)),G.solve(0)),it&&(Y.position.distanceToSquared(ut)>1e-6?(it.wait=(it.wait||0)+ie,it.time=at,l=it.wait<6?it:null):it.wait=0),Y.getWorldDirection(B),e.onListener?.(Y.position.x,Y.position.y,Y.position.z,B.x,B.z);let Ut=!!e.busy?.();Je.update(ie,Ne,!p)&&(O.shadowMap.needsUpdate=!0),Te.intensity=p?0:Ut?180+Math.sin(Ne*2)*35:0,Me.setBusy(Ut),Me.setCinematic(i==="tour"),Me.update(ie,Ne,Y,!p);let wt=Je.mood(),Bt=[Ut,wt.day.toFixed(1),wt.evening.toFixed(1),wt.weather].join();if(Bt!==q&&(q=Bt,e.onMood?.({busy:Ut,day:wt.day,evening:wt.evening,weather:wt.weather})),We.update(ie,Y,!p),O.info.reset(),he.render(),c++,r==="auto"&&ie>0&&ie<.2&&Ne-d>12&&(h+=ie,u++,u>=180)){let vt=h/u,Ue=["low","medium","high"],Ft=Ue.indexOf(o),It=vt>.028&&Ft>0?Ue[Ft-1]:vt<.017&&Ft<2?Ue[Ft+1]:o;u=0,h=0,It!==o&&(o=It,d=Ne,Fe(),Ie())}}function Mc(ie,Ne){L?.setData(ie,Ne),wn.forEach((it,at)=>{let ut=ie.find(Ut=>Ut.id===it.id);x.setColorAt(at,new xe(!ut||ut.stale?8096667:ut.state==="error"?16742504:ut.state==="running"?7730385:8175587))}),x.instanceColor.needsUpdate=!0}function fs(){t||(t=!0,a++,y.abort(),I.forEach(ie=>ie.abort()),P.clear(),document.pointerLockElement===A&&document.exitPointerLock(),e.signal?.removeEventListener("abort",fs),N.forEach(ie=>ie()),T?.disconnect(),le.dispose(),L?.dispose(),Be?.dispose(),Je.dispose(),We.dispose(),Ye.dispose(),G.dispose(),Me.dispose(),me(W),me(J),x.dispose(),F.forEach(ie=>ie.dispose()),X.forEach(ie=>ie.dispose()),he.passes.forEach(ie=>ie.dispose?.()),he.dispose(),Re.dispose(),pe.shadow.dispose(),O.dispose(),O.forceContextLoss(),A.remove(),E.clear(),R.clear())}L=$f(V,wn,{traffic:G,society:Be.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!w&&i!=="map"&&!e.replaying?.(),obstacles:Sy}),We.setReducedMotion(!!p);try{Fe(),await Ie()}catch(ie){throw fs(),ie}return{districts:wn,canvas:A,update:On,focus(ie){de(),K(ie)},setMode:_e,setQuality:ye,setData:Mc,dispose:fs,interact(){Be.interact()},socialAction:Be.socialAction,visit(ie){_e("street"),Be.visit(ie),ze()},enter(ie){_e("street"),Be.destination(ie),ze()},setEnvironment(ie){Je.set(ie),O.shadowMap.needsUpdate=!0},setVisible(ie){n=ie,ie||(An(0),Be.suspend(),L?.update(0,!1),P.clear(),Ee=null,document.pointerLockElement===A&&document.exitPointerLock())},setReducedMotion(ie){p=ie,We.setReducedMotion(!!ie),ie&&(l=null,Be.endRide(),Be.society.suspend()),ie&&i==="tour"&&_e("orbit")},setHologram(ie,Ne){We.setTexts(ie,Ne),Be.setMemory(ie)},setWorld(ie,Ne){Be.setWorld(ie,Ne)},moveKey(ie,Ne){Ne?P.add(ie):P.delete(ie)},lockPointer(){if(i==="street")return A.requestPointerLock()},project(ie){let Ne=wn.find(it=>it.id===ie);return Ne?(B.set(Ne.x,Ne.height+4,Ne.z).project(Y),{x:(B.x+1)*v/2,y:(1-B.y)*S/2,visible:B.z<1&&B.z>-1}):null},stats(){return{flying:!!l,traffic:G.stats(),experience:Be.stats(),weather:Je.stats(),life:L?.stats(),hologram:We.stats(),atmosphere:Me.stats(),drones:Ye.stats(),frames:c,tier:o,mode:i,focusedDistrict:m,loadedBytes:f+Be.stats().bytes,cachedModels:E.size,calls:O.info.render.calls,triangles:O.info.render.triangles,geometries:O.info.memory.geometries,position:Y.position.toArray(),renderer:O.getContext().getParameter(O.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||O.getContext().RENDERER),disposed:t}}}}export{O1 as createCity,by as createCityAmbience,wn as districts,Sy as obstacles,Hs as placements};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
