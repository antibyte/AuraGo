var rs={LEFT:0,MIDDLE:1,RIGHT:2,ROTATE:0,DOLLY:1,PAN:2},os={ROTATE:0,PAN:1,DOLLY_PAN:2,DOLLY_ROTATE:3},wd=0,dh=1,Ed=2;var Co=1,cl=2,Tr=3,Ln=0,tn=1,Nt=2,Wn=0,Ss=1,Xt=2,fh=3,ph=4,Td=5;var Qi=100,Ad=101,Rd=102,Cd=103,Pd=104,Id=200,Ld=201,Dd=202,Nd=203,Ra=204,Ca=205,Ud=206,Fd=207,Od=208,Bd=209,zd=210,kd=211,Hd=212,Vd=213,Gd=214,Pa=0,Ia=1,La=2,ws=3,Da=4,Na=5,Ua=6,Fa=7,hl=0,Wd=1,Xd=2,ii=0,Po=1,Io=2,Lo=3,Us=4,Do=5,No=6,Uo=7,eh="attached",qd="detached",mh=300,as=301,Fs=302,ul=303,dl=304,Fo=306,ui=1e3,Hn=1001,ar=1002,Kt=1003,fl=1004;var Os=1005;var kt=1006,Ar=1007;var si=1008;var Sn=1009,gh=1010,xh=1011,Rr=1012,pl=1013,ri=1014,Un=1015,nn=1016,ml=1017,gl=1018,Cr=1020,_h=35902,vh=35899,yh=1021,Mh=1022,Fn=1023,di=1026,ls=1027,xl=1028,_l=1029,cs=1030,vl=1031;var yl=1033,Oo=33776,Bo=33777,zo=33778,ko=33779,Ml=35840,bl=35841,Sl=35842,wl=35843,El=36196,Tl=37492,Al=37496,Rl=37488,Cl=37489,Ho=37490,Pl=37491,Il=37808,Ll=37809,Dl=37810,Nl=37811,Ul=37812,Fl=37813,Ol=37814,Bl=37815,zl=37816,kl=37817,Hl=37818,Vl=37819,Gl=37820,Wl=37821,Xl=36492,ql=36494,Yl=36495,Zl=36283,Kl=36284,Vo=36285,jl=36286,Jl=2200,Yd=2201,Zd=2202,Es=2300,Ts=2301,Aa=2302,th=2303,ys=2400,Ms=2401,Qr=2402,$l=2500,Kd=2501,bh=0,Go=1,Pr=2,jd=3200;var Wo=0,Jd=1,Xn="",zt="srgb",gn="srgb-linear",eo="linear",Mt="srgb";var vs=7680;var nh=519,$d=512,Qd=513,ef=514,Ql=515,tf=516,nf=517,ec=518,sf=519,Oa=35044;var Sh="300 es",Qn=2e3,lr=2001;function Fp(s){for(let e=s.length-1;e>=0;--e)if(s[e]>=65535)return!0;return!1}function Op(s){return ArrayBuffer.isView(s)&&!(s instanceof DataView)}function cr(s){return document.createElementNS("http://www.w3.org/1999/xhtml",s)}function rf(){let s=cr("canvas");return s.style.display="block",s}var Fu={},hr=null;function to(...s){let e="THREE."+s.shift();hr?hr("log",e,...s):console.log(e,...s)}function of(s){let e=s[0];if(typeof e=="string"&&e.startsWith("TSL:")){let t=s[1];t&&t.isStackTrace?s[0]+=" "+t.getLocation():s[1]='Stack trace not available. Enable "THREE.Node.captureStackTrace" to capture stack traces.'}return s}function We(...s){s=of(s);let e="THREE."+s.shift();if(hr)hr("warn",e,...s);else{let t=s[0];t&&t.isStackTrace?console.warn(t.getError(e)):console.warn(e,...s)}}function $e(...s){s=of(s);let e="THREE."+s.shift();if(hr)hr("error",e,...s);else{let t=s[0];t&&t.isStackTrace?console.error(t.getError(e)):console.error(e,...s)}}function bs(...s){let e=s.join(" ");e in Fu||(Fu[e]=!0,We(...s))}function af(s,e,t){return new Promise(function(n,i){function r(){switch(s.clientWaitSync(e,s.SYNC_FLUSH_COMMANDS_BIT,0)){case s.WAIT_FAILED:i();break;case s.TIMEOUT_EXPIRED:setTimeout(r,t);break;default:n()}}setTimeout(r,t)})}var lf={[Pa]:Ia,[La]:Ua,[Da]:Fa,[ws]:Na,[Ia]:Pa,[Ua]:La,[Fa]:Da,[Na]:ws},Vn=class{addEventListener(e,t){this._listeners===void 0&&(this._listeners={});let n=this._listeners;n[e]===void 0&&(n[e]=[]),n[e].indexOf(t)===-1&&n[e].push(t)}hasEventListener(e,t){let n=this._listeners;return n===void 0?!1:n[e]!==void 0&&n[e].indexOf(t)!==-1}removeEventListener(e,t){let n=this._listeners;if(n===void 0)return;let i=n[e];if(i!==void 0){let r=i.indexOf(t);r!==-1&&i.splice(r,1)}}dispatchEvent(e){let t=this._listeners;if(t===void 0)return;let n=t[e.type];if(n!==void 0){e.target=this;let i=n.slice(0);for(let r=0,o=i.length;r<o;r++)i[r].call(this,e);e.target=null}}},hn=["00","01","02","03","04","05","06","07","08","09","0a","0b","0c","0d","0e","0f","10","11","12","13","14","15","16","17","18","19","1a","1b","1c","1d","1e","1f","20","21","22","23","24","25","26","27","28","29","2a","2b","2c","2d","2e","2f","30","31","32","33","34","35","36","37","38","39","3a","3b","3c","3d","3e","3f","40","41","42","43","44","45","46","47","48","49","4a","4b","4c","4d","4e","4f","50","51","52","53","54","55","56","57","58","59","5a","5b","5c","5d","5e","5f","60","61","62","63","64","65","66","67","68","69","6a","6b","6c","6d","6e","6f","70","71","72","73","74","75","76","77","78","79","7a","7b","7c","7d","7e","7f","80","81","82","83","84","85","86","87","88","89","8a","8b","8c","8d","8e","8f","90","91","92","93","94","95","96","97","98","99","9a","9b","9c","9d","9e","9f","a0","a1","a2","a3","a4","a5","a6","a7","a8","a9","aa","ab","ac","ad","ae","af","b0","b1","b2","b3","b4","b5","b6","b7","b8","b9","ba","bb","bc","bd","be","bf","c0","c1","c2","c3","c4","c5","c6","c7","c8","c9","ca","cb","cc","cd","ce","cf","d0","d1","d2","d3","d4","d5","d6","d7","d8","d9","da","db","dc","dd","de","df","e0","e1","e2","e3","e4","e5","e6","e7","e8","e9","ea","eb","ec","ed","ee","ef","f0","f1","f2","f3","f4","f5","f6","f7","f8","f9","fa","fb","fc","fd","fe","ff"],Ou=1234567,Kr=Math.PI/180,As=180/Math.PI;function ei(){let s=Math.random()*4294967295|0,e=Math.random()*4294967295|0,t=Math.random()*4294967295|0,n=Math.random()*4294967295|0;return(hn[s&255]+hn[s>>8&255]+hn[s>>16&255]+hn[s>>24&255]+"-"+hn[e&255]+hn[e>>8&255]+"-"+hn[e>>16&15|64]+hn[e>>24&255]+"-"+hn[t&63|128]+hn[t>>8&255]+"-"+hn[t>>16&255]+hn[t>>24&255]+hn[n&255]+hn[n>>8&255]+hn[n>>16&255]+hn[n>>24&255]).toLowerCase()}function at(s,e,t){return Math.max(e,Math.min(t,s))}function wh(s,e){return(s%e+e)%e}function Bp(s,e,t,n,i){return n+(s-e)*(i-n)/(t-e)}function zp(s,e,t){return s!==e?(t-s)/(e-s):0}function jr(s,e,t){return(1-t)*s+t*e}function kp(s,e,t,n){return jr(s,e,1-Math.exp(-t*n))}function Hp(s,e=1){return e-Math.abs(wh(s,e*2)-e)}function Vp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*(3-2*s))}function Gp(s,e,t){return s<=e?0:s>=t?1:(s=(s-e)/(t-e),s*s*s*(s*(s*6-15)+10))}function Wp(s,e){return s+Math.floor(Math.random()*(e-s+1))}function Xp(s,e){return s+Math.random()*(e-s)}function qp(s){return s*(.5-Math.random())}function Yp(s){s!==void 0&&(Ou=s);let e=Ou+=1831565813;return e=Math.imul(e^e>>>15,e|1),e^=e+Math.imul(e^e>>>7,e|61),((e^e>>>14)>>>0)/4294967296}function Zp(s){return s*Kr}function Kp(s){return s*As}function jp(s){return(s&s-1)===0&&s!==0}function Jp(s){return Math.pow(2,Math.ceil(Math.log(s)/Math.LN2))}function $p(s){return Math.pow(2,Math.floor(Math.log(s)/Math.LN2))}function Qp(s,e,t,n,i){let r=Math.cos,o=Math.sin,a=r(t/2),l=o(t/2),c=r((e+n)/2),h=o((e+n)/2),u=r((e-n)/2),d=o((e-n)/2),f=r((n-e)/2),g=o((n-e)/2);switch(i){case"XYX":s.set(a*h,l*u,l*d,a*c);break;case"YZY":s.set(l*d,a*h,l*u,a*c);break;case"ZXZ":s.set(l*u,l*d,a*h,a*c);break;case"XZX":s.set(a*h,l*g,l*f,a*c);break;case"YXY":s.set(l*f,a*h,l*g,a*c);break;case"ZYZ":s.set(l*g,l*f,a*h,a*c);break;default:We("MathUtils: .setQuaternionFromProperEuler() encountered an unknown order: "+i)}}function $n(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return s/4294967295;case Uint16Array:return s/65535;case Uint8Array:return s/255;case Int32Array:return Math.max(s/2147483647,-1);case Int16Array:return Math.max(s/32767,-1);case Int8Array:return Math.max(s/127,-1);default:throw new Error("THREE.MathUtils: Invalid component type.")}}function Et(s,e){switch(e.constructor){case Float32Array:return s;case Uint32Array:return Math.round(s*4294967295);case Uint16Array:return Math.round(s*65535);case Uint8Array:return Math.round(s*255);case Int32Array:return Math.round(s*2147483647);case Int16Array:return Math.round(s*32767);case Int8Array:return Math.round(s*127);default:throw new Error("THREE.MathUtils: Invalid component type.")}}var wt={DEG2RAD:Kr,RAD2DEG:As,generateUUID:ei,clamp:at,euclideanModulo:wh,mapLinear:Bp,inverseLerp:zp,lerp:jr,damp:kp,pingpong:Hp,smoothstep:Vp,smootherstep:Gp,randInt:Wp,randFloat:Xp,randFloatSpread:qp,seededRandom:Yp,degToRad:Zp,radToDeg:Kp,isPowerOfTwo:jp,ceilPowerOfTwo:Jp,floorPowerOfTwo:$p,setQuaternionFromProperEuler:Qp,normalize:Et,denormalize:$n},_e=class s{static{s.prototype.isVector2=!0}constructor(e=0,t=0){this.x=e,this.y=t}get width(){return this.x}set width(e){this.x=e}get height(){return this.y}set height(e){this.y=e}set(e,t){return this.x=e,this.y=t,this}setScalar(e){return this.x=e,this.y=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;default:throw new Error("THREE.Vector2: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;default:throw new Error("THREE.Vector2: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y)}copy(e){return this.x=e.x,this.y=e.y,this}add(e){return this.x+=e.x,this.y+=e.y,this}addScalar(e){return this.x+=e,this.y+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this}subScalar(e){return this.x-=e,this.y-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this}multiply(e){return this.x*=e.x,this.y*=e.y,this}multiplyScalar(e){return this.x*=e,this.y*=e,this}divide(e){return this.x/=e.x,this.y/=e.y,this}divideScalar(e){return this.multiplyScalar(1/e)}applyMatrix3(e){let t=this.x,n=this.y,i=e.elements;return this.x=i[0]*t+i[3]*n+i[6],this.y=i[1]*t+i[4]*n+i[7],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this}clamp(e,t){return this.x=at(this.x,e.x,t.x),this.y=at(this.y,e.y,t.y),this}clampScalar(e,t){return this.x=at(this.x,e,t),this.y=at(this.y,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(at(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this}negate(){return this.x=-this.x,this.y=-this.y,this}dot(e){return this.x*e.x+this.y*e.y}cross(e){return this.x*e.y-this.y*e.x}lengthSq(){return this.x*this.x+this.y*this.y}length(){return Math.sqrt(this.x*this.x+this.y*this.y)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)}normalize(){return this.divideScalar(this.length()||1)}angle(){return Math.atan2(-this.y,-this.x)+Math.PI}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(at(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y;return t*t+n*n}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this}equals(e){return e.x===this.x&&e.y===this.y}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this}rotateAround(e,t){let n=Math.cos(t),i=Math.sin(t),r=this.x-e.x,o=this.y-e.y;return this.x=r*n-o*i+e.x,this.y=r*i+o*n+e.y,this}random(){return this.x=Math.random(),this.y=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y}},Zt=class{constructor(e=0,t=0,n=0,i=1){this.isQuaternion=!0,this._x=e,this._y=t,this._z=n,this._w=i}static slerpFlat(e,t,n,i,r,o,a){let l=n[i+0],c=n[i+1],h=n[i+2],u=n[i+3],d=r[o+0],f=r[o+1],g=r[o+2],y=r[o+3];if(u!==y||l!==d||c!==f||h!==g){let m=l*d+c*f+h*g+u*y;m<0&&(d=-d,f=-f,g=-g,y=-y,m=-m);let p=1-a;if(m<.9995){let b=Math.acos(m),S=Math.sin(b);p=Math.sin(p*b)/S,a=Math.sin(a*b)/S,l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+y*a}else{l=l*p+d*a,c=c*p+f*a,h=h*p+g*a,u=u*p+y*a;let b=1/Math.sqrt(l*l+c*c+h*h+u*u);l*=b,c*=b,h*=b,u*=b}}e[t]=l,e[t+1]=c,e[t+2]=h,e[t+3]=u}static multiplyQuaternionsFlat(e,t,n,i,r,o){let a=n[i],l=n[i+1],c=n[i+2],h=n[i+3],u=r[o],d=r[o+1],f=r[o+2],g=r[o+3];return e[t]=a*g+h*u+l*f-c*d,e[t+1]=l*g+h*d+c*u-a*f,e[t+2]=c*g+h*f+a*d-l*u,e[t+3]=h*g-a*u-l*d-c*f,e}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get w(){return this._w}set w(e){this._w=e,this._onChangeCallback()}set(e,t,n,i){return this._x=e,this._y=t,this._z=n,this._w=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._w)}copy(e){return this._x=e.x,this._y=e.y,this._z=e.z,this._w=e.w,this._onChangeCallback(),this}setFromEuler(e,t=!0){let n=e._x,i=e._y,r=e._z,o=e._order,a=Math.cos,l=Math.sin,c=a(n/2),h=a(i/2),u=a(r/2),d=l(n/2),f=l(i/2),g=l(r/2);switch(o){case"XYZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"YXZ":this._x=d*h*u+c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"ZXY":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u-d*f*g;break;case"ZYX":this._x=d*h*u-c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u+d*f*g;break;case"YZX":this._x=d*h*u+c*f*g,this._y=c*f*u+d*h*g,this._z=c*h*g-d*f*u,this._w=c*h*u-d*f*g;break;case"XZY":this._x=d*h*u-c*f*g,this._y=c*f*u-d*h*g,this._z=c*h*g+d*f*u,this._w=c*h*u+d*f*g;break;default:We("Quaternion: .setFromEuler() encountered an unknown order: "+o)}return t===!0&&this._onChangeCallback(),this}setFromAxisAngle(e,t){let n=t/2,i=Math.sin(n);return this._x=e.x*i,this._y=e.y*i,this._z=e.z*i,this._w=Math.cos(n),this._onChangeCallback(),this}setFromRotationMatrix(e){let t=e.elements,n=t[0],i=t[4],r=t[8],o=t[1],a=t[5],l=t[9],c=t[2],h=t[6],u=t[10],d=n+a+u;if(d>0){let f=.5/Math.sqrt(d+1);this._w=.25/f,this._x=(h-l)*f,this._y=(r-c)*f,this._z=(o-i)*f}else if(n>a&&n>u){let f=2*Math.sqrt(1+n-a-u);this._w=(h-l)/f,this._x=.25*f,this._y=(i+o)/f,this._z=(r+c)/f}else if(a>u){let f=2*Math.sqrt(1+a-n-u);this._w=(r-c)/f,this._x=(i+o)/f,this._y=.25*f,this._z=(l+h)/f}else{let f=2*Math.sqrt(1+u-n-a);this._w=(o-i)/f,this._x=(r+c)/f,this._y=(l+h)/f,this._z=.25*f}return this._onChangeCallback(),this}setFromUnitVectors(e,t){let n=e.dot(t)+1;return n<1e-8?(n=0,Math.abs(e.x)>Math.abs(e.z)?(this._x=-e.y,this._y=e.x,this._z=0,this._w=n):(this._x=0,this._y=-e.z,this._z=e.y,this._w=n)):(this._x=e.y*t.z-e.z*t.y,this._y=e.z*t.x-e.x*t.z,this._z=e.x*t.y-e.y*t.x,this._w=n),this.normalize()}angleTo(e){return 2*Math.acos(Math.abs(at(this.dot(e),-1,1)))}rotateTowards(e,t){let n=this.angleTo(e);if(n===0)return this;let i=Math.min(1,t/n);return this.slerp(e,i),this}identity(){return this.set(0,0,0,1)}invert(){return this.conjugate()}conjugate(){return this._x*=-1,this._y*=-1,this._z*=-1,this._onChangeCallback(),this}dot(e){return this._x*e._x+this._y*e._y+this._z*e._z+this._w*e._w}lengthSq(){return this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w}length(){return Math.sqrt(this._x*this._x+this._y*this._y+this._z*this._z+this._w*this._w)}normalize(){let e=this.length();return e===0?(this._x=0,this._y=0,this._z=0,this._w=1):(e=1/e,this._x=this._x*e,this._y=this._y*e,this._z=this._z*e,this._w=this._w*e),this._onChangeCallback(),this}multiply(e){return this.multiplyQuaternions(this,e)}premultiply(e){return this.multiplyQuaternions(e,this)}multiplyQuaternions(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=t._x,l=t._y,c=t._z,h=t._w;return this._x=n*h+o*a+i*c-r*l,this._y=i*h+o*l+r*a-n*c,this._z=r*h+o*c+n*l-i*a,this._w=o*h-n*a-i*l-r*c,this._onChangeCallback(),this}slerp(e,t){let n=e._x,i=e._y,r=e._z,o=e._w,a=this.dot(e);a<0&&(n=-n,i=-i,r=-r,o=-o,a=-a);let l=1-t;if(a<.9995){let c=Math.acos(a),h=Math.sin(c);l=Math.sin(l*c)/h,t=Math.sin(t*c)/h,this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this._onChangeCallback()}else this._x=this._x*l+n*t,this._y=this._y*l+i*t,this._z=this._z*l+r*t,this._w=this._w*l+o*t,this.normalize();return this}slerpQuaternions(e,t,n){return this.copy(e).slerp(t,n)}random(){let e=2*Math.PI*Math.random(),t=2*Math.PI*Math.random(),n=Math.random(),i=Math.sqrt(1-n),r=Math.sqrt(n);return this.set(i*Math.sin(e),i*Math.cos(e),r*Math.sin(t),r*Math.cos(t))}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._w===this._w}fromArray(e,t=0){return this._x=e[t],this._y=e[t+1],this._z=e[t+2],this._w=e[t+3],this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._w,e}fromBufferAttribute(e,t){return this._x=e.getX(t),this._y=e.getY(t),this._z=e.getZ(t),this._w=e.getW(t),this._onChangeCallback(),this}toJSON(){return this.toArray()}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._w}},B=class s{static{s.prototype.isVector3=!0}constructor(e=0,t=0,n=0){this.x=e,this.y=t,this.z=n}set(e,t,n){return n===void 0&&(n=this.z),this.x=e,this.y=t,this.z=n,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;default:throw new Error("THREE.Vector3: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;default:throw new Error("THREE.Vector3: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this}multiplyVectors(e,t){return this.x=e.x*t.x,this.y=e.y*t.y,this.z=e.z*t.z,this}applyEuler(e){return this.applyQuaternion(Bu.setFromEuler(e))}applyAxisAngle(e,t){return this.applyQuaternion(Bu.setFromAxisAngle(e,t))}applyMatrix3(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[3]*n+r[6]*i,this.y=r[1]*t+r[4]*n+r[7]*i,this.z=r[2]*t+r[5]*n+r[8]*i,this}applyNormalMatrix(e){return this.applyMatrix3(e).normalize()}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=e.elements,o=1/(r[3]*t+r[7]*n+r[11]*i+r[15]);return this.x=(r[0]*t+r[4]*n+r[8]*i+r[12])*o,this.y=(r[1]*t+r[5]*n+r[9]*i+r[13])*o,this.z=(r[2]*t+r[6]*n+r[10]*i+r[14])*o,this}applyQuaternion(e){let t=this.x,n=this.y,i=this.z,r=e.x,o=e.y,a=e.z,l=e.w,c=2*(o*i-a*n),h=2*(a*t-r*i),u=2*(r*n-o*t);return this.x=t+l*c+o*u-a*h,this.y=n+l*h+a*c-r*u,this.z=i+l*u+r*h-o*c,this}project(e){return this.applyMatrix4(e.matrixWorldInverse).applyMatrix4(e.projectionMatrix)}unproject(e){return this.applyMatrix4(e.projectionMatrixInverse).applyMatrix4(e.matrixWorld)}transformDirection(e){let t=this.x,n=this.y,i=this.z,r=e.elements;return this.x=r[0]*t+r[4]*n+r[8]*i,this.y=r[1]*t+r[5]*n+r[9]*i,this.z=r[2]*t+r[6]*n+r[10]*i,this.normalize()}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this}divideScalar(e){return this.multiplyScalar(1/e)}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this}clamp(e,t){return this.x=at(this.x,e.x,t.x),this.y=at(this.y,e.y,t.y),this.z=at(this.z,e.z,t.z),this}clampScalar(e,t){return this.x=at(this.x,e,t),this.y=at(this.y,e,t),this.z=at(this.z,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(at(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this}cross(e){return this.crossVectors(this,e)}crossVectors(e,t){let n=e.x,i=e.y,r=e.z,o=t.x,a=t.y,l=t.z;return this.x=i*l-r*a,this.y=r*o-n*l,this.z=n*a-i*o,this}projectOnVector(e){let t=e.lengthSq();if(t===0)return this.set(0,0,0);let n=e.dot(this)/t;return this.copy(e).multiplyScalar(n)}projectOnPlane(e){return Ec.copy(this).projectOnVector(e),this.sub(Ec)}reflect(e){return this.sub(Ec.copy(e).multiplyScalar(2*this.dot(e)))}angleTo(e){let t=Math.sqrt(this.lengthSq()*e.lengthSq());if(t===0)return Math.PI/2;let n=this.dot(e)/t;return Math.acos(at(n,-1,1))}distanceTo(e){return Math.sqrt(this.distanceToSquared(e))}distanceToSquared(e){let t=this.x-e.x,n=this.y-e.y,i=this.z-e.z;return t*t+n*n+i*i}manhattanDistanceTo(e){return Math.abs(this.x-e.x)+Math.abs(this.y-e.y)+Math.abs(this.z-e.z)}setFromSpherical(e){return this.setFromSphericalCoords(e.radius,e.phi,e.theta)}setFromSphericalCoords(e,t,n){let i=Math.sin(t)*e;return this.x=i*Math.sin(n),this.y=Math.cos(t)*e,this.z=i*Math.cos(n),this}setFromCylindrical(e){return this.setFromCylindricalCoords(e.radius,e.theta,e.y)}setFromCylindricalCoords(e,t,n){return this.x=e*Math.sin(t),this.y=n,this.z=e*Math.cos(t),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this}setFromMatrixScale(e){let t=this.setFromMatrixColumn(e,0).length(),n=this.setFromMatrixColumn(e,1).length(),i=this.setFromMatrixColumn(e,2).length();return this.x=t,this.y=n,this.z=i,this}setFromMatrixColumn(e,t){return this.fromArray(e.elements,t*4)}setFromMatrix3Column(e,t){return this.fromArray(e.elements,t*3)}setFromEuler(e){return this.x=e._x,this.y=e._y,this.z=e._z,this}setFromColor(e){return this.x=e.r,this.y=e.g,this.z=e.b,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this}randomDirection(){let e=Math.random()*Math.PI*2,t=Math.random()*2-1,n=Math.sqrt(1-t*t);return this.x=n*Math.cos(e),this.y=t,this.z=n*Math.sin(e),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z}},Ec=new B,Bu=new Zt,et=class s{static{s.prototype.isMatrix3=!0}constructor(e,t,n,i,r,o,a,l,c){this.elements=[1,0,0,0,1,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c)}set(e,t,n,i,r,o,a,l,c){let h=this.elements;return h[0]=e,h[1]=i,h[2]=a,h[3]=t,h[4]=r,h[5]=l,h[6]=n,h[7]=o,h[8]=c,this}identity(){return this.set(1,0,0,0,1,0,0,0,1),this}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],this}extractBasis(e,t,n){return e.setFromMatrix3Column(this,0),t.setFromMatrix3Column(this,1),n.setFromMatrix3Column(this,2),this}setFromMatrix4(e){let t=e.elements;return this.set(t[0],t[4],t[8],t[1],t[5],t[9],t[2],t[6],t[10]),this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[3],l=n[6],c=n[1],h=n[4],u=n[7],d=n[2],f=n[5],g=n[8],y=i[0],m=i[3],p=i[6],b=i[1],S=i[4],x=i[7],A=i[2],T=i[5],I=i[8];return r[0]=o*y+a*b+l*A,r[3]=o*m+a*S+l*T,r[6]=o*p+a*x+l*I,r[1]=c*y+h*b+u*A,r[4]=c*m+h*S+u*T,r[7]=c*p+h*x+u*I,r[2]=d*y+f*b+g*A,r[5]=d*m+f*S+g*T,r[8]=d*p+f*x+g*I,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[3]*=e,t[6]*=e,t[1]*=e,t[4]*=e,t[7]*=e,t[2]*=e,t[5]*=e,t[8]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8];return t*o*h-t*a*c-n*r*h+n*a*l+i*r*c-i*o*l}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=h*o-a*c,d=a*l-h*r,f=c*r-o*l,g=t*u+n*d+i*f;if(g===0)return this.set(0,0,0,0,0,0,0,0,0);let y=1/g;return e[0]=u*y,e[1]=(i*c-h*n)*y,e[2]=(a*n-i*o)*y,e[3]=d*y,e[4]=(h*t-i*l)*y,e[5]=(i*r-a*t)*y,e[6]=f*y,e[7]=(n*l-c*t)*y,e[8]=(o*t-n*r)*y,this}transpose(){let e,t=this.elements;return e=t[1],t[1]=t[3],t[3]=e,e=t[2],t[2]=t[6],t[6]=e,e=t[5],t[5]=t[7],t[7]=e,this}getNormalMatrix(e){return this.setFromMatrix4(e).invert().transpose()}transposeIntoArray(e){let t=this.elements;return e[0]=t[0],e[1]=t[3],e[2]=t[6],e[3]=t[1],e[4]=t[4],e[5]=t[7],e[6]=t[2],e[7]=t[5],e[8]=t[8],this}setUvTransform(e,t,n,i,r,o,a){let l=Math.cos(r),c=Math.sin(r);return this.set(n*l,n*c,-n*(l*o+c*a)+o+e,-i*c,i*l,-i*(-c*o+l*a)+a+t,0,0,1),this}scale(e,t){return bs("Matrix3: .scale() is deprecated. Use .makeScale() instead."),this.premultiply(Tc.makeScale(e,t)),this}rotate(e){return bs("Matrix3: .rotate() is deprecated. Use .makeRotation() instead."),this.premultiply(Tc.makeRotation(-e)),this}translate(e,t){return bs("Matrix3: .translate() is deprecated. Use .makeTranslation() instead."),this.premultiply(Tc.makeTranslation(e,t)),this}makeTranslation(e,t){return e.isVector2?this.set(1,0,e.x,0,1,e.y,0,0,1):this.set(1,0,e,0,1,t,0,0,1),this}makeRotation(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,n,t,0,0,0,1),this}makeScale(e,t){return this.set(e,0,0,0,t,0,0,0,1),this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<9;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<9;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e}clone(){return new this.constructor().fromArray(this.elements)}},Tc=new et,zu=new et().set(.4123908,.3575843,.1804808,.212639,.7151687,.0721923,.0193308,.1191948,.9505322),ku=new et().set(3.2409699,-1.5373832,-.4986108,-.9692436,1.8759675,.0415551,.0556301,-.203977,1.0569715);function em(){let s={enabled:!0,workingColorSpace:gn,spaces:{},convert:function(i,r,o){return this.enabled===!1||r===o||!r||!o||(this.spaces[r].transfer===Mt&&(i.r=Li(i.r),i.g=Li(i.g),i.b=Li(i.b)),this.spaces[r].primaries!==this.spaces[o].primaries&&(i.applyMatrix3(this.spaces[r].toXYZ),i.applyMatrix3(this.spaces[o].fromXYZ)),this.spaces[o].transfer===Mt&&(i.r=or(i.r),i.g=or(i.g),i.b=or(i.b))),i},workingToColorSpace:function(i,r){return this.convert(i,this.workingColorSpace,r)},colorSpaceToWorking:function(i,r){return this.convert(i,r,this.workingColorSpace)},getPrimaries:function(i){return this.spaces[i].primaries},getTransfer:function(i){return i===Xn?eo:this.spaces[i].transfer},getToneMappingMode:function(i){return this.spaces[i].outputColorSpaceConfig.toneMappingMode||"standard"},getLuminanceCoefficients:function(i,r=this.workingColorSpace){return i.fromArray(this.spaces[r].luminanceCoefficients)},define:function(i){Object.assign(this.spaces,i)},_getMatrix:function(i,r,o){return i.copy(this.spaces[r].toXYZ).multiply(this.spaces[o].fromXYZ)},_getDrawingBufferColorSpace:function(i){return this.spaces[i].outputColorSpaceConfig.drawingBufferColorSpace},_getUnpackColorSpace:function(i=this.workingColorSpace){return this.spaces[i].workingColorSpaceConfig.unpackColorSpace},fromWorkingColorSpace:function(i,r){return bs("ColorManagement: .fromWorkingColorSpace() has been renamed to .workingToColorSpace()."),s.workingToColorSpace(i,r)},toWorkingColorSpace:function(i,r){return bs("ColorManagement: .toWorkingColorSpace() has been renamed to .colorSpaceToWorking()."),s.colorSpaceToWorking(i,r)}},e=[.64,.33,.3,.6,.15,.06],t=[.2126,.7152,.0722],n=[.3127,.329];return s.define({[gn]:{primaries:e,whitePoint:n,transfer:eo,toXYZ:zu,fromXYZ:ku,luminanceCoefficients:t,workingColorSpaceConfig:{unpackColorSpace:zt},outputColorSpaceConfig:{drawingBufferColorSpace:zt}},[zt]:{primaries:e,whitePoint:n,transfer:Mt,toXYZ:zu,fromXYZ:ku,luminanceCoefficients:t,outputColorSpaceConfig:{drawingBufferColorSpace:zt}}}),s}var ot=em();function Li(s){return s<.04045?s*.0773993808:Math.pow(s*.9478672986+.0521327014,2.4)}function or(s){return s<.0031308?s*12.92:1.055*Math.pow(s,.41666)-.055}var qs,Ba=class{static getDataURL(e,t="image/png"){if(/^data:/i.test(e.src)||typeof HTMLCanvasElement>"u")return e.src;let n;if(e instanceof HTMLCanvasElement)n=e;else{qs===void 0&&(qs=cr("canvas")),qs.width=e.width,qs.height=e.height;let i=qs.getContext("2d");e instanceof ImageData?i.putImageData(e,0,0):i.drawImage(e,0,0,e.width,e.height),n=qs}return n.toDataURL(t)}static sRGBToLinear(e){if(typeof HTMLImageElement<"u"&&e instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&e instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&e instanceof ImageBitmap){let t=cr("canvas");t.width=e.width,t.height=e.height;let n=t.getContext("2d");n.drawImage(e,0,0,e.width,e.height);let i=n.getImageData(0,0,e.width,e.height),r=i.data;for(let o=0;o<r.length;o++)r[o]=Li(r[o]/255)*255;return n.putImageData(i,0,0),t}else if(e.data){let t=e.data.slice(0);for(let n=0;n<t.length;n++)t instanceof Uint8Array||t instanceof Uint8ClampedArray?t[n]=Math.floor(Li(t[n]/255)*255):t[n]=Li(t[n]);return{data:t,width:e.width,height:e.height}}else return We("ImageUtils.sRGBToLinear(): Unsupported image type. No color space conversion applied."),e}},tm=0,ur=class{constructor(e=null){this.isSource=!0,Object.defineProperty(this,"id",{value:tm++}),this.uuid=ei(),this.data=e,this.dataReady=!0,this.version=0}getSize(e){let t=this.data;return typeof HTMLVideoElement<"u"&&t instanceof HTMLVideoElement?e.set(t.videoWidth,t.videoHeight,0):typeof VideoFrame<"u"&&t instanceof VideoFrame?e.set(t.displayWidth,t.displayHeight,0):t!==null?e.set(t.width,t.height,t.depth||0):e.set(0,0,0),e}set needsUpdate(e){e===!0&&this.version++}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.images[this.uuid]!==void 0)return e.images[this.uuid];let n={uuid:this.uuid,url:""},i=this.data;if(i!==null){let r;if(Array.isArray(i)){r=[];for(let o=0,a=i.length;o<a;o++)i[o].isDataTexture?r.push(Ac(i[o].image)):r.push(Ac(i[o]))}else r=Ac(i);n.url=r}return t||(e.images[this.uuid]=n),n}};function Ac(s){return typeof HTMLImageElement<"u"&&s instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&s instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&s instanceof ImageBitmap?Ba.getDataURL(s):s.data?{data:Array.from(s.data),width:s.width,height:s.height,type:s.data.constructor.name}:(We("Texture: Unable to serialize Texture."),{})}var nm=0,Rc=new B,jt=class s extends Vn{constructor(e=s.DEFAULT_IMAGE,t=s.DEFAULT_MAPPING,n=Hn,i=Hn,r=kt,o=si,a=Fn,l=Sn,c=s.DEFAULT_ANISOTROPY,h=Xn){super(),this.isTexture=!0,Object.defineProperty(this,"id",{value:nm++}),this.uuid=ei(),this.name="",this.source=new ur(e),this.mipmaps=[],this.mapping=t,this.channel=0,this.wrapS=n,this.wrapT=i,this.magFilter=r,this.minFilter=o,this.anisotropy=c,this.format=a,this.internalFormat=null,this.type=l,this.offset=new _e(0,0),this.repeat=new _e(1,1),this.center=new _e(0,0),this.rotation=0,this.matrixAutoUpdate=!0,this.matrix=new et,this.generateMipmaps=!0,this.premultiplyAlpha=!1,this.flipY=!0,this.unpackAlignment=4,this.colorSpace=h,this.userData={},this.updateRanges=[],this.version=0,this.onUpdate=null,this.renderTarget=null,this.isRenderTargetTexture=!1,this.isArrayTexture=!!(e&&e.depth&&e.depth>1),this.pmremVersion=0,this.normalized=!1}get width(){return this.source.getSize(Rc).x}get height(){return this.source.getSize(Rc).y}get depth(){return this.source.getSize(Rc).z}get image(){return this.source.data}set image(e){this.source.data=e}updateMatrix(){this.matrix.setUvTransform(this.offset.x,this.offset.y,this.repeat.x,this.repeat.y,this.rotation,this.center.x,this.center.y)}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}clone(){return new this.constructor().copy(this)}copy(e){return this.name=e.name,this.source=e.source,this.mipmaps=e.mipmaps.slice(0),this.mapping=e.mapping,this.channel=e.channel,this.wrapS=e.wrapS,this.wrapT=e.wrapT,this.magFilter=e.magFilter,this.minFilter=e.minFilter,this.anisotropy=e.anisotropy,this.format=e.format,this.internalFormat=e.internalFormat,this.type=e.type,this.normalized=e.normalized,this.offset.copy(e.offset),this.repeat.copy(e.repeat),this.center.copy(e.center),this.rotation=e.rotation,this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrix.copy(e.matrix),this.generateMipmaps=e.generateMipmaps,this.premultiplyAlpha=e.premultiplyAlpha,this.flipY=e.flipY,this.unpackAlignment=e.unpackAlignment,this.colorSpace=e.colorSpace,this.renderTarget=e.renderTarget,this.isRenderTargetTexture=e.isRenderTargetTexture,this.isArrayTexture=e.isArrayTexture,this.userData=JSON.parse(JSON.stringify(e.userData)),this.needsUpdate=!0,this}setValues(e){for(let t in e){let n=e[t];if(n===void 0){We(`Texture.setValues(): parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){We(`Texture.setValues(): property '${t}' does not exist.`);continue}i&&n&&i.isVector2&&n.isVector2||i&&n&&i.isVector3&&n.isVector3||i&&n&&i.isMatrix3&&n.isMatrix3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";if(!t&&e.textures[this.uuid]!==void 0)return e.textures[this.uuid];let n={metadata:{version:4.7,type:"Texture",generator:"Texture.toJSON"},uuid:this.uuid,name:this.name,image:this.source.toJSON(e).uuid,mapping:this.mapping,channel:this.channel,repeat:[this.repeat.x,this.repeat.y],offset:[this.offset.x,this.offset.y],center:[this.center.x,this.center.y],rotation:this.rotation,wrap:[this.wrapS,this.wrapT],format:this.format,internalFormat:this.internalFormat,type:this.type,normalized:this.normalized,colorSpace:this.colorSpace,minFilter:this.minFilter,magFilter:this.magFilter,anisotropy:this.anisotropy,flipY:this.flipY,generateMipmaps:this.generateMipmaps,premultiplyAlpha:this.premultiplyAlpha,unpackAlignment:this.unpackAlignment};return Object.keys(this.userData).length>0&&(n.userData=this.userData),t||(e.textures[this.uuid]=n),n}dispose(){this.dispatchEvent({type:"dispose"})}transformUv(e){if(this.mapping!==mh)return e;if(e.applyMatrix3(this.matrix),e.x<0||e.x>1)switch(this.wrapS){case ui:e.x=e.x-Math.floor(e.x);break;case Hn:e.x=e.x<0?0:1;break;case ar:Math.abs(Math.floor(e.x)%2)===1?e.x=Math.ceil(e.x)-e.x:e.x=e.x-Math.floor(e.x);break}if(e.y<0||e.y>1)switch(this.wrapT){case ui:e.y=e.y-Math.floor(e.y);break;case Hn:e.y=e.y<0?0:1;break;case ar:Math.abs(Math.floor(e.y)%2)===1?e.y=Math.ceil(e.y)-e.y:e.y=e.y-Math.floor(e.y);break}return this.flipY&&(e.y=1-e.y),e}set needsUpdate(e){e===!0&&(this.version++,this.source.needsUpdate=!0)}set needsPMREMUpdate(e){e===!0&&this.pmremVersion++}};jt.DEFAULT_IMAGE=null;jt.DEFAULT_MAPPING=mh;jt.DEFAULT_ANISOTROPY=1;var _t=class s{static{s.prototype.isVector4=!0}constructor(e=0,t=0,n=0,i=1){this.x=e,this.y=t,this.z=n,this.w=i}get width(){return this.z}set width(e){this.z=e}get height(){return this.w}set height(e){this.w=e}set(e,t,n,i){return this.x=e,this.y=t,this.z=n,this.w=i,this}setScalar(e){return this.x=e,this.y=e,this.z=e,this.w=e,this}setX(e){return this.x=e,this}setY(e){return this.y=e,this}setZ(e){return this.z=e,this}setW(e){return this.w=e,this}setComponent(e,t){switch(e){case 0:this.x=t;break;case 1:this.y=t;break;case 2:this.z=t;break;case 3:this.w=t;break;default:throw new Error("THREE.Vector4: index is out of range: "+e)}return this}getComponent(e){switch(e){case 0:return this.x;case 1:return this.y;case 2:return this.z;case 3:return this.w;default:throw new Error("THREE.Vector4: index is out of range: "+e)}}clone(){return new this.constructor(this.x,this.y,this.z,this.w)}copy(e){return this.x=e.x,this.y=e.y,this.z=e.z,this.w=e.w!==void 0?e.w:1,this}add(e){return this.x+=e.x,this.y+=e.y,this.z+=e.z,this.w+=e.w,this}addScalar(e){return this.x+=e,this.y+=e,this.z+=e,this.w+=e,this}addVectors(e,t){return this.x=e.x+t.x,this.y=e.y+t.y,this.z=e.z+t.z,this.w=e.w+t.w,this}addScaledVector(e,t){return this.x+=e.x*t,this.y+=e.y*t,this.z+=e.z*t,this.w+=e.w*t,this}sub(e){return this.x-=e.x,this.y-=e.y,this.z-=e.z,this.w-=e.w,this}subScalar(e){return this.x-=e,this.y-=e,this.z-=e,this.w-=e,this}subVectors(e,t){return this.x=e.x-t.x,this.y=e.y-t.y,this.z=e.z-t.z,this.w=e.w-t.w,this}multiply(e){return this.x*=e.x,this.y*=e.y,this.z*=e.z,this.w*=e.w,this}multiplyScalar(e){return this.x*=e,this.y*=e,this.z*=e,this.w*=e,this}applyMatrix4(e){let t=this.x,n=this.y,i=this.z,r=this.w,o=e.elements;return this.x=o[0]*t+o[4]*n+o[8]*i+o[12]*r,this.y=o[1]*t+o[5]*n+o[9]*i+o[13]*r,this.z=o[2]*t+o[6]*n+o[10]*i+o[14]*r,this.w=o[3]*t+o[7]*n+o[11]*i+o[15]*r,this}divide(e){return this.x/=e.x,this.y/=e.y,this.z/=e.z,this.w/=e.w,this}divideScalar(e){return this.multiplyScalar(1/e)}setAxisAngleFromQuaternion(e){this.w=2*Math.acos(e.w);let t=Math.sqrt(1-e.w*e.w);return t<1e-4?(this.x=1,this.y=0,this.z=0):(this.x=e.x/t,this.y=e.y/t,this.z=e.z/t),this}setAxisAngleFromRotationMatrix(e){let t,n,i,r,l=e.elements,c=l[0],h=l[4],u=l[8],d=l[1],f=l[5],g=l[9],y=l[2],m=l[6],p=l[10];if(Math.abs(h-d)<.01&&Math.abs(u-y)<.01&&Math.abs(g-m)<.01){if(Math.abs(h+d)<.1&&Math.abs(u+y)<.1&&Math.abs(g+m)<.1&&Math.abs(c+f+p-3)<.1)return this.set(1,0,0,0),this;t=Math.PI;let S=(c+1)/2,x=(f+1)/2,A=(p+1)/2,T=(h+d)/4,I=(u+y)/4,v=(g+m)/4;return S>x&&S>A?S<.01?(n=0,i=.707106781,r=.707106781):(n=Math.sqrt(S),i=T/n,r=I/n):x>A?x<.01?(n=.707106781,i=0,r=.707106781):(i=Math.sqrt(x),n=T/i,r=v/i):A<.01?(n=.707106781,i=.707106781,r=0):(r=Math.sqrt(A),n=I/r,i=v/r),this.set(n,i,r,t),this}let b=Math.sqrt((m-g)*(m-g)+(u-y)*(u-y)+(d-h)*(d-h));return Math.abs(b)<.001&&(b=1),this.x=(m-g)/b,this.y=(u-y)/b,this.z=(d-h)/b,this.w=Math.acos((c+f+p-1)/2),this}setFromMatrixPosition(e){let t=e.elements;return this.x=t[12],this.y=t[13],this.z=t[14],this.w=t[15],this}min(e){return this.x=Math.min(this.x,e.x),this.y=Math.min(this.y,e.y),this.z=Math.min(this.z,e.z),this.w=Math.min(this.w,e.w),this}max(e){return this.x=Math.max(this.x,e.x),this.y=Math.max(this.y,e.y),this.z=Math.max(this.z,e.z),this.w=Math.max(this.w,e.w),this}clamp(e,t){return this.x=at(this.x,e.x,t.x),this.y=at(this.y,e.y,t.y),this.z=at(this.z,e.z,t.z),this.w=at(this.w,e.w,t.w),this}clampScalar(e,t){return this.x=at(this.x,e,t),this.y=at(this.y,e,t),this.z=at(this.z,e,t),this.w=at(this.w,e,t),this}clampLength(e,t){let n=this.length();return this.divideScalar(n||1).multiplyScalar(at(n,e,t))}floor(){return this.x=Math.floor(this.x),this.y=Math.floor(this.y),this.z=Math.floor(this.z),this.w=Math.floor(this.w),this}ceil(){return this.x=Math.ceil(this.x),this.y=Math.ceil(this.y),this.z=Math.ceil(this.z),this.w=Math.ceil(this.w),this}round(){return this.x=Math.round(this.x),this.y=Math.round(this.y),this.z=Math.round(this.z),this.w=Math.round(this.w),this}roundToZero(){return this.x=Math.trunc(this.x),this.y=Math.trunc(this.y),this.z=Math.trunc(this.z),this.w=Math.trunc(this.w),this}negate(){return this.x=-this.x,this.y=-this.y,this.z=-this.z,this.w=-this.w,this}dot(e){return this.x*e.x+this.y*e.y+this.z*e.z+this.w*e.w}lengthSq(){return this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w}length(){return Math.sqrt(this.x*this.x+this.y*this.y+this.z*this.z+this.w*this.w)}manhattanLength(){return Math.abs(this.x)+Math.abs(this.y)+Math.abs(this.z)+Math.abs(this.w)}normalize(){return this.divideScalar(this.length()||1)}setLength(e){return this.normalize().multiplyScalar(e)}lerp(e,t){return this.x+=(e.x-this.x)*t,this.y+=(e.y-this.y)*t,this.z+=(e.z-this.z)*t,this.w+=(e.w-this.w)*t,this}lerpVectors(e,t,n){return this.x=e.x+(t.x-e.x)*n,this.y=e.y+(t.y-e.y)*n,this.z=e.z+(t.z-e.z)*n,this.w=e.w+(t.w-e.w)*n,this}equals(e){return e.x===this.x&&e.y===this.y&&e.z===this.z&&e.w===this.w}fromArray(e,t=0){return this.x=e[t],this.y=e[t+1],this.z=e[t+2],this.w=e[t+3],this}toArray(e=[],t=0){return e[t]=this.x,e[t+1]=this.y,e[t+2]=this.z,e[t+3]=this.w,e}fromBufferAttribute(e,t){return this.x=e.getX(t),this.y=e.getY(t),this.z=e.getZ(t),this.w=e.getW(t),this}random(){return this.x=Math.random(),this.y=Math.random(),this.z=Math.random(),this.w=Math.random(),this}*[Symbol.iterator](){yield this.x,yield this.y,yield this.z,yield this.w}},za=class extends Vn{constructor(e=1,t=1,n={}){super(),n=Object.assign({generateMipmaps:!1,internalFormat:null,minFilter:kt,depthBuffer:!0,stencilBuffer:!1,resolveDepthBuffer:!0,resolveStencilBuffer:!0,depthTexture:null,samples:0,count:1,depth:1,multiview:!1,useArrayDepthTexture:!1},n),this.isRenderTarget=!0,this.width=e,this.height=t,this.depth=n.depth,this.scissor=new _t(0,0,e,t),this.scissorTest=!1,this.viewport=new _t(0,0,e,t),this.textures=[];let i={width:e,height:t,depth:n.depth},r=new jt(i),o=n.count;for(let a=0;a<o;a++)this.textures[a]=r.clone(),this.textures[a].isRenderTargetTexture=!0,this.textures[a].renderTarget=this;this._setTextureOptions(n),this.depthBuffer=n.depthBuffer,this.stencilBuffer=n.stencilBuffer,this.resolveDepthBuffer=n.resolveDepthBuffer,this.resolveStencilBuffer=n.resolveStencilBuffer,this._depthTexture=null,this.depthTexture=n.depthTexture,this.samples=n.samples,this.multiview=n.multiview,this.useArrayDepthTexture=n.useArrayDepthTexture}_setTextureOptions(e={}){let t={minFilter:kt,generateMipmaps:!1,flipY:!1,internalFormat:null};e.mapping!==void 0&&(t.mapping=e.mapping),e.wrapS!==void 0&&(t.wrapS=e.wrapS),e.wrapT!==void 0&&(t.wrapT=e.wrapT),e.wrapR!==void 0&&(t.wrapR=e.wrapR),e.magFilter!==void 0&&(t.magFilter=e.magFilter),e.minFilter!==void 0&&(t.minFilter=e.minFilter),e.format!==void 0&&(t.format=e.format),e.type!==void 0&&(t.type=e.type),e.anisotropy!==void 0&&(t.anisotropy=e.anisotropy),e.colorSpace!==void 0&&(t.colorSpace=e.colorSpace),e.flipY!==void 0&&(t.flipY=e.flipY),e.generateMipmaps!==void 0&&(t.generateMipmaps=e.generateMipmaps),e.internalFormat!==void 0&&(t.internalFormat=e.internalFormat);for(let n=0;n<this.textures.length;n++)this.textures[n].setValues(t)}get texture(){return this.textures[0]}set texture(e){this.textures[0]=e}set depthTexture(e){this._depthTexture!==null&&(this._depthTexture.renderTarget=null),e!==null&&(e.renderTarget=this),this._depthTexture=e}get depthTexture(){return this._depthTexture}setSize(e,t,n=1){if(this.width!==e||this.height!==t||this.depth!==n){this.width=e,this.height=t,this.depth=n;for(let i=0,r=this.textures.length;i<r;i++)this.textures[i].image.width=e,this.textures[i].image.height=t,this.textures[i].image.depth=n,this.textures[i].isData3DTexture!==!0&&(this.textures[i].isArrayTexture=this.textures[i].image.depth>1);this.dispose()}this.viewport.set(0,0,e,t),this.scissor.set(0,0,e,t)}clone(){return new this.constructor().copy(this)}copy(e){this.width=e.width,this.height=e.height,this.depth=e.depth,this.scissor.copy(e.scissor),this.scissorTest=e.scissorTest,this.viewport.copy(e.viewport),this.textures.length=0;for(let t=0,n=e.textures.length;t<n;t++){this.textures[t]=e.textures[t].clone(),this.textures[t].isRenderTargetTexture=!0,this.textures[t].renderTarget=this;let i=Object.assign({},e.textures[t].image);this.textures[t].source=new ur(i)}return this.depthBuffer=e.depthBuffer,this.stencilBuffer=e.stencilBuffer,this.resolveDepthBuffer=e.resolveDepthBuffer,this.resolveStencilBuffer=e.resolveStencilBuffer,e.depthTexture!==null&&(this.depthTexture=e.depthTexture.clone()),this.samples=e.samples,this.multiview=e.multiview,this.useArrayDepthTexture=e.useArrayDepthTexture,this}dispose(){this.dispatchEvent({type:"dispose"})}},Wt=class extends za{constructor(e=1,t=1,n={}){super(e,t,n),this.isWebGLRenderTarget=!0}},no=class extends jt{constructor(e=null,t=1,n=1,i=1){super(null),this.isDataArrayTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Kt,this.minFilter=Kt,this.wrapR=Hn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1,this.layerUpdates=new Set}addLayerUpdate(e){this.layerUpdates.add(e)}clearLayerUpdates(){this.layerUpdates.clear()}};var ka=class extends jt{constructor(e=null,t=1,n=1,i=1){super(null),this.isData3DTexture=!0,this.image={data:e,width:t,height:n,depth:i},this.magFilter=Kt,this.minFilter=Kt,this.wrapR=Hn,this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}};var Qe=class s{static{s.prototype.isMatrix4=!0}constructor(e,t,n,i,r,o,a,l,c,h,u,d,f,g,y,m){this.elements=[1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1],e!==void 0&&this.set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,y,m)}set(e,t,n,i,r,o,a,l,c,h,u,d,f,g,y,m){let p=this.elements;return p[0]=e,p[4]=t,p[8]=n,p[12]=i,p[1]=r,p[5]=o,p[9]=a,p[13]=l,p[2]=c,p[6]=h,p[10]=u,p[14]=d,p[3]=f,p[7]=g,p[11]=y,p[15]=m,this}identity(){return this.set(1,0,0,0,0,1,0,0,0,0,1,0,0,0,0,1),this}clone(){return new s().fromArray(this.elements)}copy(e){let t=this.elements,n=e.elements;return t[0]=n[0],t[1]=n[1],t[2]=n[2],t[3]=n[3],t[4]=n[4],t[5]=n[5],t[6]=n[6],t[7]=n[7],t[8]=n[8],t[9]=n[9],t[10]=n[10],t[11]=n[11],t[12]=n[12],t[13]=n[13],t[14]=n[14],t[15]=n[15],this}copyPosition(e){let t=this.elements,n=e.elements;return t[12]=n[12],t[13]=n[13],t[14]=n[14],this}setFromMatrix3(e){let t=e.elements;return this.set(t[0],t[3],t[6],0,t[1],t[4],t[7],0,t[2],t[5],t[8],0,0,0,0,1),this}extractBasis(e,t,n){return this.determinantAffine()===0?(e.set(1,0,0),t.set(0,1,0),n.set(0,0,1),this):(e.setFromMatrixColumn(this,0),t.setFromMatrixColumn(this,1),n.setFromMatrixColumn(this,2),this)}makeBasis(e,t,n){return this.set(e.x,t.x,n.x,0,e.y,t.y,n.y,0,e.z,t.z,n.z,0,0,0,0,1),this}extractRotation(e){if(e.determinantAffine()===0)return this.identity();let t=this.elements,n=e.elements,i=1/Ys.setFromMatrixColumn(e,0).length(),r=1/Ys.setFromMatrixColumn(e,1).length(),o=1/Ys.setFromMatrixColumn(e,2).length();return t[0]=n[0]*i,t[1]=n[1]*i,t[2]=n[2]*i,t[3]=0,t[4]=n[4]*r,t[5]=n[5]*r,t[6]=n[6]*r,t[7]=0,t[8]=n[8]*o,t[9]=n[9]*o,t[10]=n[10]*o,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromEuler(e){let t=this.elements,n=e.x,i=e.y,r=e.z,o=Math.cos(n),a=Math.sin(n),l=Math.cos(i),c=Math.sin(i),h=Math.cos(r),u=Math.sin(r);if(e.order==="XYZ"){let d=o*h,f=o*u,g=a*h,y=a*u;t[0]=l*h,t[4]=-l*u,t[8]=c,t[1]=f+g*c,t[5]=d-y*c,t[9]=-a*l,t[2]=y-d*c,t[6]=g+f*c,t[10]=o*l}else if(e.order==="YXZ"){let d=l*h,f=l*u,g=c*h,y=c*u;t[0]=d+y*a,t[4]=g*a-f,t[8]=o*c,t[1]=o*u,t[5]=o*h,t[9]=-a,t[2]=f*a-g,t[6]=y+d*a,t[10]=o*l}else if(e.order==="ZXY"){let d=l*h,f=l*u,g=c*h,y=c*u;t[0]=d-y*a,t[4]=-o*u,t[8]=g+f*a,t[1]=f+g*a,t[5]=o*h,t[9]=y-d*a,t[2]=-o*c,t[6]=a,t[10]=o*l}else if(e.order==="ZYX"){let d=o*h,f=o*u,g=a*h,y=a*u;t[0]=l*h,t[4]=g*c-f,t[8]=d*c+y,t[1]=l*u,t[5]=y*c+d,t[9]=f*c-g,t[2]=-c,t[6]=a*l,t[10]=o*l}else if(e.order==="YZX"){let d=o*l,f=o*c,g=a*l,y=a*c;t[0]=l*h,t[4]=y-d*u,t[8]=g*u+f,t[1]=u,t[5]=o*h,t[9]=-a*h,t[2]=-c*h,t[6]=f*u+g,t[10]=d-y*u}else if(e.order==="XZY"){let d=o*l,f=o*c,g=a*l,y=a*c;t[0]=l*h,t[4]=-u,t[8]=c*h,t[1]=d*u+y,t[5]=o*h,t[9]=f*u-g,t[2]=g*u-f,t[6]=a*h,t[10]=y*u+d}return t[3]=0,t[7]=0,t[11]=0,t[12]=0,t[13]=0,t[14]=0,t[15]=1,this}makeRotationFromQuaternion(e){return this.compose(im,e,sm)}lookAt(e,t,n){let i=this.elements;return Pn.subVectors(e,t),Pn.lengthSq()===0&&(Pn.z=1),Pn.normalize(),qi.crossVectors(n,Pn),qi.lengthSq()===0&&(Math.abs(n.z)===1?Pn.x+=1e-4:Pn.z+=1e-4,Pn.normalize(),qi.crossVectors(n,Pn)),qi.normalize(),ta.crossVectors(Pn,qi),i[0]=qi.x,i[4]=ta.x,i[8]=Pn.x,i[1]=qi.y,i[5]=ta.y,i[9]=Pn.y,i[2]=qi.z,i[6]=ta.z,i[10]=Pn.z,this}multiply(e){return this.multiplyMatrices(this,e)}premultiply(e){return this.multiplyMatrices(e,this)}multiplyMatrices(e,t){let n=e.elements,i=t.elements,r=this.elements,o=n[0],a=n[4],l=n[8],c=n[12],h=n[1],u=n[5],d=n[9],f=n[13],g=n[2],y=n[6],m=n[10],p=n[14],b=n[3],S=n[7],x=n[11],A=n[15],T=i[0],I=i[4],v=i[8],D=i[12],w=i[1],R=i[5],L=i[9],V=i[13],C=i[2],N=i[6],U=i[10],E=i[14],G=i[3],Y=i[7],X=i[11],ie=i[15];return r[0]=o*T+a*w+l*C+c*G,r[4]=o*I+a*R+l*N+c*Y,r[8]=o*v+a*L+l*U+c*X,r[12]=o*D+a*V+l*E+c*ie,r[1]=h*T+u*w+d*C+f*G,r[5]=h*I+u*R+d*N+f*Y,r[9]=h*v+u*L+d*U+f*X,r[13]=h*D+u*V+d*E+f*ie,r[2]=g*T+y*w+m*C+p*G,r[6]=g*I+y*R+m*N+p*Y,r[10]=g*v+y*L+m*U+p*X,r[14]=g*D+y*V+m*E+p*ie,r[3]=b*T+S*w+x*C+A*G,r[7]=b*I+S*R+x*N+A*Y,r[11]=b*v+S*L+x*U+A*X,r[15]=b*D+S*V+x*E+A*ie,this}multiplyScalar(e){let t=this.elements;return t[0]*=e,t[4]*=e,t[8]*=e,t[12]*=e,t[1]*=e,t[5]*=e,t[9]*=e,t[13]*=e,t[2]*=e,t[6]*=e,t[10]*=e,t[14]*=e,t[3]*=e,t[7]*=e,t[11]*=e,t[15]*=e,this}determinant(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[12],o=e[1],a=e[5],l=e[9],c=e[13],h=e[2],u=e[6],d=e[10],f=e[14],g=e[3],y=e[7],m=e[11],p=e[15],b=l*f-c*d,S=a*f-c*u,x=a*d-l*u,A=o*f-c*h,T=o*d-l*h,I=o*u-a*h;return t*(y*b-m*S+p*x)-n*(g*b-m*A+p*T)+i*(g*S-y*A+p*I)-r*(g*x-y*T+m*I)}determinantAffine(){let e=this.elements,t=e[0],n=e[4],i=e[8],r=e[1],o=e[5],a=e[9],l=e[2],c=e[6],h=e[10];return t*(o*h-a*c)-n*(r*h-a*l)+i*(r*c-o*l)}transpose(){let e=this.elements,t;return t=e[1],e[1]=e[4],e[4]=t,t=e[2],e[2]=e[8],e[8]=t,t=e[6],e[6]=e[9],e[9]=t,t=e[3],e[3]=e[12],e[12]=t,t=e[7],e[7]=e[13],e[13]=t,t=e[11],e[11]=e[14],e[14]=t,this}setPosition(e,t,n){let i=this.elements;return e.isVector3?(i[12]=e.x,i[13]=e.y,i[14]=e.z):(i[12]=e,i[13]=t,i[14]=n),this}invert(){let e=this.elements,t=e[0],n=e[1],i=e[2],r=e[3],o=e[4],a=e[5],l=e[6],c=e[7],h=e[8],u=e[9],d=e[10],f=e[11],g=e[12],y=e[13],m=e[14],p=e[15],b=t*a-n*o,S=t*l-i*o,x=t*c-r*o,A=n*l-i*a,T=n*c-r*a,I=i*c-r*l,v=h*y-u*g,D=h*m-d*g,w=h*p-f*g,R=u*m-d*y,L=u*p-f*y,V=d*p-f*m,C=b*V-S*L+x*R+A*w-T*D+I*v;if(C===0)return this.set(0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0);let N=1/C;return e[0]=(a*V-l*L+c*R)*N,e[1]=(i*L-n*V-r*R)*N,e[2]=(y*I-m*T+p*A)*N,e[3]=(d*T-u*I-f*A)*N,e[4]=(l*w-o*V-c*D)*N,e[5]=(t*V-i*w+r*D)*N,e[6]=(m*x-g*I-p*S)*N,e[7]=(h*I-d*x+f*S)*N,e[8]=(o*L-a*w+c*v)*N,e[9]=(n*w-t*L-r*v)*N,e[10]=(g*T-y*x+p*b)*N,e[11]=(u*x-h*T-f*b)*N,e[12]=(a*D-o*R-l*v)*N,e[13]=(t*R-n*D+i*v)*N,e[14]=(y*S-g*A-m*b)*N,e[15]=(h*A-u*S+d*b)*N,this}scale(e){let t=this.elements,n=e.x,i=e.y,r=e.z;return t[0]*=n,t[4]*=i,t[8]*=r,t[1]*=n,t[5]*=i,t[9]*=r,t[2]*=n,t[6]*=i,t[10]*=r,t[3]*=n,t[7]*=i,t[11]*=r,this}getMaxScaleOnAxis(){let e=this.elements,t=e[0]*e[0]+e[1]*e[1]+e[2]*e[2],n=e[4]*e[4]+e[5]*e[5]+e[6]*e[6],i=e[8]*e[8]+e[9]*e[9]+e[10]*e[10];return Math.sqrt(Math.max(t,n,i))}makeTranslation(e,t,n){return e.isVector3?this.set(1,0,0,e.x,0,1,0,e.y,0,0,1,e.z,0,0,0,1):this.set(1,0,0,e,0,1,0,t,0,0,1,n,0,0,0,1),this}makeRotationX(e){let t=Math.cos(e),n=Math.sin(e);return this.set(1,0,0,0,0,t,-n,0,0,n,t,0,0,0,0,1),this}makeRotationY(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,0,n,0,0,1,0,0,-n,0,t,0,0,0,0,1),this}makeRotationZ(e){let t=Math.cos(e),n=Math.sin(e);return this.set(t,-n,0,0,n,t,0,0,0,0,1,0,0,0,0,1),this}makeRotationAxis(e,t){let n=Math.cos(t),i=Math.sin(t),r=1-n,o=e.x,a=e.y,l=e.z,c=r*o,h=r*a;return this.set(c*o+n,c*a-i*l,c*l+i*a,0,c*a+i*l,h*a+n,h*l-i*o,0,c*l-i*a,h*l+i*o,r*l*l+n,0,0,0,0,1),this}makeScale(e,t,n){return this.set(e,0,0,0,0,t,0,0,0,0,n,0,0,0,0,1),this}makeShear(e,t,n,i,r,o){return this.set(1,n,r,0,e,1,o,0,t,i,1,0,0,0,0,1),this}compose(e,t,n){let i=this.elements,r=t._x,o=t._y,a=t._z,l=t._w,c=r+r,h=o+o,u=a+a,d=r*c,f=r*h,g=r*u,y=o*h,m=o*u,p=a*u,b=l*c,S=l*h,x=l*u,A=n.x,T=n.y,I=n.z;return i[0]=(1-(y+p))*A,i[1]=(f+x)*A,i[2]=(g-S)*A,i[3]=0,i[4]=(f-x)*T,i[5]=(1-(d+p))*T,i[6]=(m+b)*T,i[7]=0,i[8]=(g+S)*I,i[9]=(m-b)*I,i[10]=(1-(d+y))*I,i[11]=0,i[12]=e.x,i[13]=e.y,i[14]=e.z,i[15]=1,this}decompose(e,t,n){let i=this.elements;e.x=i[12],e.y=i[13],e.z=i[14];let r=this.determinantAffine();if(r===0)return n.set(1,1,1),t.identity(),this;let o=Ys.set(i[0],i[1],i[2]).length(),a=Ys.set(i[4],i[5],i[6]).length(),l=Ys.set(i[8],i[9],i[10]).length();r<0&&(o=-o),Kn.copy(this);let c=1/o,h=1/a,u=1/l;return Kn.elements[0]*=c,Kn.elements[1]*=c,Kn.elements[2]*=c,Kn.elements[4]*=h,Kn.elements[5]*=h,Kn.elements[6]*=h,Kn.elements[8]*=u,Kn.elements[9]*=u,Kn.elements[10]*=u,t.setFromRotationMatrix(Kn),n.x=o,n.y=a,n.z=l,this}makePerspective(e,t,n,i,r,o,a=Qn,l=!1){let c=this.elements,h=2*r/(t-e),u=2*r/(n-i),d=(t+e)/(t-e),f=(n+i)/(n-i),g,y;if(l)g=r/(o-r),y=o*r/(o-r);else if(a===Qn)g=-(o+r)/(o-r),y=-2*o*r/(o-r);else if(a===lr)g=-o/(o-r),y=-o*r/(o-r);else throw new Error("THREE.Matrix4.makePerspective(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=d,c[12]=0,c[1]=0,c[5]=u,c[9]=f,c[13]=0,c[2]=0,c[6]=0,c[10]=g,c[14]=y,c[3]=0,c[7]=0,c[11]=-1,c[15]=0,this}makeOrthographic(e,t,n,i,r,o,a=Qn,l=!1){let c=this.elements,h=2/(t-e),u=2/(n-i),d=-(t+e)/(t-e),f=-(n+i)/(n-i),g,y;if(l)g=1/(o-r),y=o/(o-r);else if(a===Qn)g=-2/(o-r),y=-(o+r)/(o-r);else if(a===lr)g=-1/(o-r),y=-r/(o-r);else throw new Error("THREE.Matrix4.makeOrthographic(): Invalid coordinate system: "+a);return c[0]=h,c[4]=0,c[8]=0,c[12]=d,c[1]=0,c[5]=u,c[9]=0,c[13]=f,c[2]=0,c[6]=0,c[10]=g,c[14]=y,c[3]=0,c[7]=0,c[11]=0,c[15]=1,this}equals(e){let t=this.elements,n=e.elements;for(let i=0;i<16;i++)if(t[i]!==n[i])return!1;return!0}fromArray(e,t=0){for(let n=0;n<16;n++)this.elements[n]=e[n+t];return this}toArray(e=[],t=0){let n=this.elements;return e[t]=n[0],e[t+1]=n[1],e[t+2]=n[2],e[t+3]=n[3],e[t+4]=n[4],e[t+5]=n[5],e[t+6]=n[6],e[t+7]=n[7],e[t+8]=n[8],e[t+9]=n[9],e[t+10]=n[10],e[t+11]=n[11],e[t+12]=n[12],e[t+13]=n[13],e[t+14]=n[14],e[t+15]=n[15],e}},Ys=new B,Kn=new Qe,im=new B(0,0,0),sm=new B(1,1,1),qi=new B,ta=new B,Pn=new B,Hu=new Qe,Vu=new Zt,Gn=class s{constructor(e=0,t=0,n=0,i=s.DEFAULT_ORDER){this.isEuler=!0,this._x=e,this._y=t,this._z=n,this._order=i}get x(){return this._x}set x(e){this._x=e,this._onChangeCallback()}get y(){return this._y}set y(e){this._y=e,this._onChangeCallback()}get z(){return this._z}set z(e){this._z=e,this._onChangeCallback()}get order(){return this._order}set order(e){this._order=e,this._onChangeCallback()}set(e,t,n,i=this._order){return this._x=e,this._y=t,this._z=n,this._order=i,this._onChangeCallback(),this}clone(){return new this.constructor(this._x,this._y,this._z,this._order)}copy(e){return this._x=e._x,this._y=e._y,this._z=e._z,this._order=e._order,this._onChangeCallback(),this}setFromRotationMatrix(e,t=this._order,n=!0){let i=e.elements,r=i[0],o=i[4],a=i[8],l=i[1],c=i[5],h=i[9],u=i[2],d=i[6],f=i[10];switch(t){case"XYZ":this._y=Math.asin(at(a,-1,1)),Math.abs(a)<.9999999?(this._x=Math.atan2(-h,f),this._z=Math.atan2(-o,r)):(this._x=Math.atan2(d,c),this._z=0);break;case"YXZ":this._x=Math.asin(-at(h,-1,1)),Math.abs(h)<.9999999?(this._y=Math.atan2(a,f),this._z=Math.atan2(l,c)):(this._y=Math.atan2(-u,r),this._z=0);break;case"ZXY":this._x=Math.asin(at(d,-1,1)),Math.abs(d)<.9999999?(this._y=Math.atan2(-u,f),this._z=Math.atan2(-o,c)):(this._y=0,this._z=Math.atan2(l,r));break;case"ZYX":this._y=Math.asin(-at(u,-1,1)),Math.abs(u)<.9999999?(this._x=Math.atan2(d,f),this._z=Math.atan2(l,r)):(this._x=0,this._z=Math.atan2(-o,c));break;case"YZX":this._z=Math.asin(at(l,-1,1)),Math.abs(l)<.9999999?(this._x=Math.atan2(-h,c),this._y=Math.atan2(-u,r)):(this._x=0,this._y=Math.atan2(a,f));break;case"XZY":this._z=Math.asin(-at(o,-1,1)),Math.abs(o)<.9999999?(this._x=Math.atan2(d,c),this._y=Math.atan2(a,r)):(this._x=Math.atan2(-h,f),this._y=0);break;default:We("Euler: .setFromRotationMatrix() encountered an unknown order: "+t)}return this._order=t,n===!0&&this._onChangeCallback(),this}setFromQuaternion(e,t,n){return Hu.makeRotationFromQuaternion(e),this.setFromRotationMatrix(Hu,t,n)}setFromVector3(e,t=this._order){return this.set(e.x,e.y,e.z,t)}reorder(e){return Vu.setFromEuler(this),this.setFromQuaternion(Vu,e)}equals(e){return e._x===this._x&&e._y===this._y&&e._z===this._z&&e._order===this._order}fromArray(e){return this._x=e[0],this._y=e[1],this._z=e[2],e[3]!==void 0&&(this._order=e[3]),this._onChangeCallback(),this}toArray(e=[],t=0){return e[t]=this._x,e[t+1]=this._y,e[t+2]=this._z,e[t+3]=this._order,e}_onChange(e){return this._onChangeCallback=e,this}_onChangeCallback(){}*[Symbol.iterator](){yield this._x,yield this._y,yield this._z,yield this._order}};Gn.DEFAULT_ORDER="XYZ";var dr=class{constructor(){this.mask=1}set(e){this.mask=(1<<e|0)>>>0}enable(e){this.mask|=1<<e|0}enableAll(){this.mask=-1}toggle(e){this.mask^=1<<e|0}disable(e){this.mask&=~(1<<e|0)}disableAll(){this.mask=0}test(e){return(this.mask&e.mask)!==0}isEnabled(e){return(this.mask&(1<<e|0))!==0}},rm=0,Gu=new B,Zs=new Zt,Ti=new Qe,na=new B,Hr=new B,om=new B,am=new Zt,Wu=new B(1,0,0),Xu=new B(0,1,0),qu=new B(0,0,1),Yu={type:"added"},lm={type:"removed"},Ks={type:"childadded",child:null},Cc={type:"childremoved",child:null},St=class s extends Vn{constructor(){super(),this.isObject3D=!0,Object.defineProperty(this,"id",{value:rm++}),this.uuid=ei(),this.name="",this.type="Object3D",this.parent=null,this.children=[],this.up=s.DEFAULT_UP.clone();let e=new B,t=new Gn,n=new Zt,i=new B(1,1,1);function r(){n.setFromEuler(t,!1)}function o(){t.setFromQuaternion(n,void 0,!1)}t._onChange(r),n._onChange(o),Object.defineProperties(this,{position:{configurable:!0,enumerable:!0,value:e},rotation:{configurable:!0,enumerable:!0,value:t},quaternion:{configurable:!0,enumerable:!0,value:n},scale:{configurable:!0,enumerable:!0,value:i},modelViewMatrix:{value:new Qe},normalMatrix:{value:new et}}),this.matrix=new Qe,this.matrixWorld=new Qe,this.matrixAutoUpdate=s.DEFAULT_MATRIX_AUTO_UPDATE,this.matrixWorldAutoUpdate=s.DEFAULT_MATRIX_WORLD_AUTO_UPDATE,this.matrixWorldNeedsUpdate=!1,this.layers=new dr,this.visible=!0,this.castShadow=!1,this.receiveShadow=!1,this.frustumCulled=!0,this.renderOrder=0,this.animations=[],this.customDepthMaterial=void 0,this.customDistanceMaterial=void 0,this.static=!1,this.userData={},this.pivot=null}onBeforeShadow(){}onAfterShadow(){}onBeforeRender(){}onAfterRender(){}applyMatrix4(e){this.matrixAutoUpdate&&this.updateMatrix(),this.matrix.premultiply(e),this.matrix.decompose(this.position,this.quaternion,this.scale)}applyQuaternion(e){return this.quaternion.premultiply(e),this}setRotationFromAxisAngle(e,t){this.quaternion.setFromAxisAngle(e,t)}setRotationFromEuler(e){this.quaternion.setFromEuler(e,!0)}setRotationFromMatrix(e){this.quaternion.setFromRotationMatrix(e)}setRotationFromQuaternion(e){this.quaternion.copy(e)}rotateOnAxis(e,t){return Zs.setFromAxisAngle(e,t),this.quaternion.multiply(Zs),this}rotateOnWorldAxis(e,t){return Zs.setFromAxisAngle(e,t),this.quaternion.premultiply(Zs),this}rotateX(e){return this.rotateOnAxis(Wu,e)}rotateY(e){return this.rotateOnAxis(Xu,e)}rotateZ(e){return this.rotateOnAxis(qu,e)}translateOnAxis(e,t){return Gu.copy(e).applyQuaternion(this.quaternion),this.position.add(Gu.multiplyScalar(t)),this}translateX(e){return this.translateOnAxis(Wu,e)}translateY(e){return this.translateOnAxis(Xu,e)}translateZ(e){return this.translateOnAxis(qu,e)}localToWorld(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(this.matrixWorld)}worldToLocal(e){return this.updateWorldMatrix(!0,!1),e.applyMatrix4(Ti.copy(this.matrixWorld).invert())}lookAt(e,t,n){e.isVector3?na.copy(e):na.set(e,t,n);let i=this.parent;this.updateWorldMatrix(!0,!1),Hr.setFromMatrixPosition(this.matrixWorld),this.isCamera||this.isLight?Ti.lookAt(Hr,na,this.up):Ti.lookAt(na,Hr,this.up),this.quaternion.setFromRotationMatrix(Ti),i&&(Ti.extractRotation(i.matrixWorld),Zs.setFromRotationMatrix(Ti),this.quaternion.premultiply(Zs.invert()))}add(e){if(arguments.length>1){for(let t=0;t<arguments.length;t++)this.add(arguments[t]);return this}return e===this?($e("Object3D.add: object can't be added as a child of itself.",e),this):(e&&e.isObject3D?(e.removeFromParent(),e.parent=this,this.children.push(e),e.dispatchEvent(Yu),Ks.child=e,this.dispatchEvent(Ks),Ks.child=null):$e("Object3D.add: object not an instance of THREE.Object3D.",e),this)}remove(e){if(arguments.length>1){for(let n=0;n<arguments.length;n++)this.remove(arguments[n]);return this}let t=this.children.indexOf(e);return t!==-1&&(e.parent=null,this.children.splice(t,1),e.dispatchEvent(lm),Cc.child=e,this.dispatchEvent(Cc),Cc.child=null),this}removeFromParent(){let e=this.parent;return e!==null&&e.remove(this),this}clear(){return this.remove(...this.children)}attach(e){return this.updateWorldMatrix(!0,!1),Ti.copy(this.matrixWorld).invert(),e.parent!==null&&(e.parent.updateWorldMatrix(!0,!1),Ti.multiply(e.parent.matrixWorld)),e.applyMatrix4(Ti),e.removeFromParent(),e.parent=this,this.children.push(e),e.updateWorldMatrix(!1,!0),e.dispatchEvent(Yu),Ks.child=e,this.dispatchEvent(Ks),Ks.child=null,this}getObjectById(e){return this.getObjectByProperty("id",e)}getObjectByName(e){return this.getObjectByProperty("name",e)}getObjectByProperty(e,t){if(this[e]===t)return this;for(let n=0,i=this.children.length;n<i;n++){let o=this.children[n].getObjectByProperty(e,t);if(o!==void 0)return o}}getObjectsByProperty(e,t,n=[]){this[e]===t&&n.push(this);let i=this.children;for(let r=0,o=i.length;r<o;r++)i[r].getObjectsByProperty(e,t,n);return n}getWorldPosition(e){return this.updateWorldMatrix(!0,!1),e.setFromMatrixPosition(this.matrixWorld)}getWorldQuaternion(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Hr,e,om),e}getWorldScale(e){return this.updateWorldMatrix(!0,!1),this.matrixWorld.decompose(Hr,am,e),e}getWorldDirection(e){this.updateWorldMatrix(!0,!1);let t=this.matrixWorld.elements;return e.set(t[8],t[9],t[10]).normalize()}raycast(){}traverse(e){e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverse(e)}traverseVisible(e){if(this.visible===!1)return;e(this);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].traverseVisible(e)}traverseAncestors(e){let t=this.parent;t!==null&&(e(t),t.traverseAncestors(e))}updateMatrix(){this.matrix.compose(this.position,this.quaternion,this.scale);let e=this.pivot;if(e!==null){let t=e.x,n=e.y,i=e.z,r=this.matrix.elements;r[12]+=t-r[0]*t-r[4]*n-r[8]*i,r[13]+=n-r[1]*t-r[5]*n-r[9]*i,r[14]+=i-r[2]*t-r[6]*n-r[10]*i}this.matrixWorldNeedsUpdate=!0}updateMatrixWorld(e){this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||e)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,e=!0);let t=this.children;for(let n=0,i=t.length;n<i;n++)t[n].updateMatrixWorld(e)}updateWorldMatrix(e,t,n=!1){let i=this.parent;if(e===!0&&i!==null&&i.updateWorldMatrix(!0,!1),this.matrixAutoUpdate&&this.updateMatrix(),(this.matrixWorldNeedsUpdate||n)&&(this.matrixWorldAutoUpdate===!0&&(this.parent===null?this.matrixWorld.copy(this.matrix):this.matrixWorld.multiplyMatrices(this.parent.matrixWorld,this.matrix)),this.matrixWorldNeedsUpdate=!1,n=!0),t===!0){let r=this.children;for(let o=0,a=r.length;o<a;o++)r[o].updateWorldMatrix(!1,!0,n)}}toJSON(e){let t=e===void 0||typeof e=="string",n={};t&&(e={geometries:{},materials:{},textures:{},images:{},shapes:{},skeletons:{},animations:{},nodes:{}},n.metadata={version:4.7,type:"Object",generator:"Object3D.toJSON"});let i={};i.uuid=this.uuid,i.type=this.type,this.name!==""&&(i.name=this.name),this.castShadow===!0&&(i.castShadow=!0),this.receiveShadow===!0&&(i.receiveShadow=!0),this.visible===!1&&(i.visible=!1),this.frustumCulled===!1&&(i.frustumCulled=!1),this.renderOrder!==0&&(i.renderOrder=this.renderOrder),this.static!==!1&&(i.static=this.static),Object.keys(this.userData).length>0&&(i.userData=this.userData),i.layers=this.layers.mask,i.matrix=this.matrix.toArray(),i.up=this.up.toArray(),this.pivot!==null&&(i.pivot=this.pivot.toArray()),this.matrixAutoUpdate===!1&&(i.matrixAutoUpdate=!1),this.morphTargetDictionary!==void 0&&(i.morphTargetDictionary=Object.assign({},this.morphTargetDictionary)),this.morphTargetInfluences!==void 0&&(i.morphTargetInfluences=this.morphTargetInfluences.slice()),this.isInstancedMesh&&(i.type="InstancedMesh",i.count=this.count,i.instanceMatrix=this.instanceMatrix.toJSON(),this.instanceColor!==null&&(i.instanceColor=this.instanceColor.toJSON())),this.isBatchedMesh&&(i.type="BatchedMesh",i.perObjectFrustumCulled=this.perObjectFrustumCulled,i.sortObjects=this.sortObjects,i.drawRanges=this._drawRanges,i.reservedRanges=this._reservedRanges,i.geometryInfo=this._geometryInfo.map(a=>({...a,boundingBox:a.boundingBox?a.boundingBox.toJSON():void 0,boundingSphere:a.boundingSphere?a.boundingSphere.toJSON():void 0})),i.instanceInfo=this._instanceInfo.map(a=>({...a})),i.availableInstanceIds=this._availableInstanceIds.slice(),i.availableGeometryIds=this._availableGeometryIds.slice(),i.nextIndexStart=this._nextIndexStart,i.nextVertexStart=this._nextVertexStart,i.geometryCount=this._geometryCount,i.maxInstanceCount=this._maxInstanceCount,i.maxVertexCount=this._maxVertexCount,i.maxIndexCount=this._maxIndexCount,i.geometryInitialized=this._geometryInitialized,i.matricesTexture=this._matricesTexture.toJSON(e),i.indirectTexture=this._indirectTexture.toJSON(e),this._colorsTexture!==null&&(i.colorsTexture=this._colorsTexture.toJSON(e)),this.boundingSphere!==null&&(i.boundingSphere=this.boundingSphere.toJSON()),this.boundingBox!==null&&(i.boundingBox=this.boundingBox.toJSON()));function r(a,l){return a[l.uuid]===void 0&&(a[l.uuid]=l.toJSON(e)),l.uuid}if(this.isScene)this.background&&(this.background.isColor?i.background=this.background.toJSON():this.background.isTexture&&(i.background=this.background.toJSON(e).uuid)),this.environment&&this.environment.isTexture&&this.environment.isRenderTargetTexture!==!0&&(i.environment=this.environment.toJSON(e).uuid);else if(this.isMesh||this.isLine||this.isPoints){i.geometry=r(e.geometries,this.geometry);let a=this.geometry.parameters;if(a!==void 0&&a.shapes!==void 0){let l=a.shapes;if(Array.isArray(l))for(let c=0,h=l.length;c<h;c++){let u=l[c];r(e.shapes,u)}else r(e.shapes,l)}}if(this.isSkinnedMesh&&(i.bindMode=this.bindMode,i.bindMatrix=this.bindMatrix.toArray(),this.skeleton!==void 0&&(r(e.skeletons,this.skeleton),i.skeleton=this.skeleton.uuid)),this.material!==void 0)if(Array.isArray(this.material)){let a=[];for(let l=0,c=this.material.length;l<c;l++)a.push(r(e.materials,this.material[l]));i.material=a}else i.material=r(e.materials,this.material);if(this.children.length>0){i.children=[];for(let a=0;a<this.children.length;a++)i.children.push(this.children[a].toJSON(e).object)}if(this.animations.length>0){i.animations=[];for(let a=0;a<this.animations.length;a++){let l=this.animations[a];i.animations.push(r(e.animations,l))}}if(t){let a=o(e.geometries),l=o(e.materials),c=o(e.textures),h=o(e.images),u=o(e.shapes),d=o(e.skeletons),f=o(e.animations),g=o(e.nodes);a.length>0&&(n.geometries=a),l.length>0&&(n.materials=l),c.length>0&&(n.textures=c),h.length>0&&(n.images=h),u.length>0&&(n.shapes=u),d.length>0&&(n.skeletons=d),f.length>0&&(n.animations=f),g.length>0&&(n.nodes=g)}return n.object=i,n;function o(a){let l=[];for(let c in a){let h=a[c];delete h.metadata,l.push(h)}return l}}clone(e){return new this.constructor().copy(this,e)}copy(e,t=!0){if(this.name=e.name,this.up.copy(e.up),this.position.copy(e.position),this.rotation.order=e.rotation.order,this.quaternion.copy(e.quaternion),this.scale.copy(e.scale),this.pivot=e.pivot!==null?e.pivot.clone():null,this.matrix.copy(e.matrix),this.matrixWorld.copy(e.matrixWorld),this.matrixAutoUpdate=e.matrixAutoUpdate,this.matrixWorldAutoUpdate=e.matrixWorldAutoUpdate,this.matrixWorldNeedsUpdate=e.matrixWorldNeedsUpdate,this.layers.mask=e.layers.mask,this.visible=e.visible,this.castShadow=e.castShadow,this.receiveShadow=e.receiveShadow,this.frustumCulled=e.frustumCulled,this.renderOrder=e.renderOrder,this.static=e.static,this.animations=e.animations.slice(),this.userData=JSON.parse(JSON.stringify(e.userData)),t===!0)for(let n=0;n<e.children.length;n++){let i=e.children[n];this.add(i.clone())}return this}};St.DEFAULT_UP=new B(0,1,0);St.DEFAULT_MATRIX_AUTO_UPDATE=!0;St.DEFAULT_MATRIX_WORLD_AUTO_UPDATE=!0;var ct=class extends St{constructor(){super(),this.isGroup=!0,this.type="Group"}},cm={type:"move"},fr=class{constructor(){this._targetRay=null,this._grip=null,this._hand=null}getHandSpace(){return this._hand===null&&(this._hand=new ct,this._hand.matrixAutoUpdate=!1,this._hand.visible=!1,this._hand.joints={},this._hand.inputState={pinching:!1}),this._hand}getTargetRaySpace(){return this._targetRay===null&&(this._targetRay=new ct,this._targetRay.matrixAutoUpdate=!1,this._targetRay.visible=!1,this._targetRay.hasLinearVelocity=!1,this._targetRay.linearVelocity=new B,this._targetRay.hasAngularVelocity=!1,this._targetRay.angularVelocity=new B),this._targetRay}getGripSpace(){return this._grip===null&&(this._grip=new ct,this._grip.matrixAutoUpdate=!1,this._grip.visible=!1,this._grip.hasLinearVelocity=!1,this._grip.linearVelocity=new B,this._grip.hasAngularVelocity=!1,this._grip.angularVelocity=new B,this._grip.eventsEnabled=!1),this._grip}dispatchEvent(e){return this._targetRay!==null&&this._targetRay.dispatchEvent(e),this._grip!==null&&this._grip.dispatchEvent(e),this._hand!==null&&this._hand.dispatchEvent(e),this}connect(e){if(e&&e.hand){let t=this._hand;if(t)for(let n of e.hand.values())this._getHandJoint(t,n)}return this.dispatchEvent({type:"connected",data:e}),this}disconnect(e){return this.dispatchEvent({type:"disconnected",data:e}),this._targetRay!==null&&(this._targetRay.visible=!1),this._grip!==null&&(this._grip.visible=!1),this._hand!==null&&(this._hand.visible=!1),this}update(e,t,n){let i=null,r=null,o=null,a=this._targetRay,l=this._grip,c=this._hand;if(e&&t.session.visibilityState!=="visible-blurred"){if(c&&e.hand){o=!0;for(let y of e.hand.values()){let m=t.getJointPose(y,n),p=this._getHandJoint(c,y);m!==null&&(p.matrix.fromArray(m.transform.matrix),p.matrix.decompose(p.position,p.rotation,p.scale),p.matrixWorldNeedsUpdate=!0,p.jointRadius=m.radius),p.visible=m!==null}let h=c.joints["index-finger-tip"],u=c.joints["thumb-tip"],d=h.position.distanceTo(u.position),f=.02,g=.005;c.inputState.pinching&&d>f+g?(c.inputState.pinching=!1,this.dispatchEvent({type:"pinchend",handedness:e.handedness,target:this})):!c.inputState.pinching&&d<=f-g&&(c.inputState.pinching=!0,this.dispatchEvent({type:"pinchstart",handedness:e.handedness,target:this}))}else l!==null&&e.gripSpace&&(r=t.getPose(e.gripSpace,n),r!==null&&(l.matrix.fromArray(r.transform.matrix),l.matrix.decompose(l.position,l.rotation,l.scale),l.matrixWorldNeedsUpdate=!0,r.linearVelocity?(l.hasLinearVelocity=!0,l.linearVelocity.copy(r.linearVelocity)):l.hasLinearVelocity=!1,r.angularVelocity?(l.hasAngularVelocity=!0,l.angularVelocity.copy(r.angularVelocity)):l.hasAngularVelocity=!1,l.eventsEnabled&&l.dispatchEvent({type:"gripUpdated",data:e,target:this})));a!==null&&(i=t.getPose(e.targetRaySpace,n),i===null&&r!==null&&(i=r),i!==null&&(a.matrix.fromArray(i.transform.matrix),a.matrix.decompose(a.position,a.rotation,a.scale),a.matrixWorldNeedsUpdate=!0,i.linearVelocity?(a.hasLinearVelocity=!0,a.linearVelocity.copy(i.linearVelocity)):a.hasLinearVelocity=!1,i.angularVelocity?(a.hasAngularVelocity=!0,a.angularVelocity.copy(i.angularVelocity)):a.hasAngularVelocity=!1,this.dispatchEvent(cm)))}return a!==null&&(a.visible=i!==null),l!==null&&(l.visible=r!==null),c!==null&&(c.visible=o!==null),this}_getHandJoint(e,t){if(e.joints[t.jointName]===void 0){let n=new ct;n.matrixAutoUpdate=!1,n.visible=!1,e.joints[t.jointName]=n,e.add(n)}return e.joints[t.jointName]}},cf={aliceblue:15792383,antiquewhite:16444375,aqua:65535,aquamarine:8388564,azure:15794175,beige:16119260,bisque:16770244,black:0,blanchedalmond:16772045,blue:255,blueviolet:9055202,brown:10824234,burlywood:14596231,cadetblue:6266528,chartreuse:8388352,chocolate:13789470,coral:16744272,cornflowerblue:6591981,cornsilk:16775388,crimson:14423100,cyan:65535,darkblue:139,darkcyan:35723,darkgoldenrod:12092939,darkgray:11119017,darkgreen:25600,darkgrey:11119017,darkkhaki:12433259,darkmagenta:9109643,darkolivegreen:5597999,darkorange:16747520,darkorchid:10040012,darkred:9109504,darksalmon:15308410,darkseagreen:9419919,darkslateblue:4734347,darkslategray:3100495,darkslategrey:3100495,darkturquoise:52945,darkviolet:9699539,deeppink:16716947,deepskyblue:49151,dimgray:6908265,dimgrey:6908265,dodgerblue:2003199,firebrick:11674146,floralwhite:16775920,forestgreen:2263842,fuchsia:16711935,gainsboro:14474460,ghostwhite:16316671,gold:16766720,goldenrod:14329120,gray:8421504,green:32768,greenyellow:11403055,grey:8421504,honeydew:15794160,hotpink:16738740,indianred:13458524,indigo:4915330,ivory:16777200,khaki:15787660,lavender:15132410,lavenderblush:16773365,lawngreen:8190976,lemonchiffon:16775885,lightblue:11393254,lightcoral:15761536,lightcyan:14745599,lightgoldenrodyellow:16448210,lightgray:13882323,lightgreen:9498256,lightgrey:13882323,lightpink:16758465,lightsalmon:16752762,lightseagreen:2142890,lightskyblue:8900346,lightslategray:7833753,lightslategrey:7833753,lightsteelblue:11584734,lightyellow:16777184,lime:65280,limegreen:3329330,linen:16445670,magenta:16711935,maroon:8388608,mediumaquamarine:6737322,mediumblue:205,mediumorchid:12211667,mediumpurple:9662683,mediumseagreen:3978097,mediumslateblue:8087790,mediumspringgreen:64154,mediumturquoise:4772300,mediumvioletred:13047173,midnightblue:1644912,mintcream:16121850,mistyrose:16770273,moccasin:16770229,navajowhite:16768685,navy:128,oldlace:16643558,olive:8421376,olivedrab:7048739,orange:16753920,orangered:16729344,orchid:14315734,palegoldenrod:15657130,palegreen:10025880,paleturquoise:11529966,palevioletred:14381203,papayawhip:16773077,peachpuff:16767673,peru:13468991,pink:16761035,plum:14524637,powderblue:11591910,purple:8388736,rebeccapurple:6697881,red:16711680,rosybrown:12357519,royalblue:4286945,saddlebrown:9127187,salmon:16416882,sandybrown:16032864,seagreen:3050327,seashell:16774638,sienna:10506797,silver:12632256,skyblue:8900331,slateblue:6970061,slategray:7372944,slategrey:7372944,snow:16775930,springgreen:65407,steelblue:4620980,tan:13808780,teal:32896,thistle:14204888,tomato:16737095,turquoise:4251856,violet:15631086,wheat:16113331,white:16777215,whitesmoke:16119285,yellow:16776960,yellowgreen:10145074},Yi={h:0,s:0,l:0},ia={h:0,s:0,l:0};function Pc(s,e,t){return t<0&&(t+=1),t>1&&(t-=1),t<1/6?s+(e-s)*6*t:t<1/2?e:t<2/3?s+(e-s)*6*(2/3-t):s}var xe=class{constructor(e,t,n){return this.isColor=!0,this.r=1,this.g=1,this.b=1,this.set(e,t,n)}set(e,t,n){if(t===void 0&&n===void 0){let i=e;i&&i.isColor?this.copy(i):typeof i=="number"?this.setHex(i):typeof i=="string"&&this.setStyle(i)}else this.setRGB(e,t,n);return this}setScalar(e){return this.r=e,this.g=e,this.b=e,this}setHex(e,t=zt){return e=Math.floor(e),this.r=(e>>16&255)/255,this.g=(e>>8&255)/255,this.b=(e&255)/255,ot.colorSpaceToWorking(this,t),this}setRGB(e,t,n,i=ot.workingColorSpace){return this.r=e,this.g=t,this.b=n,ot.colorSpaceToWorking(this,i),this}setHSL(e,t,n,i=ot.workingColorSpace){if(e=wh(e,1),t=at(t,0,1),n=at(n,0,1),t===0)this.r=this.g=this.b=n;else{let r=n<=.5?n*(1+t):n+t-n*t,o=2*n-r;this.r=Pc(o,r,e+1/3),this.g=Pc(o,r,e),this.b=Pc(o,r,e-1/3)}return ot.colorSpaceToWorking(this,i),this}setStyle(e,t=zt){function n(r){r!==void 0&&parseFloat(r)<1&&We("Color: Alpha component of "+e+" will be ignored.")}let i;if(i=/^(\w+)\(([^\)]*)\)/.exec(e)){let r,o=i[1],a=i[2];switch(o){case"rgb":case"rgba":if(r=/^\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(255,parseInt(r[1],10))/255,Math.min(255,parseInt(r[2],10))/255,Math.min(255,parseInt(r[3],10))/255,t);if(r=/^\s*(\d+)\%\s*,\s*(\d+)\%\s*,\s*(\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setRGB(Math.min(100,parseInt(r[1],10))/100,Math.min(100,parseInt(r[2],10))/100,Math.min(100,parseInt(r[3],10))/100,t);break;case"hsl":case"hsla":if(r=/^\s*(\d*\.?\d+)\s*,\s*(\d*\.?\d+)\%\s*,\s*(\d*\.?\d+)\%\s*(?:,\s*(\d*\.?\d+)\s*)?$/.exec(a))return n(r[4]),this.setHSL(parseFloat(r[1])/360,parseFloat(r[2])/100,parseFloat(r[3])/100,t);break;default:We("Color: Unknown color model "+e)}}else if(i=/^\#([A-Fa-f\d]+)$/.exec(e)){let r=i[1],o=r.length;if(o===3)return this.setRGB(parseInt(r.charAt(0),16)/15,parseInt(r.charAt(1),16)/15,parseInt(r.charAt(2),16)/15,t);if(o===6)return this.setHex(parseInt(r,16),t);We("Color: Invalid hex color "+e)}else if(e&&e.length>0)return this.setColorName(e,t);return this}setColorName(e,t=zt){let n=cf[e.toLowerCase()];return n!==void 0?this.setHex(n,t):We("Color: Unknown color "+e),this}clone(){return new this.constructor(this.r,this.g,this.b)}copy(e){return this.r=e.r,this.g=e.g,this.b=e.b,this}copySRGBToLinear(e){return this.r=Li(e.r),this.g=Li(e.g),this.b=Li(e.b),this}copyLinearToSRGB(e){return this.r=or(e.r),this.g=or(e.g),this.b=or(e.b),this}convertSRGBToLinear(){return this.copySRGBToLinear(this),this}convertLinearToSRGB(){return this.copyLinearToSRGB(this),this}getHex(e=zt){return ot.workingToColorSpace(un.copy(this),e),Math.round(at(un.r*255,0,255))*65536+Math.round(at(un.g*255,0,255))*256+Math.round(at(un.b*255,0,255))}getHexString(e=zt){return("000000"+this.getHex(e).toString(16)).slice(-6)}getHSL(e,t=ot.workingColorSpace){ot.workingToColorSpace(un.copy(this),t);let n=un.r,i=un.g,r=un.b,o=Math.max(n,i,r),a=Math.min(n,i,r),l,c,h=(a+o)/2;if(a===o)l=0,c=0;else{let u=o-a;switch(c=h<=.5?u/(o+a):u/(2-o-a),o){case n:l=(i-r)/u+(i<r?6:0);break;case i:l=(r-n)/u+2;break;case r:l=(n-i)/u+4;break}l/=6}return e.h=l,e.s=c,e.l=h,e}getRGB(e,t=ot.workingColorSpace){return ot.workingToColorSpace(un.copy(this),t),e.r=un.r,e.g=un.g,e.b=un.b,e}getStyle(e=zt){ot.workingToColorSpace(un.copy(this),e);let t=un.r,n=un.g,i=un.b;return e!==zt?`color(${e} ${t.toFixed(3)} ${n.toFixed(3)} ${i.toFixed(3)})`:`rgb(${Math.round(t*255)},${Math.round(n*255)},${Math.round(i*255)})`}offsetHSL(e,t,n){return this.getHSL(Yi),this.setHSL(Yi.h+e,Yi.s+t,Yi.l+n)}add(e){return this.r+=e.r,this.g+=e.g,this.b+=e.b,this}addColors(e,t){return this.r=e.r+t.r,this.g=e.g+t.g,this.b=e.b+t.b,this}addScalar(e){return this.r+=e,this.g+=e,this.b+=e,this}sub(e){return this.r=Math.max(0,this.r-e.r),this.g=Math.max(0,this.g-e.g),this.b=Math.max(0,this.b-e.b),this}multiply(e){return this.r*=e.r,this.g*=e.g,this.b*=e.b,this}multiplyScalar(e){return this.r*=e,this.g*=e,this.b*=e,this}lerp(e,t){return this.r+=(e.r-this.r)*t,this.g+=(e.g-this.g)*t,this.b+=(e.b-this.b)*t,this}lerpColors(e,t,n){return this.r=e.r+(t.r-e.r)*n,this.g=e.g+(t.g-e.g)*n,this.b=e.b+(t.b-e.b)*n,this}lerpHSL(e,t){this.getHSL(Yi),e.getHSL(ia);let n=jr(Yi.h,ia.h,t),i=jr(Yi.s,ia.s,t),r=jr(Yi.l,ia.l,t);return this.setHSL(n,i,r),this}setFromVector3(e){return this.r=e.x,this.g=e.y,this.b=e.z,this}applyMatrix3(e){let t=this.r,n=this.g,i=this.b,r=e.elements;return this.r=r[0]*t+r[3]*n+r[6]*i,this.g=r[1]*t+r[4]*n+r[7]*i,this.b=r[2]*t+r[5]*n+r[8]*i,this}equals(e){return e.r===this.r&&e.g===this.g&&e.b===this.b}fromArray(e,t=0){return this.r=e[t],this.g=e[t+1],this.b=e[t+2],this}toArray(e=[],t=0){return e[t]=this.r,e[t+1]=this.g,e[t+2]=this.b,e}fromBufferAttribute(e,t){return this.r=e.getX(t),this.g=e.getY(t),this.b=e.getZ(t),this}toJSON(){return this.getHex()}*[Symbol.iterator](){yield this.r,yield this.g,yield this.b}},un=new xe;xe.NAMES=cf;var io=class s{constructor(e,t=25e-5){this.isFogExp2=!0,this.name="",this.color=new xe(e),this.density=t}clone(){return new s(this.color,this.density)}toJSON(){return{type:"FogExp2",name:this.name,color:this.color.getHex(),density:this.density}}};var Rs=class extends St{constructor(){super(),this.isScene=!0,this.type="Scene",this.background=null,this.environment=null,this.fog=null,this.backgroundBlurriness=0,this.backgroundIntensity=1,this.backgroundRotation=new Gn,this.environmentIntensity=1,this.environmentRotation=new Gn,this.overrideMaterial=null,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}copy(e,t){return super.copy(e,t),e.background!==null&&(this.background=e.background.clone()),e.environment!==null&&(this.environment=e.environment.clone()),e.fog!==null&&(this.fog=e.fog.clone()),this.backgroundBlurriness=e.backgroundBlurriness,this.backgroundIntensity=e.backgroundIntensity,this.backgroundRotation.copy(e.backgroundRotation),this.environmentIntensity=e.environmentIntensity,this.environmentRotation.copy(e.environmentRotation),e.overrideMaterial!==null&&(this.overrideMaterial=e.overrideMaterial.clone()),this.matrixAutoUpdate=e.matrixAutoUpdate,this}toJSON(e){let t=super.toJSON(e);return this.fog!==null&&(t.object.fog=this.fog.toJSON()),this.backgroundBlurriness>0&&(t.object.backgroundBlurriness=this.backgroundBlurriness),this.backgroundIntensity!==1&&(t.object.backgroundIntensity=this.backgroundIntensity),t.object.backgroundRotation=this.backgroundRotation.toArray(),this.environmentIntensity!==1&&(t.object.environmentIntensity=this.environmentIntensity),t.object.environmentRotation=this.environmentRotation.toArray(),t}},jn=new B,Ai=new B,Ic=new B,Ri=new B,js=new B,Js=new B,Zu=new B,Lc=new B,Dc=new B,Nc=new B,Uc=new _t,Fc=new _t,Oc=new _t,$i=class s{constructor(e=new B,t=new B,n=new B){this.a=e,this.b=t,this.c=n}static getNormal(e,t,n,i){i.subVectors(n,t),jn.subVectors(e,t),i.cross(jn);let r=i.lengthSq();return r>0?i.multiplyScalar(1/Math.sqrt(r)):i.set(0,0,0)}static getBarycoord(e,t,n,i,r){jn.subVectors(i,t),Ai.subVectors(n,t),Ic.subVectors(e,t);let o=jn.dot(jn),a=jn.dot(Ai),l=jn.dot(Ic),c=Ai.dot(Ai),h=Ai.dot(Ic),u=o*c-a*a;if(u===0)return r.set(0,0,0),null;let d=1/u,f=(c*l-a*h)*d,g=(o*h-a*l)*d;return r.set(1-f-g,g,f)}static containsPoint(e,t,n,i){return this.getBarycoord(e,t,n,i,Ri)===null?!1:Ri.x>=0&&Ri.y>=0&&Ri.x+Ri.y<=1}static getInterpolation(e,t,n,i,r,o,a,l){return this.getBarycoord(e,t,n,i,Ri)===null?(l.x=0,l.y=0,"z"in l&&(l.z=0),"w"in l&&(l.w=0),null):(l.setScalar(0),l.addScaledVector(r,Ri.x),l.addScaledVector(o,Ri.y),l.addScaledVector(a,Ri.z),l)}static getInterpolatedAttribute(e,t,n,i,r,o){return Uc.setScalar(0),Fc.setScalar(0),Oc.setScalar(0),Uc.fromBufferAttribute(e,t),Fc.fromBufferAttribute(e,n),Oc.fromBufferAttribute(e,i),o.setScalar(0),o.addScaledVector(Uc,r.x),o.addScaledVector(Fc,r.y),o.addScaledVector(Oc,r.z),o}static isFrontFacing(e,t,n,i){return jn.subVectors(n,t),Ai.subVectors(e,t),jn.cross(Ai).dot(i)<0}set(e,t,n){return this.a.copy(e),this.b.copy(t),this.c.copy(n),this}setFromPointsAndIndices(e,t,n,i){return this.a.copy(e[t]),this.b.copy(e[n]),this.c.copy(e[i]),this}setFromAttributeAndIndices(e,t,n,i){return this.a.fromBufferAttribute(e,t),this.b.fromBufferAttribute(e,n),this.c.fromBufferAttribute(e,i),this}clone(){return new this.constructor().copy(this)}copy(e){return this.a.copy(e.a),this.b.copy(e.b),this.c.copy(e.c),this}getArea(){return jn.subVectors(this.c,this.b),Ai.subVectors(this.a,this.b),jn.cross(Ai).length()*.5}getMidpoint(e){return e.addVectors(this.a,this.b).add(this.c).multiplyScalar(1/3)}getNormal(e){return s.getNormal(this.a,this.b,this.c,e)}getPlane(e){return e.setFromCoplanarPoints(this.a,this.b,this.c)}getBarycoord(e,t){return s.getBarycoord(e,this.a,this.b,this.c,t)}getInterpolation(e,t,n,i,r){return s.getInterpolation(e,this.a,this.b,this.c,t,n,i,r)}containsPoint(e){return s.containsPoint(e,this.a,this.b,this.c)}isFrontFacing(e){return s.isFrontFacing(this.a,this.b,this.c,e)}intersectsBox(e){return e.intersectsTriangle(this)}closestPointToPoint(e,t){let n=this.a,i=this.b,r=this.c,o,a;js.subVectors(i,n),Js.subVectors(r,n),Lc.subVectors(e,n);let l=js.dot(Lc),c=Js.dot(Lc);if(l<=0&&c<=0)return t.copy(n);Dc.subVectors(e,i);let h=js.dot(Dc),u=Js.dot(Dc);if(h>=0&&u<=h)return t.copy(i);let d=l*u-h*c;if(d<=0&&l>=0&&h<=0)return o=l/(l-h),t.copy(n).addScaledVector(js,o);Nc.subVectors(e,r);let f=js.dot(Nc),g=Js.dot(Nc);if(g>=0&&f<=g)return t.copy(r);let y=f*c-l*g;if(y<=0&&c>=0&&g<=0)return a=c/(c-g),t.copy(n).addScaledVector(Js,a);let m=h*g-f*u;if(m<=0&&u-h>=0&&f-g>=0)return Zu.subVectors(r,i),a=(u-h)/(u-h+(f-g)),t.copy(i).addScaledVector(Zu,a);let p=1/(m+y+d);return o=y*p,a=d*p,t.copy(n).addScaledVector(js,o).addScaledVector(Js,a)}equals(e){return e.a.equals(this.a)&&e.b.equals(this.b)&&e.c.equals(this.c)}},en=class{constructor(e=new B(1/0,1/0,1/0),t=new B(-1/0,-1/0,-1/0)){this.isBox3=!0,this.min=e,this.max=t}set(e,t){return this.min.copy(e),this.max.copy(t),this}setFromArray(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t+=3)this.expandByPoint(Jn.fromArray(e,t));return this}setFromBufferAttribute(e){this.makeEmpty();for(let t=0,n=e.count;t<n;t++)this.expandByPoint(Jn.fromBufferAttribute(e,t));return this}setFromPoints(e){this.makeEmpty();for(let t=0,n=e.length;t<n;t++)this.expandByPoint(e[t]);return this}setFromCenterAndSize(e,t){let n=Jn.copy(t).multiplyScalar(.5);return this.min.copy(e).sub(n),this.max.copy(e).add(n),this}setFromObject(e,t=!1){return this.makeEmpty(),this.expandByObject(e,t)}clone(){return new this.constructor().copy(this)}copy(e){return this.min.copy(e.min),this.max.copy(e.max),this}makeEmpty(){return this.min.x=this.min.y=this.min.z=1/0,this.max.x=this.max.y=this.max.z=-1/0,this}isEmpty(){return this.max.x<this.min.x||this.max.y<this.min.y||this.max.z<this.min.z}getCenter(e){return this.isEmpty()?e.set(0,0,0):e.addVectors(this.min,this.max).multiplyScalar(.5)}getSize(e){return this.isEmpty()?e.set(0,0,0):e.subVectors(this.max,this.min)}expandByPoint(e){return this.min.min(e),this.max.max(e),this}expandByVector(e){return this.min.sub(e),this.max.add(e),this}expandByScalar(e){return this.min.addScalar(-e),this.max.addScalar(e),this}expandByObject(e,t=!1){e.updateWorldMatrix(!1,!1);let n=e.geometry;if(n!==void 0){let r=n.getAttribute("position");if(t===!0&&r!==void 0&&e.isInstancedMesh!==!0)for(let o=0,a=r.count;o<a;o++)e.isMesh===!0?e.getVertexPosition(o,Jn):Jn.fromBufferAttribute(r,o),Jn.applyMatrix4(e.matrixWorld),this.expandByPoint(Jn);else e.boundingBox!==void 0?(e.boundingBox===null&&e.computeBoundingBox(),sa.copy(e.boundingBox)):(n.boundingBox===null&&n.computeBoundingBox(),sa.copy(n.boundingBox)),sa.applyMatrix4(e.matrixWorld),this.union(sa)}let i=e.children;for(let r=0,o=i.length;r<o;r++)this.expandByObject(i[r],t);return this}containsPoint(e){return e.x>=this.min.x&&e.x<=this.max.x&&e.y>=this.min.y&&e.y<=this.max.y&&e.z>=this.min.z&&e.z<=this.max.z}containsBox(e){return this.min.x<=e.min.x&&e.max.x<=this.max.x&&this.min.y<=e.min.y&&e.max.y<=this.max.y&&this.min.z<=e.min.z&&e.max.z<=this.max.z}getParameter(e,t){return t.set((e.x-this.min.x)/(this.max.x-this.min.x),(e.y-this.min.y)/(this.max.y-this.min.y),(e.z-this.min.z)/(this.max.z-this.min.z))}intersectsBox(e){return e.max.x>=this.min.x&&e.min.x<=this.max.x&&e.max.y>=this.min.y&&e.min.y<=this.max.y&&e.max.z>=this.min.z&&e.min.z<=this.max.z}intersectsSphere(e){return this.clampPoint(e.center,Jn),Jn.distanceToSquared(e.center)<=e.radius*e.radius}intersectsPlane(e){let t,n;return e.normal.x>0?(t=e.normal.x*this.min.x,n=e.normal.x*this.max.x):(t=e.normal.x*this.max.x,n=e.normal.x*this.min.x),e.normal.y>0?(t+=e.normal.y*this.min.y,n+=e.normal.y*this.max.y):(t+=e.normal.y*this.max.y,n+=e.normal.y*this.min.y),e.normal.z>0?(t+=e.normal.z*this.min.z,n+=e.normal.z*this.max.z):(t+=e.normal.z*this.max.z,n+=e.normal.z*this.min.z),t<=-e.constant&&n>=-e.constant}intersectsTriangle(e){if(this.isEmpty())return!1;this.getCenter(Vr),ra.subVectors(this.max,Vr),$s.subVectors(e.a,Vr),Qs.subVectors(e.b,Vr),er.subVectors(e.c,Vr),Zi.subVectors(Qs,$s),Ki.subVectors(er,Qs),ms.subVectors($s,er);let t=[0,-Zi.z,Zi.y,0,-Ki.z,Ki.y,0,-ms.z,ms.y,Zi.z,0,-Zi.x,Ki.z,0,-Ki.x,ms.z,0,-ms.x,-Zi.y,Zi.x,0,-Ki.y,Ki.x,0,-ms.y,ms.x,0];return!Bc(t,$s,Qs,er,ra)||(t=[1,0,0,0,1,0,0,0,1],!Bc(t,$s,Qs,er,ra))?!1:(oa.crossVectors(Zi,Ki),t=[oa.x,oa.y,oa.z],Bc(t,$s,Qs,er,ra))}clampPoint(e,t){return t.copy(e).clamp(this.min,this.max)}distanceToPoint(e){return this.clampPoint(e,Jn).distanceTo(e)}getBoundingSphere(e){return this.isEmpty()?e.makeEmpty():(this.getCenter(e.center),e.radius=this.getSize(Jn).length()*.5),e}intersect(e){return this.min.max(e.min),this.max.min(e.max),this.isEmpty()&&this.makeEmpty(),this}union(e){return this.min.min(e.min),this.max.max(e.max),this}applyMatrix4(e){return this.isEmpty()?this:(Ci[0].set(this.min.x,this.min.y,this.min.z).applyMatrix4(e),Ci[1].set(this.min.x,this.min.y,this.max.z).applyMatrix4(e),Ci[2].set(this.min.x,this.max.y,this.min.z).applyMatrix4(e),Ci[3].set(this.min.x,this.max.y,this.max.z).applyMatrix4(e),Ci[4].set(this.max.x,this.min.y,this.min.z).applyMatrix4(e),Ci[5].set(this.max.x,this.min.y,this.max.z).applyMatrix4(e),Ci[6].set(this.max.x,this.max.y,this.min.z).applyMatrix4(e),Ci[7].set(this.max.x,this.max.y,this.max.z).applyMatrix4(e),this.setFromPoints(Ci),this)}translate(e){return this.min.add(e),this.max.add(e),this}equals(e){return e.min.equals(this.min)&&e.max.equals(this.max)}toJSON(){return{min:this.min.toArray(),max:this.max.toArray()}}fromJSON(e){return this.min.fromArray(e.min),this.max.fromArray(e.max),this}},Ci=[new B,new B,new B,new B,new B,new B,new B,new B],Jn=new B,sa=new en,$s=new B,Qs=new B,er=new B,Zi=new B,Ki=new B,ms=new B,Vr=new B,ra=new B,oa=new B,gs=new B;function Bc(s,e,t,n,i){for(let r=0,o=s.length-3;r<=o;r+=3){gs.fromArray(s,r);let a=i.x*Math.abs(gs.x)+i.y*Math.abs(gs.y)+i.z*Math.abs(gs.z),l=e.dot(gs),c=t.dot(gs),h=n.dot(gs);if(Math.max(-Math.max(l,c,h),Math.min(l,c,h))>a)return!1}return!0}var $t=new B,aa=new _e,hm=0,xt=class extends Vn{constructor(e,t,n=!1){if(super(),Array.isArray(e))throw new TypeError("THREE.BufferAttribute: array should be a Typed Array.");this.isBufferAttribute=!0,Object.defineProperty(this,"id",{value:hm++}),this.name="",this.array=e,this.itemSize=t,this.count=e!==void 0?e.length/t:0,this.normalized=n,this.usage=Oa,this.updateRanges=[],this.gpuType=Un,this.version=0}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.name=e.name,this.array=new e.array.constructor(e.array),this.itemSize=e.itemSize,this.count=e.count,this.normalized=e.normalized,this.usage=e.usage,this.gpuType=e.gpuType,this}copyAt(e,t,n){e*=this.itemSize,n*=t.itemSize;for(let i=0,r=this.itemSize;i<r;i++)this.array[e+i]=t.array[n+i];return this}copyArray(e){return this.array.set(e),this}applyMatrix3(e){if(this.itemSize===2)for(let t=0,n=this.count;t<n;t++)aa.fromBufferAttribute(this,t),aa.applyMatrix3(e),this.setXY(t,aa.x,aa.y);else if(this.itemSize===3)for(let t=0,n=this.count;t<n;t++)$t.fromBufferAttribute(this,t),$t.applyMatrix3(e),this.setXYZ(t,$t.x,$t.y,$t.z);return this}applyMatrix4(e){for(let t=0,n=this.count;t<n;t++)$t.fromBufferAttribute(this,t),$t.applyMatrix4(e),this.setXYZ(t,$t.x,$t.y,$t.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)$t.fromBufferAttribute(this,t),$t.applyNormalMatrix(e),this.setXYZ(t,$t.x,$t.y,$t.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)$t.fromBufferAttribute(this,t),$t.transformDirection(e),this.setXYZ(t,$t.x,$t.y,$t.z);return this}set(e,t=0){return this.array.set(e,t),this}getComponent(e,t){let n=this.array[e*this.itemSize+t];return this.normalized&&(n=$n(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Et(n,this.array)),this.array[e*this.itemSize+t]=n,this}getX(e){let t=this.array[e*this.itemSize];return this.normalized&&(t=$n(t,this.array)),t}setX(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize]=t,this}getY(e){let t=this.array[e*this.itemSize+1];return this.normalized&&(t=$n(t,this.array)),t}setY(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize+1]=t,this}getZ(e){let t=this.array[e*this.itemSize+2];return this.normalized&&(t=$n(t,this.array)),t}setZ(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize+2]=t,this}getW(e){let t=this.array[e*this.itemSize+3];return this.normalized&&(t=$n(t,this.array)),t}setW(e,t){return this.normalized&&(t=Et(t,this.array)),this.array[e*this.itemSize+3]=t,this}setXY(e,t,n){return e*=this.itemSize,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array)),this.array[e+0]=t,this.array[e+1]=n,this}setXYZ(e,t,n,i){return e*=this.itemSize,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e*=this.itemSize,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array),r=Et(r,this.array)),this.array[e+0]=t,this.array[e+1]=n,this.array[e+2]=i,this.array[e+3]=r,this}onUpload(e){return this.onUploadCallback=e,this}clone(){return new this.constructor(this.array,this.itemSize).copy(this)}toJSON(){let e={itemSize:this.itemSize,type:this.array.constructor.name,array:Array.from(this.array),normalized:this.normalized};return this.name!==""&&(e.name=this.name),this.usage!==Oa&&(e.usage=this.usage),e}dispose(){this.dispatchEvent({type:"dispose"})}};var so=class extends xt{constructor(e,t,n){super(new Uint16Array(e),t,n)}};var ro=class extends xt{constructor(e,t,n){super(new Uint32Array(e),t,n)}};var nt=class extends xt{constructor(e,t,n){super(new Float32Array(e),t,n)}},um=new en,Gr=new B,zc=new B,dn=class{constructor(e=new B,t=-1){this.isSphere=!0,this.center=e,this.radius=t}set(e,t){return this.center.copy(e),this.radius=t,this}setFromPoints(e,t){let n=this.center;t!==void 0?n.copy(t):um.setFromPoints(e).getCenter(n);let i=0;for(let r=0,o=e.length;r<o;r++)i=Math.max(i,n.distanceToSquared(e[r]));return this.radius=Math.sqrt(i),this}copy(e){return this.center.copy(e.center),this.radius=e.radius,this}isEmpty(){return this.radius<0}makeEmpty(){return this.center.set(0,0,0),this.radius=-1,this}containsPoint(e){return e.distanceToSquared(this.center)<=this.radius*this.radius}distanceToPoint(e){return e.distanceTo(this.center)-this.radius}intersectsSphere(e){let t=this.radius+e.radius;return e.center.distanceToSquared(this.center)<=t*t}intersectsBox(e){return e.intersectsSphere(this)}intersectsPlane(e){return Math.abs(e.distanceToPoint(this.center))<=this.radius}clampPoint(e,t){let n=this.center.distanceToSquared(e);return t.copy(e),n>this.radius*this.radius&&(t.sub(this.center).normalize(),t.multiplyScalar(this.radius).add(this.center)),t}getBoundingBox(e){return this.isEmpty()?(e.makeEmpty(),e):(e.set(this.center,this.center),e.expandByScalar(this.radius),e)}applyMatrix4(e){return this.center.applyMatrix4(e),this.radius=this.radius*e.getMaxScaleOnAxis(),this}translate(e){return this.center.add(e),this}expandByPoint(e){if(this.isEmpty())return this.center.copy(e),this.radius=0,this;Gr.subVectors(e,this.center);let t=Gr.lengthSq();if(t>this.radius*this.radius){let n=Math.sqrt(t),i=(n-this.radius)*.5;this.center.addScaledVector(Gr,i/n),this.radius+=i}return this}union(e){return e.isEmpty()?this:this.isEmpty()?(this.copy(e),this):(this.center.equals(e.center)===!0?this.radius=Math.max(this.radius,e.radius):(zc.subVectors(e.center,this.center).setLength(e.radius),this.expandByPoint(Gr.copy(e.center).add(zc)),this.expandByPoint(Gr.copy(e.center).sub(zc))),this)}equals(e){return e.center.equals(this.center)&&e.radius===this.radius}clone(){return new this.constructor().copy(this)}toJSON(){return{radius:this.radius,center:this.center.toArray()}}fromJSON(e){return this.radius=e.radius,this.center.fromArray(e.center),this}},dm=0,zn=new Qe,kc=new St,tr=new B,In=new en,Wr=new en,an=new B,pt=class s extends Vn{constructor(){super(),this.isBufferGeometry=!0,Object.defineProperty(this,"id",{value:dm++}),this.uuid=ei(),this.name="",this.type="BufferGeometry",this.index=null,this.indirect=null,this.indirectOffset=0,this.attributes={},this.morphAttributes={},this.morphTargetsRelative=!1,this.groups=[],this.boundingBox=null,this.boundingSphere=null,this.drawRange={start:0,count:1/0},this.userData={},this._transformed=!1}getIndex(){return this.index}setIndex(e){return Array.isArray(e)?this.index=new(Fp(e)?ro:so)(e,1):this.index=e,this}setIndirect(e,t=0){return this.indirect=e,this.indirectOffset=t,this}getIndirect(){return this.indirect}getAttribute(e){return this.attributes[e]}setAttribute(e,t){return this.attributes[e]=t,this}deleteAttribute(e){return delete this.attributes[e],this}hasAttribute(e){return this.attributes[e]!==void 0}addGroup(e,t,n=0){this.groups.push({start:e,count:t,materialIndex:n})}clearGroups(){this.groups=[]}setDrawRange(e,t){this.drawRange.start=e,this.drawRange.count=t}applyMatrix4(e){let t=this.attributes.position;t!==void 0&&(t.applyMatrix4(e),t.needsUpdate=!0);let n=this.attributes.normal;if(n!==void 0){let r=new et().getNormalMatrix(e);n.applyNormalMatrix(r),n.needsUpdate=!0}let i=this.attributes.tangent;return i!==void 0&&(i.transformDirection(e),i.needsUpdate=!0),this.boundingBox!==null&&this.computeBoundingBox(),this.boundingSphere!==null&&this.computeBoundingSphere(),this._transformed=!0,this}applyQuaternion(e){return zn.makeRotationFromQuaternion(e),this.applyMatrix4(zn),this}rotateX(e){return zn.makeRotationX(e),this.applyMatrix4(zn),this}rotateY(e){return zn.makeRotationY(e),this.applyMatrix4(zn),this}rotateZ(e){return zn.makeRotationZ(e),this.applyMatrix4(zn),this}translate(e,t,n){return zn.makeTranslation(e,t,n),this.applyMatrix4(zn),this}scale(e,t,n){return zn.makeScale(e,t,n),this.applyMatrix4(zn),this}lookAt(e){return kc.lookAt(e),kc.updateMatrix(),this.applyMatrix4(kc.matrix),this}center(){return this.computeBoundingBox(),this.boundingBox.getCenter(tr).negate(),this.translate(tr.x,tr.y,tr.z),this}setFromPoints(e){let t=this.getAttribute("position");if(t===void 0){let n=[];for(let i=0,r=e.length;i<r;i++){let o=e[i];n.push(o.x,o.y,o.z||0)}this.setAttribute("position",new nt(n,3))}else{let n=Math.min(e.length,t.count);for(let i=0;i<n;i++){let r=e[i];t.setXYZ(i,r.x,r.y,r.z||0)}e.length>t.count&&We("BufferGeometry: Buffer size too small for points data. Use .dispose() and create a new geometry."),t.needsUpdate=!0}return this}computeBoundingBox(){this.boundingBox===null&&(this.boundingBox=new en);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){$e("BufferGeometry.computeBoundingBox(): GLBufferAttribute requires a manual bounding box.",this),this.boundingBox.set(new B(-1/0,-1/0,-1/0),new B(1/0,1/0,1/0));return}if(e!==void 0){if(this.boundingBox.setFromBufferAttribute(e),t)for(let n=0,i=t.length;n<i;n++){let r=t[n];In.setFromBufferAttribute(r),this.morphTargetsRelative?(an.addVectors(this.boundingBox.min,In.min),this.boundingBox.expandByPoint(an),an.addVectors(this.boundingBox.max,In.max),this.boundingBox.expandByPoint(an)):(this.boundingBox.expandByPoint(In.min),this.boundingBox.expandByPoint(In.max))}}else this.boundingBox.makeEmpty();(isNaN(this.boundingBox.min.x)||isNaN(this.boundingBox.min.y)||isNaN(this.boundingBox.min.z))&&$e('BufferGeometry.computeBoundingBox(): Computed min/max have NaN values. The "position" attribute is likely to have NaN values.',this)}computeBoundingSphere(){this.boundingSphere===null&&(this.boundingSphere=new dn);let e=this.attributes.position,t=this.morphAttributes.position;if(e&&e.isGLBufferAttribute){$e("BufferGeometry.computeBoundingSphere(): GLBufferAttribute requires a manual bounding sphere.",this),this.boundingSphere.set(new B,1/0);return}if(e){let n=this.boundingSphere.center;if(In.setFromBufferAttribute(e),t)for(let r=0,o=t.length;r<o;r++){let a=t[r];Wr.setFromBufferAttribute(a),this.morphTargetsRelative?(an.addVectors(In.min,Wr.min),In.expandByPoint(an),an.addVectors(In.max,Wr.max),In.expandByPoint(an)):(In.expandByPoint(Wr.min),In.expandByPoint(Wr.max))}In.getCenter(n);let i=0;for(let r=0,o=e.count;r<o;r++)an.fromBufferAttribute(e,r),i=Math.max(i,n.distanceToSquared(an));if(t)for(let r=0,o=t.length;r<o;r++){let a=t[r],l=this.morphTargetsRelative;for(let c=0,h=a.count;c<h;c++)an.fromBufferAttribute(a,c),l&&(tr.fromBufferAttribute(e,c),an.add(tr)),i=Math.max(i,n.distanceToSquared(an))}this.boundingSphere.radius=Math.sqrt(i),isNaN(this.boundingSphere.radius)&&$e('BufferGeometry.computeBoundingSphere(): Computed radius is NaN. The "position" attribute is likely to have NaN values.',this)}}computeTangents(){let e=this.index,t=this.attributes;if(e===null||t.position===void 0||t.normal===void 0||t.uv===void 0){$e("BufferGeometry: .computeTangents() failed. Missing required attributes (index, position, normal or uv)");return}let n=t.position,i=t.normal,r=t.uv,o=this.getAttribute("tangent");(o===void 0||o.count!==n.count)&&(o=new xt(new Float32Array(4*n.count),4),this.setAttribute("tangent",o));let a=[],l=[];for(let v=0;v<n.count;v++)a[v]=new B,l[v]=new B;let c=new B,h=new B,u=new B,d=new _e,f=new _e,g=new _e,y=new B,m=new B;function p(v,D,w){c.fromBufferAttribute(n,v),h.fromBufferAttribute(n,D),u.fromBufferAttribute(n,w),d.fromBufferAttribute(r,v),f.fromBufferAttribute(r,D),g.fromBufferAttribute(r,w),h.sub(c),u.sub(c),f.sub(d),g.sub(d);let R=1/(f.x*g.y-g.x*f.y);isFinite(R)&&(y.copy(h).multiplyScalar(g.y).addScaledVector(u,-f.y).multiplyScalar(R),m.copy(u).multiplyScalar(f.x).addScaledVector(h,-g.x).multiplyScalar(R),a[v].add(y),a[D].add(y),a[w].add(y),l[v].add(m),l[D].add(m),l[w].add(m))}let b=this.groups;b.length===0&&(b=[{start:0,count:e.count}]);for(let v=0,D=b.length;v<D;++v){let w=b[v],R=w.start,L=w.count;for(let V=R,C=R+L;V<C;V+=3)p(e.getX(V+0),e.getX(V+1),e.getX(V+2))}let S=new B,x=new B,A=new B,T=new B;function I(v){A.fromBufferAttribute(i,v),T.copy(A);let D=a[v];S.copy(D),S.sub(A.multiplyScalar(A.dot(D))).normalize(),x.crossVectors(T,D);let R=x.dot(l[v])<0?-1:1;o.setXYZW(v,S.x,S.y,S.z,R)}for(let v=0,D=b.length;v<D;++v){let w=b[v],R=w.start,L=w.count;for(let V=R,C=R+L;V<C;V+=3)I(e.getX(V+0)),I(e.getX(V+1)),I(e.getX(V+2))}this._transformed=!0}computeVertexNormals(){let e=this.index,t=this.getAttribute("position");if(t!==void 0){let n=this.getAttribute("normal");if(n===void 0||n.count!==t.count)n=new xt(new Float32Array(t.count*3),3),this.setAttribute("normal",n);else for(let d=0,f=n.count;d<f;d++)n.setXYZ(d,0,0,0);let i=new B,r=new B,o=new B,a=new B,l=new B,c=new B,h=new B,u=new B;if(e)for(let d=0,f=e.count;d<f;d+=3){let g=e.getX(d+0),y=e.getX(d+1),m=e.getX(d+2);i.fromBufferAttribute(t,g),r.fromBufferAttribute(t,y),o.fromBufferAttribute(t,m),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),a.fromBufferAttribute(n,g),l.fromBufferAttribute(n,y),c.fromBufferAttribute(n,m),a.add(h),l.add(h),c.add(h),n.setXYZ(g,a.x,a.y,a.z),n.setXYZ(y,l.x,l.y,l.z),n.setXYZ(m,c.x,c.y,c.z)}else for(let d=0,f=t.count;d<f;d+=3)i.fromBufferAttribute(t,d+0),r.fromBufferAttribute(t,d+1),o.fromBufferAttribute(t,d+2),h.subVectors(o,r),u.subVectors(i,r),h.cross(u),n.setXYZ(d+0,h.x,h.y,h.z),n.setXYZ(d+1,h.x,h.y,h.z),n.setXYZ(d+2,h.x,h.y,h.z);this.normalizeNormals(),n.needsUpdate=!0}}normalizeNormals(){let e=this.attributes.normal;for(let t=0,n=e.count;t<n;t++)an.fromBufferAttribute(e,t),an.normalize(),e.setXYZ(t,an.x,an.y,an.z)}toNonIndexed(){function e(a,l){let c=a.array,h=a.itemSize,u=a.normalized,d=new c.constructor(l.length*h),f=0,g=0;for(let y=0,m=l.length;y<m;y++){a.isInterleavedBufferAttribute?f=l[y]*a.data.stride+a.offset:f=l[y]*h;for(let p=0;p<h;p++)d[g++]=c[f++]}return new xt(d,h,u)}if(this.index===null)return We("BufferGeometry.toNonIndexed(): BufferGeometry is already non-indexed."),this;let t=new s,n=this.index.array,i=this.attributes;for(let a in i){let l=i[a],c=e(l,n);t.setAttribute(a,c)}let r=this.morphAttributes;for(let a in r){let l=[],c=r[a];for(let h=0,u=c.length;h<u;h++){let d=c[h],f=e(d,n);l.push(f)}t.morphAttributes[a]=l}t.morphTargetsRelative=this.morphTargetsRelative;let o=this.groups;for(let a=0,l=o.length;a<l;a++){let c=o[a];t.addGroup(c.start,c.count,c.materialIndex)}return t}toJSON(){let e={metadata:{version:4.7,type:"BufferGeometry",generator:"BufferGeometry.toJSON"}};if(e.uuid=this.uuid,e.type=this.parameters!==void 0&&this._transformed===!0?"BufferGeometry":this.type,this.name!==""&&(e.name=this.name),Object.keys(this.userData).length>0&&(e.userData=this.userData),this.parameters!==void 0&&this._transformed!==!0){let l=this.parameters;for(let c in l)l[c]!==void 0&&(e[c]=l[c]);return e}e.data={attributes:{}};let t=this.index;t!==null&&(e.data.index={type:t.array.constructor.name,array:Array.prototype.slice.call(t.array)});let n=this.attributes;for(let l in n){let c=n[l];e.data.attributes[l]=c.toJSON(e.data)}let i={},r=!1;for(let l in this.morphAttributes){let c=this.morphAttributes[l],h=[];for(let u=0,d=c.length;u<d;u++){let f=c[u];h.push(f.toJSON(e.data))}h.length>0&&(i[l]=h,r=!0)}r&&(e.data.morphAttributes=i,e.data.morphTargetsRelative=this.morphTargetsRelative);let o=this.groups;o.length>0&&(e.data.groups=JSON.parse(JSON.stringify(o)));let a=this.boundingSphere;return a!==null&&(e.data.boundingSphere=a.toJSON()),e}clone(){return new this.constructor().copy(this)}copy(e){this.index=null,this.attributes={},this.morphAttributes={},this.groups=[],this.boundingBox=null,this.boundingSphere=null;let t={};this.name=e.name;let n=e.index;n!==null&&this.setIndex(n.clone());let i=e.attributes;for(let c in i){let h=i[c];this.setAttribute(c,h.clone(t))}let r=e.morphAttributes;for(let c in r){let h=[],u=r[c];for(let d=0,f=u.length;d<f;d++)h.push(u[d].clone(t));this.morphAttributes[c]=h}this.morphTargetsRelative=e.morphTargetsRelative;let o=e.groups;for(let c=0,h=o.length;c<h;c++){let u=o[c];this.addGroup(u.start,u.count,u.materialIndex)}let a=e.boundingBox;a!==null&&(this.boundingBox=a.clone());let l=e.boundingSphere;return l!==null&&(this.boundingSphere=l.clone()),this.drawRange.start=e.drawRange.start,this.drawRange.count=e.drawRange.count,this.userData=e.userData,this._transformed=e._transformed,this}dispose(){this.dispatchEvent({type:"dispose"})}},pr=class{constructor(e,t){this.isInterleavedBuffer=!0,this.array=e,this.stride=t,this.count=e!==void 0?e.length/t:0,this.usage=Oa,this.updateRanges=[],this.version=0,this.uuid=ei()}onUploadCallback(){}set needsUpdate(e){e===!0&&this.version++}setUsage(e){return this.usage=e,this}addUpdateRange(e,t){this.updateRanges.push({start:e,count:t})}clearUpdateRanges(){this.updateRanges.length=0}copy(e){return this.array=new e.array.constructor(e.array),this.count=e.count,this.stride=e.stride,this.usage=e.usage,this}copyAt(e,t,n){e*=this.stride,n*=t.stride;for(let i=0,r=this.stride;i<r;i++)this.array[e+i]=t.array[n+i];return this}set(e,t=0){return this.array.set(e,t),this}clone(e){e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=ei()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=this.array.slice(0).buffer);let t=new this.array.constructor(e.arrayBuffers[this.array.buffer._uuid]),n=new this.constructor(t,this.stride);return n.setUsage(this.usage),n}onUpload(e){return this.onUploadCallback=e,this}toJSON(e){return e.arrayBuffers===void 0&&(e.arrayBuffers={}),this.array.buffer._uuid===void 0&&(this.array.buffer._uuid=ei()),e.arrayBuffers[this.array.buffer._uuid]===void 0&&(e.arrayBuffers[this.array.buffer._uuid]=Array.from(new Uint32Array(this.array.buffer))),{uuid:this.uuid,buffer:this.array.buffer._uuid,type:this.array.constructor.name,stride:this.stride}}},mn=new B,mr=class s{constructor(e,t,n,i=!1){this.isInterleavedBufferAttribute=!0,this.name="",this.data=e,this.itemSize=t,this.offset=n,this.normalized=i}get count(){return this.data.count}get array(){return this.data.array}set needsUpdate(e){this.data.needsUpdate=e}applyMatrix4(e){for(let t=0,n=this.data.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.applyMatrix4(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}applyNormalMatrix(e){for(let t=0,n=this.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.applyNormalMatrix(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}transformDirection(e){for(let t=0,n=this.count;t<n;t++)mn.fromBufferAttribute(this,t),mn.transformDirection(e),this.setXYZ(t,mn.x,mn.y,mn.z);return this}getComponent(e,t){let n=this.array[e*this.data.stride+this.offset+t];return this.normalized&&(n=$n(n,this.array)),n}setComponent(e,t,n){return this.normalized&&(n=Et(n,this.array)),this.data.array[e*this.data.stride+this.offset+t]=n,this}setX(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset]=t,this}setY(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset+1]=t,this}setZ(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset+2]=t,this}setW(e,t){return this.normalized&&(t=Et(t,this.array)),this.data.array[e*this.data.stride+this.offset+3]=t,this}getX(e){let t=this.data.array[e*this.data.stride+this.offset];return this.normalized&&(t=$n(t,this.array)),t}getY(e){let t=this.data.array[e*this.data.stride+this.offset+1];return this.normalized&&(t=$n(t,this.array)),t}getZ(e){let t=this.data.array[e*this.data.stride+this.offset+2];return this.normalized&&(t=$n(t,this.array)),t}getW(e){let t=this.data.array[e*this.data.stride+this.offset+3];return this.normalized&&(t=$n(t,this.array)),t}setXY(e,t,n){return e=e*this.data.stride+this.offset,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this}setXYZ(e,t,n,i){return e=e*this.data.stride+this.offset,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this}setXYZW(e,t,n,i,r){return e=e*this.data.stride+this.offset,this.normalized&&(t=Et(t,this.array),n=Et(n,this.array),i=Et(i,this.array),r=Et(r,this.array)),this.data.array[e+0]=t,this.data.array[e+1]=n,this.data.array[e+2]=i,this.data.array[e+3]=r,this}clone(e){if(e===void 0){to("InterleavedBufferAttribute.clone(): Cloning an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return new xt(new this.array.constructor(t),this.itemSize,this.normalized)}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.clone(e)),new s(e.interleavedBuffers[this.data.uuid],this.itemSize,this.offset,this.normalized)}toJSON(e){if(e===void 0){to("InterleavedBufferAttribute.toJSON(): Serializing an interleaved buffer attribute will de-interleave buffer data.");let t=[];for(let n=0;n<this.count;n++){let i=n*this.data.stride+this.offset;for(let r=0;r<this.itemSize;r++)t.push(this.data.array[i+r])}return{itemSize:this.itemSize,type:this.array.constructor.name,array:t,normalized:this.normalized}}else return e.interleavedBuffers===void 0&&(e.interleavedBuffers={}),e.interleavedBuffers[this.data.uuid]===void 0&&(e.interleavedBuffers[this.data.uuid]=this.data.toJSON(e)),{isInterleavedBufferAttribute:!0,itemSize:this.itemSize,data:this.data.uuid,offset:this.offset,normalized:this.normalized}}},fm=0,xn=class extends Vn{constructor(){super(),this.isMaterial=!0,Object.defineProperty(this,"id",{value:fm++}),this.uuid=ei(),this.name="",this.type="Material",this.blending=Ss,this.side=Ln,this.vertexColors=!1,this.opacity=1,this.transparent=!1,this.alphaHash=!1,this.blendSrc=Ra,this.blendDst=Ca,this.blendEquation=Qi,this.blendSrcAlpha=null,this.blendDstAlpha=null,this.blendEquationAlpha=null,this.blendColor=new xe(0,0,0),this.blendAlpha=0,this.depthFunc=ws,this.depthTest=!0,this.depthWrite=!0,this.stencilWriteMask=255,this.stencilFunc=nh,this.stencilRef=0,this.stencilFuncMask=255,this.stencilFail=vs,this.stencilZFail=vs,this.stencilZPass=vs,this.stencilWrite=!1,this.clippingPlanes=null,this.clipIntersection=!1,this.clipShadows=!1,this.shadowSide=null,this.colorWrite=!0,this.precision=null,this.polygonOffset=!1,this.polygonOffsetFactor=0,this.polygonOffsetUnits=0,this.dithering=!1,this.alphaToCoverage=!1,this.premultipliedAlpha=!1,this.forceSinglePass=!1,this.allowOverride=!0,this.visible=!0,this.toneMapped=!0,this.userData={},this.version=0,this._alphaTest=0}get alphaTest(){return this._alphaTest}set alphaTest(e){this._alphaTest>0!=e>0&&this.version++,this._alphaTest=e}onBeforeRender(){}onBeforeCompile(){}customProgramCacheKey(){return this.onBeforeCompile.toString()}setValues(e){if(e!==void 0)for(let t in e){let n=e[t];if(n===void 0){We(`Material: parameter '${t}' has value of undefined.`);continue}let i=this[t];if(i===void 0){We(`Material: '${t}' is not a property of THREE.${this.type}.`);continue}i&&i.isColor?i.set(n):i&&i.isVector2&&n&&n.isVector2||i&&i.isEuler&&n&&n.isEuler||i&&i.isVector3&&n&&n.isVector3?i.copy(n):this[t]=n}}toJSON(e){let t=e===void 0||typeof e=="string";t&&(e={textures:{},images:{}});let n={metadata:{version:4.7,type:"Material",generator:"Material.toJSON"}};n.uuid=this.uuid,n.type=this.type,this.name!==""&&(n.name=this.name),this.color&&this.color.isColor&&(n.color=this.color.getHex()),this.roughness!==void 0&&(n.roughness=this.roughness),this.metalness!==void 0&&(n.metalness=this.metalness),this.sheen!==void 0&&(n.sheen=this.sheen),this.sheenColor&&this.sheenColor.isColor&&(n.sheenColor=this.sheenColor.getHex()),this.sheenRoughness!==void 0&&(n.sheenRoughness=this.sheenRoughness),this.emissive&&this.emissive.isColor&&(n.emissive=this.emissive.getHex()),this.emissiveIntensity!==void 0&&this.emissiveIntensity!==1&&(n.emissiveIntensity=this.emissiveIntensity),this.specular&&this.specular.isColor&&(n.specular=this.specular.getHex()),this.specularIntensity!==void 0&&(n.specularIntensity=this.specularIntensity),this.specularColor&&this.specularColor.isColor&&(n.specularColor=this.specularColor.getHex()),this.shininess!==void 0&&(n.shininess=this.shininess),this.clearcoat!==void 0&&(n.clearcoat=this.clearcoat),this.clearcoatRoughness!==void 0&&(n.clearcoatRoughness=this.clearcoatRoughness),this.clearcoatMap&&this.clearcoatMap.isTexture&&(n.clearcoatMap=this.clearcoatMap.toJSON(e).uuid),this.clearcoatRoughnessMap&&this.clearcoatRoughnessMap.isTexture&&(n.clearcoatRoughnessMap=this.clearcoatRoughnessMap.toJSON(e).uuid),this.clearcoatNormalMap&&this.clearcoatNormalMap.isTexture&&(n.clearcoatNormalMap=this.clearcoatNormalMap.toJSON(e).uuid,n.clearcoatNormalScale=this.clearcoatNormalScale.toArray()),this.sheenColorMap&&this.sheenColorMap.isTexture&&(n.sheenColorMap=this.sheenColorMap.toJSON(e).uuid),this.sheenRoughnessMap&&this.sheenRoughnessMap.isTexture&&(n.sheenRoughnessMap=this.sheenRoughnessMap.toJSON(e).uuid),this.dispersion!==void 0&&(n.dispersion=this.dispersion),this.iridescence!==void 0&&(n.iridescence=this.iridescence),this.iridescenceIOR!==void 0&&(n.iridescenceIOR=this.iridescenceIOR),this.iridescenceThicknessRange!==void 0&&(n.iridescenceThicknessRange=this.iridescenceThicknessRange),this.iridescenceMap&&this.iridescenceMap.isTexture&&(n.iridescenceMap=this.iridescenceMap.toJSON(e).uuid),this.iridescenceThicknessMap&&this.iridescenceThicknessMap.isTexture&&(n.iridescenceThicknessMap=this.iridescenceThicknessMap.toJSON(e).uuid),this.anisotropy!==void 0&&(n.anisotropy=this.anisotropy),this.anisotropyRotation!==void 0&&(n.anisotropyRotation=this.anisotropyRotation),this.anisotropyMap&&this.anisotropyMap.isTexture&&(n.anisotropyMap=this.anisotropyMap.toJSON(e).uuid),this.map&&this.map.isTexture&&(n.map=this.map.toJSON(e).uuid),this.matcap&&this.matcap.isTexture&&(n.matcap=this.matcap.toJSON(e).uuid),this.alphaMap&&this.alphaMap.isTexture&&(n.alphaMap=this.alphaMap.toJSON(e).uuid),this.lightMap&&this.lightMap.isTexture&&(n.lightMap=this.lightMap.toJSON(e).uuid,n.lightMapIntensity=this.lightMapIntensity),this.aoMap&&this.aoMap.isTexture&&(n.aoMap=this.aoMap.toJSON(e).uuid,n.aoMapIntensity=this.aoMapIntensity),this.bumpMap&&this.bumpMap.isTexture&&(n.bumpMap=this.bumpMap.toJSON(e).uuid,n.bumpScale=this.bumpScale),this.normalMap&&this.normalMap.isTexture&&(n.normalMap=this.normalMap.toJSON(e).uuid,n.normalMapType=this.normalMapType,n.normalScale=this.normalScale.toArray()),this.displacementMap&&this.displacementMap.isTexture&&(n.displacementMap=this.displacementMap.toJSON(e).uuid,n.displacementScale=this.displacementScale,n.displacementBias=this.displacementBias),this.roughnessMap&&this.roughnessMap.isTexture&&(n.roughnessMap=this.roughnessMap.toJSON(e).uuid),this.metalnessMap&&this.metalnessMap.isTexture&&(n.metalnessMap=this.metalnessMap.toJSON(e).uuid),this.emissiveMap&&this.emissiveMap.isTexture&&(n.emissiveMap=this.emissiveMap.toJSON(e).uuid),this.specularMap&&this.specularMap.isTexture&&(n.specularMap=this.specularMap.toJSON(e).uuid),this.specularIntensityMap&&this.specularIntensityMap.isTexture&&(n.specularIntensityMap=this.specularIntensityMap.toJSON(e).uuid),this.specularColorMap&&this.specularColorMap.isTexture&&(n.specularColorMap=this.specularColorMap.toJSON(e).uuid),this.envMap&&this.envMap.isTexture&&(n.envMap=this.envMap.toJSON(e).uuid,this.combine!==void 0&&(n.combine=this.combine)),this.envMapRotation!==void 0&&(n.envMapRotation=this.envMapRotation.toArray()),this.envMapIntensity!==void 0&&(n.envMapIntensity=this.envMapIntensity),this.reflectivity!==void 0&&(n.reflectivity=this.reflectivity),this.refractionRatio!==void 0&&(n.refractionRatio=this.refractionRatio),this.gradientMap&&this.gradientMap.isTexture&&(n.gradientMap=this.gradientMap.toJSON(e).uuid),this.transmission!==void 0&&(n.transmission=this.transmission),this.transmissionMap&&this.transmissionMap.isTexture&&(n.transmissionMap=this.transmissionMap.toJSON(e).uuid),this.thickness!==void 0&&(n.thickness=this.thickness),this.thicknessMap&&this.thicknessMap.isTexture&&(n.thicknessMap=this.thicknessMap.toJSON(e).uuid),this.attenuationDistance!==void 0&&this.attenuationDistance!==1/0&&(n.attenuationDistance=this.attenuationDistance),this.attenuationColor!==void 0&&(n.attenuationColor=this.attenuationColor.getHex()),this.size!==void 0&&(n.size=this.size),this.shadowSide!==null&&(n.shadowSide=this.shadowSide),this.sizeAttenuation!==void 0&&(n.sizeAttenuation=this.sizeAttenuation),this.blending!==Ss&&(n.blending=this.blending),this.side!==Ln&&(n.side=this.side),this.vertexColors===!0&&(n.vertexColors=!0),this.opacity<1&&(n.opacity=this.opacity),this.transparent===!0&&(n.transparent=!0),this.blendSrc!==Ra&&(n.blendSrc=this.blendSrc),this.blendDst!==Ca&&(n.blendDst=this.blendDst),this.blendEquation!==Qi&&(n.blendEquation=this.blendEquation),this.blendSrcAlpha!==null&&(n.blendSrcAlpha=this.blendSrcAlpha),this.blendDstAlpha!==null&&(n.blendDstAlpha=this.blendDstAlpha),this.blendEquationAlpha!==null&&(n.blendEquationAlpha=this.blendEquationAlpha),this.blendColor&&this.blendColor.isColor&&(n.blendColor=this.blendColor.getHex()),this.blendAlpha!==0&&(n.blendAlpha=this.blendAlpha),this.depthFunc!==ws&&(n.depthFunc=this.depthFunc),this.depthTest===!1&&(n.depthTest=this.depthTest),this.depthWrite===!1&&(n.depthWrite=this.depthWrite),this.colorWrite===!1&&(n.colorWrite=this.colorWrite),this.stencilWriteMask!==255&&(n.stencilWriteMask=this.stencilWriteMask),this.stencilFunc!==nh&&(n.stencilFunc=this.stencilFunc),this.stencilRef!==0&&(n.stencilRef=this.stencilRef),this.stencilFuncMask!==255&&(n.stencilFuncMask=this.stencilFuncMask),this.stencilFail!==vs&&(n.stencilFail=this.stencilFail),this.stencilZFail!==vs&&(n.stencilZFail=this.stencilZFail),this.stencilZPass!==vs&&(n.stencilZPass=this.stencilZPass),this.stencilWrite===!0&&(n.stencilWrite=this.stencilWrite),this.rotation!==void 0&&this.rotation!==0&&(n.rotation=this.rotation),this.polygonOffset===!0&&(n.polygonOffset=!0),this.polygonOffsetFactor!==0&&(n.polygonOffsetFactor=this.polygonOffsetFactor),this.polygonOffsetUnits!==0&&(n.polygonOffsetUnits=this.polygonOffsetUnits),this.linewidth!==void 0&&this.linewidth!==1&&(n.linewidth=this.linewidth),this.dashSize!==void 0&&(n.dashSize=this.dashSize),this.gapSize!==void 0&&(n.gapSize=this.gapSize),this.scale!==void 0&&(n.scale=this.scale),this.dithering===!0&&(n.dithering=!0),this.alphaTest>0&&(n.alphaTest=this.alphaTest),this.alphaHash===!0&&(n.alphaHash=!0),this.alphaToCoverage===!0&&(n.alphaToCoverage=!0),this.premultipliedAlpha===!0&&(n.premultipliedAlpha=!0),this.forceSinglePass===!0&&(n.forceSinglePass=!0),this.allowOverride===!1&&(n.allowOverride=!1),this.wireframe===!0&&(n.wireframe=!0),this.wireframeLinewidth>1&&(n.wireframeLinewidth=this.wireframeLinewidth),this.wireframeLinecap!=="round"&&(n.wireframeLinecap=this.wireframeLinecap),this.wireframeLinejoin!=="round"&&(n.wireframeLinejoin=this.wireframeLinejoin),this.flatShading===!0&&(n.flatShading=!0),this.visible===!1&&(n.visible=!1),this.toneMapped===!1&&(n.toneMapped=!1),this.fog===!1&&(n.fog=!1),Object.keys(this.userData).length>0&&(n.userData=this.userData);function i(r){let o=[];for(let a in r){let l=r[a];delete l.metadata,o.push(l)}return o}if(t){let r=i(e.textures),o=i(e.images);r.length>0&&(n.textures=r),o.length>0&&(n.images=o)}return n}fromJSON(e,t){if(e.uuid!==void 0&&(this.uuid=e.uuid),e.name!==void 0&&(this.name=e.name),e.color!==void 0&&this.color!==void 0&&this.color.setHex(e.color),e.roughness!==void 0&&(this.roughness=e.roughness),e.metalness!==void 0&&(this.metalness=e.metalness),e.sheen!==void 0&&(this.sheen=e.sheen),e.sheenColor!==void 0&&(this.sheenColor=new xe().setHex(e.sheenColor)),e.sheenRoughness!==void 0&&(this.sheenRoughness=e.sheenRoughness),e.emissive!==void 0&&this.emissive!==void 0&&this.emissive.setHex(e.emissive),e.specular!==void 0&&this.specular!==void 0&&this.specular.setHex(e.specular),e.specularIntensity!==void 0&&(this.specularIntensity=e.specularIntensity),e.specularColor!==void 0&&this.specularColor!==void 0&&this.specularColor.setHex(e.specularColor),e.shininess!==void 0&&(this.shininess=e.shininess),e.clearcoat!==void 0&&(this.clearcoat=e.clearcoat),e.clearcoatRoughness!==void 0&&(this.clearcoatRoughness=e.clearcoatRoughness),e.dispersion!==void 0&&(this.dispersion=e.dispersion),e.iridescence!==void 0&&(this.iridescence=e.iridescence),e.iridescenceIOR!==void 0&&(this.iridescenceIOR=e.iridescenceIOR),e.iridescenceThicknessRange!==void 0&&(this.iridescenceThicknessRange=e.iridescenceThicknessRange),e.transmission!==void 0&&(this.transmission=e.transmission),e.thickness!==void 0&&(this.thickness=e.thickness),e.attenuationDistance!==void 0&&(this.attenuationDistance=e.attenuationDistance),e.attenuationColor!==void 0&&this.attenuationColor!==void 0&&this.attenuationColor.setHex(e.attenuationColor),e.anisotropy!==void 0&&(this.anisotropy=e.anisotropy),e.anisotropyRotation!==void 0&&(this.anisotropyRotation=e.anisotropyRotation),e.fog!==void 0&&(this.fog=e.fog),e.flatShading!==void 0&&(this.flatShading=e.flatShading),e.blending!==void 0&&(this.blending=e.blending),e.combine!==void 0&&(this.combine=e.combine),e.side!==void 0&&(this.side=e.side),e.shadowSide!==void 0&&(this.shadowSide=e.shadowSide),e.opacity!==void 0&&(this.opacity=e.opacity),e.transparent!==void 0&&(this.transparent=e.transparent),e.alphaTest!==void 0&&(this.alphaTest=e.alphaTest),e.alphaHash!==void 0&&(this.alphaHash=e.alphaHash),e.depthFunc!==void 0&&(this.depthFunc=e.depthFunc),e.depthTest!==void 0&&(this.depthTest=e.depthTest),e.depthWrite!==void 0&&(this.depthWrite=e.depthWrite),e.colorWrite!==void 0&&(this.colorWrite=e.colorWrite),e.blendSrc!==void 0&&(this.blendSrc=e.blendSrc),e.blendDst!==void 0&&(this.blendDst=e.blendDst),e.blendEquation!==void 0&&(this.blendEquation=e.blendEquation),e.blendSrcAlpha!==void 0&&(this.blendSrcAlpha=e.blendSrcAlpha),e.blendDstAlpha!==void 0&&(this.blendDstAlpha=e.blendDstAlpha),e.blendEquationAlpha!==void 0&&(this.blendEquationAlpha=e.blendEquationAlpha),e.blendColor!==void 0&&this.blendColor!==void 0&&this.blendColor.setHex(e.blendColor),e.blendAlpha!==void 0&&(this.blendAlpha=e.blendAlpha),e.stencilWriteMask!==void 0&&(this.stencilWriteMask=e.stencilWriteMask),e.stencilFunc!==void 0&&(this.stencilFunc=e.stencilFunc),e.stencilRef!==void 0&&(this.stencilRef=e.stencilRef),e.stencilFuncMask!==void 0&&(this.stencilFuncMask=e.stencilFuncMask),e.stencilFail!==void 0&&(this.stencilFail=e.stencilFail),e.stencilZFail!==void 0&&(this.stencilZFail=e.stencilZFail),e.stencilZPass!==void 0&&(this.stencilZPass=e.stencilZPass),e.stencilWrite!==void 0&&(this.stencilWrite=e.stencilWrite),e.wireframe!==void 0&&(this.wireframe=e.wireframe),e.wireframeLinewidth!==void 0&&(this.wireframeLinewidth=e.wireframeLinewidth),e.wireframeLinecap!==void 0&&(this.wireframeLinecap=e.wireframeLinecap),e.wireframeLinejoin!==void 0&&(this.wireframeLinejoin=e.wireframeLinejoin),e.rotation!==void 0&&(this.rotation=e.rotation),e.linewidth!==void 0&&(this.linewidth=e.linewidth),e.dashSize!==void 0&&(this.dashSize=e.dashSize),e.gapSize!==void 0&&(this.gapSize=e.gapSize),e.scale!==void 0&&(this.scale=e.scale),e.polygonOffset!==void 0&&(this.polygonOffset=e.polygonOffset),e.polygonOffsetFactor!==void 0&&(this.polygonOffsetFactor=e.polygonOffsetFactor),e.polygonOffsetUnits!==void 0&&(this.polygonOffsetUnits=e.polygonOffsetUnits),e.dithering!==void 0&&(this.dithering=e.dithering),e.alphaToCoverage!==void 0&&(this.alphaToCoverage=e.alphaToCoverage),e.premultipliedAlpha!==void 0&&(this.premultipliedAlpha=e.premultipliedAlpha),e.forceSinglePass!==void 0&&(this.forceSinglePass=e.forceSinglePass),e.allowOverride!==void 0&&(this.allowOverride=e.allowOverride),e.visible!==void 0&&(this.visible=e.visible),e.toneMapped!==void 0&&(this.toneMapped=e.toneMapped),e.userData!==void 0&&(this.userData=e.userData),e.vertexColors!==void 0&&(typeof e.vertexColors=="number"?this.vertexColors=e.vertexColors>0:this.vertexColors=e.vertexColors),e.size!==void 0&&(this.size=e.size),e.sizeAttenuation!==void 0&&(this.sizeAttenuation=e.sizeAttenuation),e.map!==void 0&&(this.map=t[e.map]||null),e.matcap!==void 0&&(this.matcap=t[e.matcap]||null),e.alphaMap!==void 0&&(this.alphaMap=t[e.alphaMap]||null),e.bumpMap!==void 0&&(this.bumpMap=t[e.bumpMap]||null),e.bumpScale!==void 0&&(this.bumpScale=e.bumpScale),e.normalMap!==void 0&&(this.normalMap=t[e.normalMap]||null),e.normalMapType!==void 0&&(this.normalMapType=e.normalMapType),e.normalScale!==void 0){let n=e.normalScale;Array.isArray(n)===!1&&(n=[n,n]),this.normalScale=new _e().fromArray(n)}return e.displacementMap!==void 0&&(this.displacementMap=t[e.displacementMap]||null),e.displacementScale!==void 0&&(this.displacementScale=e.displacementScale),e.displacementBias!==void 0&&(this.displacementBias=e.displacementBias),e.roughnessMap!==void 0&&(this.roughnessMap=t[e.roughnessMap]||null),e.metalnessMap!==void 0&&(this.metalnessMap=t[e.metalnessMap]||null),e.emissiveMap!==void 0&&(this.emissiveMap=t[e.emissiveMap]||null),e.emissiveIntensity!==void 0&&(this.emissiveIntensity=e.emissiveIntensity),e.specularMap!==void 0&&(this.specularMap=t[e.specularMap]||null),e.specularIntensityMap!==void 0&&(this.specularIntensityMap=t[e.specularIntensityMap]||null),e.specularColorMap!==void 0&&(this.specularColorMap=t[e.specularColorMap]||null),e.envMap!==void 0&&(this.envMap=t[e.envMap]||null),e.envMapRotation!==void 0&&this.envMapRotation.fromArray(e.envMapRotation),e.envMapIntensity!==void 0&&(this.envMapIntensity=e.envMapIntensity),e.reflectivity!==void 0&&(this.reflectivity=e.reflectivity),e.refractionRatio!==void 0&&(this.refractionRatio=e.refractionRatio),e.lightMap!==void 0&&(this.lightMap=t[e.lightMap]||null),e.lightMapIntensity!==void 0&&(this.lightMapIntensity=e.lightMapIntensity),e.aoMap!==void 0&&(this.aoMap=t[e.aoMap]||null),e.aoMapIntensity!==void 0&&(this.aoMapIntensity=e.aoMapIntensity),e.gradientMap!==void 0&&(this.gradientMap=t[e.gradientMap]||null),e.clearcoatMap!==void 0&&(this.clearcoatMap=t[e.clearcoatMap]||null),e.clearcoatRoughnessMap!==void 0&&(this.clearcoatRoughnessMap=t[e.clearcoatRoughnessMap]||null),e.clearcoatNormalMap!==void 0&&(this.clearcoatNormalMap=t[e.clearcoatNormalMap]||null),e.clearcoatNormalScale!==void 0&&(this.clearcoatNormalScale=new _e().fromArray(e.clearcoatNormalScale)),e.iridescenceMap!==void 0&&(this.iridescenceMap=t[e.iridescenceMap]||null),e.iridescenceThicknessMap!==void 0&&(this.iridescenceThicknessMap=t[e.iridescenceThicknessMap]||null),e.transmissionMap!==void 0&&(this.transmissionMap=t[e.transmissionMap]||null),e.thicknessMap!==void 0&&(this.thicknessMap=t[e.thicknessMap]||null),e.anisotropyMap!==void 0&&(this.anisotropyMap=t[e.anisotropyMap]||null),e.sheenColorMap!==void 0&&(this.sheenColorMap=t[e.sheenColorMap]||null),e.sheenRoughnessMap!==void 0&&(this.sheenRoughnessMap=t[e.sheenRoughnessMap]||null),this}clone(){return new this.constructor().copy(this)}copy(e){this.name=e.name,this.blending=e.blending,this.side=e.side,this.vertexColors=e.vertexColors,this.opacity=e.opacity,this.transparent=e.transparent,this.blendSrc=e.blendSrc,this.blendDst=e.blendDst,this.blendEquation=e.blendEquation,this.blendSrcAlpha=e.blendSrcAlpha,this.blendDstAlpha=e.blendDstAlpha,this.blendEquationAlpha=e.blendEquationAlpha,this.blendColor.copy(e.blendColor),this.blendAlpha=e.blendAlpha,this.depthFunc=e.depthFunc,this.depthTest=e.depthTest,this.depthWrite=e.depthWrite,this.stencilWriteMask=e.stencilWriteMask,this.stencilFunc=e.stencilFunc,this.stencilRef=e.stencilRef,this.stencilFuncMask=e.stencilFuncMask,this.stencilFail=e.stencilFail,this.stencilZFail=e.stencilZFail,this.stencilZPass=e.stencilZPass,this.stencilWrite=e.stencilWrite;let t=e.clippingPlanes,n=null;if(t!==null){let i=t.length;n=new Array(i);for(let r=0;r!==i;++r)n[r]=t[r].clone()}return this.clippingPlanes=n,this.clipIntersection=e.clipIntersection,this.clipShadows=e.clipShadows,this.shadowSide=e.shadowSide,this.colorWrite=e.colorWrite,this.precision=e.precision,this.polygonOffset=e.polygonOffset,this.polygonOffsetFactor=e.polygonOffsetFactor,this.polygonOffsetUnits=e.polygonOffsetUnits,this.dithering=e.dithering,this.alphaTest=e.alphaTest,this.alphaHash=e.alphaHash,this.alphaToCoverage=e.alphaToCoverage,this.premultipliedAlpha=e.premultipliedAlpha,this.forceSinglePass=e.forceSinglePass,this.allowOverride=e.allowOverride,this.visible=e.visible,this.toneMapped=e.toneMapped,this.userData=JSON.parse(JSON.stringify(e.userData)),this}dispose(){this.dispatchEvent({type:"dispose"})}set needsUpdate(e){e===!0&&this.version++}};var Pi=new B,Hc=new B,la=new B,ji=new B,Vc=new B,ca=new B,Gc=new B,fi=class{constructor(e=new B,t=new B(0,0,-1)){this.origin=e,this.direction=t}set(e,t){return this.origin.copy(e),this.direction.copy(t),this}copy(e){return this.origin.copy(e.origin),this.direction.copy(e.direction),this}at(e,t){return t.copy(this.origin).addScaledVector(this.direction,e)}lookAt(e){return this.direction.copy(e).sub(this.origin).normalize(),this}recast(e){return this.origin.copy(this.at(e,Pi)),this}closestPointToPoint(e,t){t.subVectors(e,this.origin);let n=t.dot(this.direction);return n<0?t.copy(this.origin):t.copy(this.origin).addScaledVector(this.direction,n)}distanceToPoint(e){return Math.sqrt(this.distanceSqToPoint(e))}distanceSqToPoint(e){let t=Pi.subVectors(e,this.origin).dot(this.direction);return t<0?this.origin.distanceToSquared(e):(Pi.copy(this.origin).addScaledVector(this.direction,t),Pi.distanceToSquared(e))}distanceSqToSegment(e,t,n,i){Hc.copy(e).add(t).multiplyScalar(.5),la.copy(t).sub(e).normalize(),ji.copy(this.origin).sub(Hc);let r=e.distanceTo(t)*.5,o=-this.direction.dot(la),a=ji.dot(this.direction),l=-ji.dot(la),c=ji.lengthSq(),h=Math.abs(1-o*o),u,d,f,g;if(h>0)if(u=o*l-a,d=o*a-l,g=r*h,u>=0)if(d>=-g)if(d<=g){let y=1/h;u*=y,d*=y,f=u*(u+o*d+2*a)+d*(o*u+d+2*l)+c}else d=r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d=-r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;else d<=-g?(u=Math.max(0,-(-o*r+a)),d=u>0?-r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c):d<=g?(u=0,d=Math.min(Math.max(-r,-l),r),f=d*(d+2*l)+c):(u=Math.max(0,-(o*r+a)),d=u>0?r:Math.min(Math.max(-r,-l),r),f=-u*u+d*(d+2*l)+c);else d=o>0?-r:r,u=Math.max(0,-(o*d+a)),f=-u*u+d*(d+2*l)+c;return n&&n.copy(this.origin).addScaledVector(this.direction,u),i&&i.copy(Hc).addScaledVector(la,d),f}intersectSphere(e,t){Pi.subVectors(e.center,this.origin);let n=Pi.dot(this.direction),i=Pi.dot(Pi)-n*n,r=e.radius*e.radius;if(i>r)return null;let o=Math.sqrt(r-i),a=n-o,l=n+o;return l<0?null:a<0?this.at(l,t):this.at(a,t)}intersectsSphere(e){return e.radius<0?!1:this.distanceSqToPoint(e.center)<=e.radius*e.radius}distanceToPlane(e){let t=e.normal.dot(this.direction);if(t===0)return e.distanceToPoint(this.origin)===0?0:null;let n=-(this.origin.dot(e.normal)+e.constant)/t;return n>=0?n:null}intersectPlane(e,t){let n=this.distanceToPlane(e);return n===null?null:this.at(n,t)}intersectsPlane(e){let t=e.distanceToPoint(this.origin);return t===0||e.normal.dot(this.direction)*t<0}intersectBox(e,t){let n,i,r,o,a,l,c=1/this.direction.x,h=1/this.direction.y,u=1/this.direction.z,d=this.origin;return c>=0?(n=(e.min.x-d.x)*c,i=(e.max.x-d.x)*c):(n=(e.max.x-d.x)*c,i=(e.min.x-d.x)*c),h>=0?(r=(e.min.y-d.y)*h,o=(e.max.y-d.y)*h):(r=(e.max.y-d.y)*h,o=(e.min.y-d.y)*h),n>o||r>i||((r>n||isNaN(n))&&(n=r),(o<i||isNaN(i))&&(i=o),u>=0?(a=(e.min.z-d.z)*u,l=(e.max.z-d.z)*u):(a=(e.max.z-d.z)*u,l=(e.min.z-d.z)*u),n>l||a>i)||((a>n||n!==n)&&(n=a),(l<i||i!==i)&&(i=l),i<0)?null:this.at(n>=0?n:i,t)}intersectsBox(e){return this.intersectBox(e,Pi)!==null}intersectTriangle(e,t,n,i,r){Vc.subVectors(t,e),ca.subVectors(n,e),Gc.crossVectors(Vc,ca);let o=this.direction.dot(Gc),a;if(o>0){if(i)return null;a=1}else if(o<0)a=-1,o=-o;else return null;ji.subVectors(this.origin,e);let l=a*this.direction.dot(ca.crossVectors(ji,ca));if(l<0)return null;let c=a*this.direction.dot(Vc.cross(ji));if(c<0||l+c>o)return null;let h=-a*ji.dot(Gc);return h<0?null:this.at(h/o,r)}applyMatrix4(e){return this.origin.applyMatrix4(e),this.direction.transformDirection(e),this}equals(e){return e.origin.equals(this.origin)&&e.direction.equals(this.direction)}clone(){return new this.constructor().copy(this)}},bt=class extends xn{constructor(e){super(),this.isMeshBasicMaterial=!0,this.type="MeshBasicMaterial",this.color=new xe(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Gn,this.combine=hl,this.reflectivity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.fog=e.fog,this}},Ku=new Qe,xs=new fi,ha=new dn,ju=new B,ua=new B,da=new B,fa=new B,Wc=new B,pa=new B,Ju=new B,ma=new B,Ke=class extends St{constructor(e=new pt,t=new bt){super(),this.isMesh=!0,this.type="Mesh",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.count=1,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),e.morphTargetInfluences!==void 0&&(this.morphTargetInfluences=e.morphTargetInfluences.slice()),e.morphTargetDictionary!==void 0&&(this.morphTargetDictionary=Object.assign({},e.morphTargetDictionary)),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}getVertexPosition(e,t){let n=this.geometry,i=n.attributes.position,r=n.morphAttributes.position,o=n.morphTargetsRelative;t.fromBufferAttribute(i,e);let a=this.morphTargetInfluences;if(r&&a){pa.set(0,0,0);for(let l=0,c=r.length;l<c;l++){let h=a[l],u=r[l];h!==0&&(Wc.fromBufferAttribute(u,e),o?pa.addScaledVector(Wc,h):pa.addScaledVector(Wc.sub(t),h))}t.add(pa)}return t}raycast(e,t){let n=this.geometry,i=this.material,r=this.matrixWorld;i!==void 0&&(n.boundingSphere===null&&n.computeBoundingSphere(),ha.copy(n.boundingSphere),ha.applyMatrix4(r),xs.copy(e.ray).recast(e.near),!(ha.containsPoint(xs.origin)===!1&&(xs.intersectSphere(ha,ju)===null||xs.origin.distanceToSquared(ju)>(e.far-e.near)**2))&&(Ku.copy(r).invert(),xs.copy(e.ray).applyMatrix4(Ku),!(n.boundingBox!==null&&xs.intersectsBox(n.boundingBox)===!1)&&this._computeIntersections(e,t,xs)))}_computeIntersections(e,t,n){let i,r=this.geometry,o=this.material,a=r.index,l=r.attributes.position,c=r.attributes.uv,h=r.attributes.uv1,u=r.attributes.normal,d=r.groups,f=r.drawRange;if(a!==null)if(Array.isArray(o))for(let g=0,y=d.length;g<y;g++){let m=d[g],p=o[m.materialIndex],b=Math.max(m.start,f.start),S=Math.min(a.count,Math.min(m.start+m.count,f.start+f.count));for(let x=b,A=S;x<A;x+=3){let T=a.getX(x),I=a.getX(x+1),v=a.getX(x+2);i=ga(this,p,e,n,c,h,u,T,I,v),i&&(i.faceIndex=Math.floor(x/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),y=Math.min(a.count,f.start+f.count);for(let m=g,p=y;m<p;m+=3){let b=a.getX(m),S=a.getX(m+1),x=a.getX(m+2);i=ga(this,o,e,n,c,h,u,b,S,x),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}else if(l!==void 0)if(Array.isArray(o))for(let g=0,y=d.length;g<y;g++){let m=d[g],p=o[m.materialIndex],b=Math.max(m.start,f.start),S=Math.min(l.count,Math.min(m.start+m.count,f.start+f.count));for(let x=b,A=S;x<A;x+=3){let T=x,I=x+1,v=x+2;i=ga(this,p,e,n,c,h,u,T,I,v),i&&(i.faceIndex=Math.floor(x/3),i.face.materialIndex=m.materialIndex,t.push(i))}}else{let g=Math.max(0,f.start),y=Math.min(l.count,f.start+f.count);for(let m=g,p=y;m<p;m+=3){let b=m,S=m+1,x=m+2;i=ga(this,o,e,n,c,h,u,b,S,x),i&&(i.faceIndex=Math.floor(m/3),t.push(i))}}}};function pm(s,e,t,n,i,r,o,a){let l;if(e.side===tn?l=n.intersectTriangle(o,r,i,!0,a):l=n.intersectTriangle(i,r,o,e.side===Ln,a),l===null)return null;ma.copy(a),ma.applyMatrix4(s.matrixWorld);let c=t.ray.origin.distanceTo(ma);return c<t.near||c>t.far?null:{distance:c,point:ma.clone(),object:s}}function ga(s,e,t,n,i,r,o,a,l,c){s.getVertexPosition(a,ua),s.getVertexPosition(l,da),s.getVertexPosition(c,fa);let h=pm(s,e,t,n,ua,da,fa,Ju);if(h){let u=new B;$i.getBarycoord(Ju,ua,da,fa,u),i&&(h.uv=$i.getInterpolatedAttribute(i,a,l,c,u,new _e)),r&&(h.uv1=$i.getInterpolatedAttribute(r,a,l,c,u,new _e)),o&&(h.normal=$i.getInterpolatedAttribute(o,a,l,c,u,new B),h.normal.dot(n.direction)>0&&h.normal.multiplyScalar(-1));let d={a,b:l,c,normal:new B,materialIndex:0};$i.getNormal(ua,da,fa,d.normal),h.face=d,h.barycoord=u}return h}var Xr=new _t,$u=new _t,Qu=new _t,mm=new _t,ed=new Qe,xa=new B,Xc=new dn,td=new Qe,qc=new fi,oo=class extends Ke{constructor(e,t){super(e,t),this.isSkinnedMesh=!0,this.type="SkinnedMesh",this.bindMode=eh,this.bindMatrix=new Qe,this.bindMatrixInverse=new Qe,this.boundingBox=null,this.boundingSphere=null}computeBoundingBox(){let e=this.geometry;this.boundingBox===null&&(this.boundingBox=new en),this.boundingBox.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,xa),this.boundingBox.expandByPoint(xa)}computeBoundingSphere(){let e=this.geometry;this.boundingSphere===null&&(this.boundingSphere=new dn),this.boundingSphere.makeEmpty();let t=e.getAttribute("position");for(let n=0;n<t.count;n++)this.getVertexPosition(n,xa),this.boundingSphere.expandByPoint(xa)}copy(e,t){return super.copy(e,t),this.bindMode=e.bindMode,this.bindMatrix.copy(e.bindMatrix),this.bindMatrixInverse.copy(e.bindMatrixInverse),this.skeleton=e.skeleton,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}raycast(e,t){let n=this.material,i=this.matrixWorld;n!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Xc.copy(this.boundingSphere),Xc.applyMatrix4(i),e.ray.intersectsSphere(Xc)!==!1&&(td.copy(i).invert(),qc.copy(e.ray).applyMatrix4(td),!(this.boundingBox!==null&&qc.intersectsBox(this.boundingBox)===!1)&&this._computeIntersections(e,t,qc)))}getVertexPosition(e,t){return super.getVertexPosition(e,t),this.applyBoneTransform(e,t),t}bind(e,t){this.skeleton=e,t===void 0&&(this.updateMatrixWorld(!0),this.skeleton.calculateInverses(),t=this.matrixWorld),this.bindMatrix.copy(t),this.bindMatrixInverse.copy(t).invert()}pose(){this.skeleton.pose()}normalizeSkinWeights(){let e=new _t,t=this.geometry.attributes.skinWeight;for(let n=0,i=t.count;n<i;n++){e.fromBufferAttribute(t,n);let r=1/e.manhattanLength();r!==1/0?e.multiplyScalar(r):e.set(1,0,0,0),t.setXYZW(n,e.x,e.y,e.z,e.w)}}updateMatrixWorld(e){super.updateMatrixWorld(e),this.bindMode===eh?this.bindMatrixInverse.copy(this.matrixWorld).invert():this.bindMode===qd?this.bindMatrixInverse.copy(this.bindMatrix).invert():We("SkinnedMesh: Unrecognized bindMode: "+this.bindMode)}applyBoneTransform(e,t){let n=this.skeleton,i=this.geometry;$u.fromBufferAttribute(i.attributes.skinIndex,e),Qu.fromBufferAttribute(i.attributes.skinWeight,e),t.isVector4?(Xr.copy(t),t.set(0,0,0,0)):(Xr.set(...t,1),t.set(0,0,0)),Xr.applyMatrix4(this.bindMatrix);for(let r=0;r<4;r++){let o=Qu.getComponent(r);if(o!==0){let a=$u.getComponent(r);ed.multiplyMatrices(n.bones[a].matrixWorld,n.boneInverses[a]),t.addScaledVector(mm.copy(Xr).applyMatrix4(ed),o)}}return t.isVector4&&(t.w=Xr.w),t.applyMatrix4(this.bindMatrixInverse)}},gr=class extends St{constructor(){super(),this.isBone=!0,this.type="Bone"}},es=class extends jt{constructor(e=null,t=1,n=1,i,r,o,a,l,c=Kt,h=Kt,u,d){super(null,o,a,l,c,h,i,r,u,d),this.isDataTexture=!0,this.image={data:e,width:t,height:n},this.generateMipmaps=!1,this.flipY=!1,this.unpackAlignment=1}},nd=new Qe,gm=new Qe,ao=class s{constructor(e=[],t=[]){this.uuid=ei(),this.bones=e.slice(0),this.boneInverses=t,this.boneMatrices=null,this.boneTexture=null,this.init()}init(){let e=this.bones,t=this.boneInverses;if(this.boneMatrices=new Float32Array(e.length*16),t.length===0)this.calculateInverses();else if(e.length!==t.length){We("Skeleton: Number of inverse bone matrices does not match amount of bones."),this.boneInverses=[];for(let n=0,i=this.bones.length;n<i;n++)this.boneInverses.push(new Qe)}}calculateInverses(){this.boneInverses.length=0;for(let e=0,t=this.bones.length;e<t;e++){let n=new Qe;this.bones[e]&&n.copy(this.bones[e].matrixWorld).invert(),this.boneInverses.push(n)}}pose(){for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&n.matrixWorld.copy(this.boneInverses[e]).invert()}for(let e=0,t=this.bones.length;e<t;e++){let n=this.bones[e];n&&(n.parent&&n.parent.isBone?(n.matrix.copy(n.parent.matrixWorld).invert(),n.matrix.multiply(n.matrixWorld)):n.matrix.copy(n.matrixWorld),n.matrix.decompose(n.position,n.quaternion,n.scale))}}update(){let e=this.bones,t=this.boneInverses,n=this.boneMatrices,i=this.boneTexture;for(let r=0,o=e.length;r<o;r++){let a=e[r]?e[r].matrixWorld:gm;nd.multiplyMatrices(a,t[r]),nd.toArray(n,r*16)}i!==null&&(i.needsUpdate=!0)}clone(){return new s(this.bones,this.boneInverses)}computeBoneTexture(){let e=Math.sqrt(this.bones.length*4);e=Math.ceil(e/4)*4,e=Math.max(e,4);let t=new Float32Array(e*e*4);t.set(this.boneMatrices);let n=new es(t,e,e,Fn,Un);return n.needsUpdate=!0,this.boneMatrices=t,this.boneTexture=n,this}getBoneByName(e){for(let t=0,n=this.bones.length;t<n;t++){let i=this.bones[t];if(i.name===e)return i}}dispose(){this.boneTexture!==null&&(this.boneTexture.dispose(),this.boneTexture=null)}fromJSON(e,t){this.uuid=e.uuid;for(let n=0,i=e.bones.length;n<i;n++){let r=e.bones[n],o=t[r];o===void 0&&(We("Skeleton: No bone found with UUID:",r),o=new gr),this.bones.push(o),this.boneInverses.push(new Qe().fromArray(e.boneInverses[n]))}return this.init(),this}toJSON(){let e={metadata:{version:4.7,type:"Skeleton",generator:"Skeleton.toJSON"},bones:[],boneInverses:[]};e.uuid=this.uuid;let t=this.bones,n=this.boneInverses;for(let i=0,r=t.length;i<r;i++){let o=t[i];e.bones.push(o.uuid);let a=n[i];e.boneInverses.push(a.toArray())}return e}},ts=class extends xt{constructor(e,t,n,i=1){super(e,t,n),this.isInstancedBufferAttribute=!0,this.meshPerAttribute=i}copy(e){return super.copy(e),this.meshPerAttribute=e.meshPerAttribute,this}toJSON(){let e=super.toJSON();return e.meshPerAttribute=this.meshPerAttribute,e.isInstancedBufferAttribute=!0,e}},nr=new Qe,id=new Qe,_a=[],sd=new en,xm=new Qe,qr=new Ke,Yr=new dn,vn=class extends Ke{constructor(e,t,n){super(e,t),this.isInstancedMesh=!0,this.instanceMatrix=new ts(new Float32Array(n*16),16),this.instanceColor=null,this.morphTexture=null,this.count=n,this.boundingBox=null,this.boundingSphere=null;for(let i=0;i<n;i++)this.setMatrixAt(i,xm)}computeBoundingBox(){let e=this.geometry,t=this.count;this.boundingBox===null&&(this.boundingBox=new en),e.boundingBox===null&&e.computeBoundingBox(),this.boundingBox.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,nr),sd.copy(e.boundingBox).applyMatrix4(nr),this.boundingBox.union(sd)}computeBoundingSphere(){let e=this.geometry,t=this.count;this.boundingSphere===null&&(this.boundingSphere=new dn),e.boundingSphere===null&&e.computeBoundingSphere(),this.boundingSphere.makeEmpty();for(let n=0;n<t;n++)this.getMatrixAt(n,nr),Yr.copy(e.boundingSphere).applyMatrix4(nr),this.boundingSphere.union(Yr)}copy(e,t){return super.copy(e,t),this.instanceMatrix.copy(e.instanceMatrix),e.morphTexture!==null&&(this.morphTexture=e.morphTexture.clone()),e.instanceColor!==null&&(this.instanceColor=e.instanceColor.clone()),this.count=e.count,e.boundingBox!==null&&(this.boundingBox=e.boundingBox.clone()),e.boundingSphere!==null&&(this.boundingSphere=e.boundingSphere.clone()),this}getColorAt(e,t){return this.instanceColor===null?t.setRGB(1,1,1):t.fromArray(this.instanceColor.array,e*3)}getMatrixAt(e,t){return t.fromArray(this.instanceMatrix.array,e*16)}getMorphAt(e,t){let n=t.morphTargetInfluences,i=this.morphTexture.source.data.data,r=n.length+1,o=e*r+1;for(let a=0;a<n.length;a++)n[a]=i[o+a]}raycast(e,t){let n=this.matrixWorld,i=this.count;if(qr.geometry=this.geometry,qr.material=this.material,qr.material!==void 0&&(this.boundingSphere===null&&this.computeBoundingSphere(),Yr.copy(this.boundingSphere),Yr.applyMatrix4(n),e.ray.intersectsSphere(Yr)!==!1))for(let r=0;r<i;r++){this.getMatrixAt(r,nr),id.multiplyMatrices(n,nr),qr.matrixWorld=id,qr.raycast(e,_a);for(let o=0,a=_a.length;o<a;o++){let l=_a[o];l.instanceId=r,l.object=this,t.push(l)}_a.length=0}}setColorAt(e,t){return this.instanceColor===null&&(this.instanceColor=new ts(new Float32Array(this.instanceMatrix.count*3).fill(1),3)),t.toArray(this.instanceColor.array,e*3),this}setMatrixAt(e,t){return t.toArray(this.instanceMatrix.array,e*16),this}setMorphAt(e,t){let n=t.morphTargetInfluences,i=n.length+1;this.morphTexture===null&&(this.morphTexture=new es(new Float32Array(i*this.count),i,this.count,xl,Un));let r=this.morphTexture.source.data.data,o=0;for(let c=0;c<n.length;c++)o+=n[c];let a=this.geometry.morphTargetsRelative?1:1-o,l=i*e;return r[l]=a,r.set(n,l+1),this}updateMorphTargets(){}dispose(){this.dispatchEvent({type:"dispose"}),this.morphTexture!==null&&(this.morphTexture.dispose(),this.morphTexture=null)}},Yc=new B,_m=new B,vm=new et,kn=class{constructor(e=new B(1,0,0),t=0){this.isPlane=!0,this.normal=e,this.constant=t}set(e,t){return this.normal.copy(e),this.constant=t,this}setComponents(e,t,n,i){return this.normal.set(e,t,n),this.constant=i,this}setFromNormalAndCoplanarPoint(e,t){return this.normal.copy(e),this.constant=-t.dot(this.normal),this}setFromCoplanarPoints(e,t,n){let i=Yc.subVectors(n,t).cross(_m.subVectors(e,t)).normalize();return this.setFromNormalAndCoplanarPoint(i,e),this}copy(e){return this.normal.copy(e.normal),this.constant=e.constant,this}normalize(){let e=1/this.normal.length();return this.normal.multiplyScalar(e),this.constant*=e,this}negate(){return this.constant*=-1,this.normal.negate(),this}distanceToPoint(e){return this.normal.dot(e)+this.constant}distanceToSphere(e){return this.distanceToPoint(e.center)-e.radius}projectPoint(e,t){return t.copy(e).addScaledVector(this.normal,-this.distanceToPoint(e))}intersectLine(e,t,n=!0){let i=e.delta(Yc),r=this.normal.dot(i);if(r===0)return this.distanceToPoint(e.start)===0?t.copy(e.start):null;let o=-(e.start.dot(this.normal)+this.constant)/r;return n===!0&&(o<0||o>1)?null:t.copy(e.start).addScaledVector(i,o)}intersectsLine(e){let t=this.distanceToPoint(e.start),n=this.distanceToPoint(e.end);return t<0&&n>0||n<0&&t>0}intersectsBox(e){return e.intersectsPlane(this)}intersectsSphere(e){return e.intersectsPlane(this)}coplanarPoint(e){return e.copy(this.normal).multiplyScalar(-this.constant)}applyMatrix4(e,t){let n=t||vm.getNormalMatrix(e),i=this.coplanarPoint(Yc).applyMatrix4(e),r=this.normal.applyMatrix3(n).normalize();return this.constant=-i.dot(r),this}translate(e){return this.constant-=e.dot(this.normal),this}equals(e){return e.normal.equals(this.normal)&&e.constant===this.constant}clone(){return new this.constructor().copy(this)}},_s=new dn,ym=new _e(.5,.5),va=new B,xr=class{constructor(e=new kn,t=new kn,n=new kn,i=new kn,r=new kn,o=new kn){this.planes=[e,t,n,i,r,o]}set(e,t,n,i,r,o){let a=this.planes;return a[0].copy(e),a[1].copy(t),a[2].copy(n),a[3].copy(i),a[4].copy(r),a[5].copy(o),this}copy(e){let t=this.planes;for(let n=0;n<6;n++)t[n].copy(e.planes[n]);return this}setFromProjectionMatrix(e,t=Qn,n=!1){let i=this.planes,r=e.elements,o=r[0],a=r[1],l=r[2],c=r[3],h=r[4],u=r[5],d=r[6],f=r[7],g=r[8],y=r[9],m=r[10],p=r[11],b=r[12],S=r[13],x=r[14],A=r[15];if(i[0].setComponents(c-o,f-h,p-g,A-b).normalize(),i[1].setComponents(c+o,f+h,p+g,A+b).normalize(),i[2].setComponents(c+a,f+u,p+y,A+S).normalize(),i[3].setComponents(c-a,f-u,p-y,A-S).normalize(),n)i[4].setComponents(l,d,m,x).normalize(),i[5].setComponents(c-l,f-d,p-m,A-x).normalize();else if(i[4].setComponents(c-l,f-d,p-m,A-x).normalize(),t===Qn)i[5].setComponents(c+l,f+d,p+m,A+x).normalize();else if(t===lr)i[5].setComponents(l,d,m,x).normalize();else throw new Error("THREE.Frustum.setFromProjectionMatrix(): Invalid coordinate system: "+t);return this}intersectsObject(e){if(e.boundingSphere!==void 0)e.boundingSphere===null&&e.computeBoundingSphere(),_s.copy(e.boundingSphere).applyMatrix4(e.matrixWorld);else{let t=e.geometry;t.boundingSphere===null&&t.computeBoundingSphere(),_s.copy(t.boundingSphere).applyMatrix4(e.matrixWorld)}return this.intersectsSphere(_s)}intersectsSprite(e){_s.center.set(0,0,0);let t=ym.distanceTo(e.center);return _s.radius=.7071067811865476+t,_s.applyMatrix4(e.matrixWorld),this.intersectsSphere(_s)}intersectsSphere(e){let t=this.planes,n=e.center,i=-e.radius;for(let r=0;r<6;r++)if(t[r].distanceToPoint(n)<i)return!1;return!0}intersectsBox(e){let t=this.planes;for(let n=0;n<6;n++){let i=t[n];if(va.x=i.normal.x>0?e.max.x:e.min.x,va.y=i.normal.y>0?e.max.y:e.min.y,va.z=i.normal.z>0?e.max.z:e.min.z,i.distanceToPoint(va)<0)return!1}return!0}containsPoint(e){let t=this.planes;for(let n=0;n<6;n++)if(t[n].distanceToPoint(e)<0)return!1;return!0}clone(){return new this.constructor().copy(this)}};var pi=class extends xn{constructor(e){super(),this.isLineBasicMaterial=!0,this.type="LineBasicMaterial",this.color=new xe(16777215),this.map=null,this.linewidth=1,this.linecap="round",this.linejoin="round",this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.linewidth=e.linewidth,this.linecap=e.linecap,this.linejoin=e.linejoin,this.fog=e.fog,this}},Ha=new B,Va=new B,rd=new Qe,Zr=new fi,ya=new dn,Zc=new B,od=new B,Di=class extends St{constructor(e=new pt,t=new pi){super(),this.isLine=!0,this.type="Line",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[0];for(let i=1,r=t.count;i<r;i++)Ha.fromBufferAttribute(t,i-1),Va.fromBufferAttribute(t,i),n[i]=n[i-1],n[i]+=Ha.distanceTo(Va);e.setAttribute("lineDistance",new nt(n,1))}else We("Line.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Line.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),ya.copy(n.boundingSphere),ya.applyMatrix4(i),ya.radius+=r,e.ray.intersectsSphere(ya)===!1)return;rd.copy(i).invert(),Zr.copy(e.ray).applyMatrix4(rd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=this.isLineSegments?2:1,h=n.index,d=n.attributes.position;if(h!==null){let f=Math.max(0,o.start),g=Math.min(h.count,o.start+o.count);for(let y=f,m=g-1;y<m;y+=c){let p=h.getX(y),b=h.getX(y+1),S=Ma(this,e,Zr,l,p,b,y);S&&t.push(S)}if(this.isLineLoop){let y=h.getX(g-1),m=h.getX(f),p=Ma(this,e,Zr,l,y,m,g-1);p&&t.push(p)}}else{let f=Math.max(0,o.start),g=Math.min(d.count,o.start+o.count);for(let y=f,m=g-1;y<m;y+=c){let p=Ma(this,e,Zr,l,y,y+1,y);p&&t.push(p)}if(this.isLineLoop){let y=Ma(this,e,Zr,l,g-1,f,g-1);y&&t.push(y)}}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function Ma(s,e,t,n,i,r,o){let a=s.geometry.attributes.position;if(Ha.fromBufferAttribute(a,i),Va.fromBufferAttribute(a,r),t.distanceSqToSegment(Ha,Va,Zc,od)>n)return;Zc.applyMatrix4(s.matrixWorld);let c=e.ray.origin.distanceTo(Zc);if(!(c<e.near||c>e.far))return{distance:c,point:od.clone().applyMatrix4(s.matrixWorld),index:o,face:null,faceIndex:null,barycoord:null,object:s}}var ad=new B,ld=new B,Cs=class extends Di{constructor(e,t){super(e,t),this.isLineSegments=!0,this.type="LineSegments"}computeLineDistances(){let e=this.geometry;if(e.index===null){let t=e.attributes.position,n=[];for(let i=0,r=t.count;i<r;i+=2)ad.fromBufferAttribute(t,i),ld.fromBufferAttribute(t,i+1),n[i]=i===0?0:n[i-1],n[i+1]=n[i]+ad.distanceTo(ld);e.setAttribute("lineDistance",new nt(n,1))}else We("LineSegments.computeLineDistances(): Computation only possible with non-indexed BufferGeometry.");return this}},lo=class extends Di{constructor(e,t){super(e,t),this.isLineLoop=!0,this.type="LineLoop"}},_r=class extends xn{constructor(e){super(),this.isPointsMaterial=!0,this.type="PointsMaterial",this.color=new xe(16777215),this.map=null,this.alphaMap=null,this.size=1,this.sizeAttenuation=!0,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.alphaMap=e.alphaMap,this.size=e.size,this.sizeAttenuation=e.sizeAttenuation,this.fog=e.fog,this}},cd=new Qe,ih=new fi,ba=new dn,Sa=new B,_n=class extends St{constructor(e=new pt,t=new _r){super(),this.isPoints=!0,this.type="Points",this.geometry=e,this.material=t,this.morphTargetDictionary=void 0,this.morphTargetInfluences=void 0,this.updateMorphTargets()}copy(e,t){return super.copy(e,t),this.material=Array.isArray(e.material)?e.material.slice():e.material,this.geometry=e.geometry,this}raycast(e,t){let n=this.geometry,i=this.matrixWorld,r=e.params.Points.threshold,o=n.drawRange;if(n.boundingSphere===null&&n.computeBoundingSphere(),ba.copy(n.boundingSphere),ba.applyMatrix4(i),ba.radius+=r,e.ray.intersectsSphere(ba)===!1)return;cd.copy(i).invert(),ih.copy(e.ray).applyMatrix4(cd);let a=r/((this.scale.x+this.scale.y+this.scale.z)/3),l=a*a,c=n.index,u=n.attributes.position;if(c!==null){let d=Math.max(0,o.start),f=Math.min(c.count,o.start+o.count);for(let g=d,y=f;g<y;g++){let m=c.getX(g);Sa.fromBufferAttribute(u,m),hd(Sa,m,l,i,e,t,this)}}else{let d=Math.max(0,o.start),f=Math.min(u.count,o.start+o.count);for(let g=d,y=f;g<y;g++)Sa.fromBufferAttribute(u,g),hd(Sa,g,l,i,e,t,this)}}updateMorphTargets(){let t=this.geometry.morphAttributes,n=Object.keys(t);if(n.length>0){let i=t[n[0]];if(i!==void 0){this.morphTargetInfluences=[],this.morphTargetDictionary={};for(let r=0,o=i.length;r<o;r++){let a=i[r].name||String(r);this.morphTargetInfluences.push(0),this.morphTargetDictionary[a]=r}}}}};function hd(s,e,t,n,i,r,o){let a=ih.distanceSqToPoint(s);if(a<t){let l=new B;ih.closestPointToPoint(s,l),l.applyMatrix4(n);let c=i.ray.origin.distanceTo(l);if(c<i.near||c>i.far)return;r.push({distance:c,distanceToRay:Math.sqrt(a),point:l,index:e,face:null,faceIndex:null,barycoord:null,object:o})}}var co=class extends jt{constructor(e=[],t=as,n,i,r,o,a,l,c,h){super(e,t,n,i,r,o,a,l,c,h),this.isCubeTexture=!0,this.flipY=!1}get images(){return this.image}set images(e){this.image=e}},Ps=class extends jt{constructor(e,t,n,i,r,o,a,l,c){super(e,t,n,i,r,o,a,l,c),this.isCanvasTexture=!0,this.needsUpdate=!0}};var Ni=class extends jt{constructor(e,t,n=ri,i,r,o,a=Kt,l=Kt,c,h=di,u=1){if(h!==di&&h!==ls)throw new Error("THREE.DepthTexture: format must be either THREE.DepthFormat or THREE.DepthStencilFormat");let d={width:e,height:t,depth:u};super(d,i,r,o,a,l,h,n,c),this.isDepthTexture=!0,this.flipY=!1,this.generateMipmaps=!1,this.compareFunction=null}copy(e){return super.copy(e),this.source=new ur(Object.assign({},e.image)),this.compareFunction=e.compareFunction,this}toJSON(e){let t=super.toJSON(e);return this.compareFunction!==null&&(t.compareFunction=this.compareFunction),t}},Ga=class extends Ni{constructor(e,t=ri,n=as,i,r,o=Kt,a=Kt,l,c=di){let h={width:e,height:e,depth:1},u=[h,h,h,h,h,h];super(e,e,t,n,i,r,o,a,l,c),this.image=u,this.isCubeDepthTexture=!0,this.isCubeTexture=!0}get images(){return this.image}set images(e){this.image=e}},ho=class extends jt{constructor(e=null){super(),this.sourceTexture=e,this.isExternalTexture=!0}copy(e){return super.copy(e),this.sourceTexture=e.sourceTexture,this}},mi=class s extends pt{constructor(e=1,t=1,n=1,i=1,r=1,o=1){super(),this.type="BoxGeometry",this.parameters={width:e,height:t,depth:n,widthSegments:i,heightSegments:r,depthSegments:o};let a=this;i=Math.floor(i),r=Math.floor(r),o=Math.floor(o);let l=[],c=[],h=[],u=[],d=0,f=0;g("z","y","x",-1,-1,n,t,e,o,r,0),g("z","y","x",1,-1,n,t,-e,o,r,1),g("x","z","y",1,1,e,n,t,i,o,2),g("x","z","y",1,-1,e,n,-t,i,o,3),g("x","y","z",1,-1,e,t,n,i,r,4),g("x","y","z",-1,-1,e,t,-n,i,r,5),this.setIndex(l),this.setAttribute("position",new nt(c,3)),this.setAttribute("normal",new nt(h,3)),this.setAttribute("uv",new nt(u,2));function g(y,m,p,b,S,x,A,T,I,v,D){let w=x/I,R=A/v,L=x/2,V=A/2,C=T/2,N=I+1,U=v+1,E=0,G=0,Y=new B;for(let X=0;X<U;X++){let ie=X*R-V;for(let ce=0;ce<N;ce++){let be=ce*w-L;Y[y]=be*b,Y[m]=ie*S,Y[p]=C,c.push(Y.x,Y.y,Y.z),Y[y]=0,Y[m]=0,Y[p]=T>0?1:-1,h.push(Y.x,Y.y,Y.z),u.push(ce/I),u.push(1-X/v),E+=1}}for(let X=0;X<v;X++)for(let ie=0;ie<I;ie++){let ce=d+ie+N*X,be=d+ie+N*(X+1),Le=d+(ie+1)+N*(X+1),Re=d+(ie+1)+N*X;l.push(ce,be,Re),l.push(be,Le,Re),G+=6}a.addGroup(f,G,D),f+=G,d+=E}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.depth,e.widthSegments,e.heightSegments,e.depthSegments)}};var vr=class s extends pt{constructor(e=1,t=32,n=0,i=Math.PI*2){super(),this.type="CircleGeometry",this.parameters={radius:e,segments:t,thetaStart:n,thetaLength:i},t=Math.max(3,t);let r=[],o=[],a=[],l=[],c=new B,h=new _e;o.push(0,0,0),a.push(0,0,1),l.push(.5,.5);for(let u=0,d=3;u<=t;u++,d+=3){let f=n+u/t*i;c.x=e*Math.cos(f),c.y=e*Math.sin(f),o.push(c.x,c.y,c.z),a.push(0,0,1),h.x=(o[d]/e+1)/2,h.y=(o[d+1]/e+1)/2,l.push(h.x,h.y)}for(let u=1;u<=t;u++)r.push(u,u+1,0);this.setIndex(r),this.setAttribute("position",new nt(o,3)),this.setAttribute("normal",new nt(a,3)),this.setAttribute("uv",new nt(l,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.segments,e.thetaStart,e.thetaLength)}},ns=class s extends pt{constructor(e=1,t=1,n=1,i=32,r=1,o=!1,a=0,l=Math.PI*2){super(),this.type="CylinderGeometry",this.parameters={radiusTop:e,radiusBottom:t,height:n,radialSegments:i,heightSegments:r,openEnded:o,thetaStart:a,thetaLength:l};let c=this;i=Math.floor(i),r=Math.floor(r);let h=[],u=[],d=[],f=[],g=0,y=[],m=n/2,p=0;b(),o===!1&&(e>0&&S(!0),t>0&&S(!1)),this.setIndex(h),this.setAttribute("position",new nt(u,3)),this.setAttribute("normal",new nt(d,3)),this.setAttribute("uv",new nt(f,2));function b(){let x=new B,A=new B,T=0,I=(t-e)/n;for(let v=0;v<=r;v++){let D=[],w=v/r,R=w*(t-e)+e;for(let L=0;L<=i;L++){let V=L/i,C=V*l+a,N=Math.sin(C),U=Math.cos(C);A.x=R*N,A.y=-w*n+m,A.z=R*U,u.push(A.x,A.y,A.z),x.set(N,I,U).normalize(),d.push(x.x,x.y,x.z),f.push(V,1-w),D.push(g++)}y.push(D)}for(let v=0;v<i;v++)for(let D=0;D<r;D++){let w=y[D][v],R=y[D+1][v],L=y[D+1][v+1],V=y[D][v+1];(e>0||D!==0)&&(h.push(w,R,V),T+=3),(t>0||D!==r-1)&&(h.push(R,L,V),T+=3)}c.addGroup(p,T,0),p+=T}function S(x){let A=g,T=new _e,I=new B,v=0,D=x===!0?e:t,w=x===!0?1:-1;for(let L=1;L<=i;L++)u.push(0,m*w,0),d.push(0,w,0),f.push(.5,.5),g++;let R=g;for(let L=0;L<=i;L++){let C=L/i*l+a,N=Math.cos(C),U=Math.sin(C);I.x=D*U,I.y=m*w,I.z=D*N,u.push(I.x,I.y,I.z),d.push(0,w,0),T.x=N*.5+.5,T.y=U*.5*w+.5,f.push(T.x,T.y),g++}for(let L=0;L<i;L++){let V=A+L,C=R+L;x===!0?h.push(C,C+1,V):h.push(C+1,C,V),v+=3}c.addGroup(p,v,x===!0?1:2),p+=v}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radiusTop,e.radiusBottom,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Is=class s extends ns{constructor(e=1,t=1,n=32,i=1,r=!1,o=0,a=Math.PI*2){super(0,e,t,n,i,r,o,a),this.type="ConeGeometry",this.parameters={radius:e,height:t,radialSegments:n,heightSegments:i,openEnded:r,thetaStart:o,thetaLength:a}}static fromJSON(e){return new s(e.radius,e.height,e.radialSegments,e.heightSegments,e.openEnded,e.thetaStart,e.thetaLength)}},Wa=class s extends pt{constructor(e=[],t=[],n=1,i=0){super(),this.type="PolyhedronGeometry",this.parameters={vertices:e,indices:t,radius:n,detail:i};let r=[],o=[];a(i),c(n),h(),this.setAttribute("position",new nt(r,3)),this.setAttribute("normal",new nt(r.slice(),3)),this.setAttribute("uv",new nt(o,2)),i===0?this.computeVertexNormals():this.normalizeNormals();function a(b){let S=new B,x=new B,A=new B;for(let T=0;T<t.length;T+=3)f(t[T+0],S),f(t[T+1],x),f(t[T+2],A),l(S,x,A,b)}function l(b,S,x,A){let T=A+1,I=[];for(let v=0;v<=T;v++){I[v]=[];let D=b.clone().lerp(x,v/T),w=S.clone().lerp(x,v/T),R=T-v;for(let L=0;L<=R;L++)L===0&&v===T?I[v][L]=D:I[v][L]=D.clone().lerp(w,L/R)}for(let v=0;v<T;v++)for(let D=0;D<2*(T-v)-1;D++){let w=Math.floor(D/2);D%2===0?(d(I[v][w+1]),d(I[v+1][w]),d(I[v][w])):(d(I[v][w+1]),d(I[v+1][w+1]),d(I[v+1][w]))}}function c(b){let S=new B;for(let x=0;x<r.length;x+=3)S.x=r[x+0],S.y=r[x+1],S.z=r[x+2],S.normalize().multiplyScalar(b),r[x+0]=S.x,r[x+1]=S.y,r[x+2]=S.z}function h(){let b=new B;for(let S=0;S<r.length;S+=3){b.x=r[S+0],b.y=r[S+1],b.z=r[S+2];let x=m(b)/2/Math.PI+.5,A=p(b)/Math.PI+.5;o.push(x,1-A)}g(),u()}function u(){for(let b=0;b<o.length;b+=6){let S=o[b+0],x=o[b+2],A=o[b+4],T=Math.max(S,x,A),I=Math.min(S,x,A);T>.9&&I<.1&&(S<.2&&(o[b+0]+=1),x<.2&&(o[b+2]+=1),A<.2&&(o[b+4]+=1))}}function d(b){r.push(b.x,b.y,b.z)}function f(b,S){let x=b*3;S.x=e[x+0],S.y=e[x+1],S.z=e[x+2]}function g(){let b=new B,S=new B,x=new B,A=new B,T=new _e,I=new _e,v=new _e;for(let D=0,w=0;D<r.length;D+=9,w+=6){b.set(r[D+0],r[D+1],r[D+2]),S.set(r[D+3],r[D+4],r[D+5]),x.set(r[D+6],r[D+7],r[D+8]),T.set(o[w+0],o[w+1]),I.set(o[w+2],o[w+3]),v.set(o[w+4],o[w+5]),A.copy(b).add(S).add(x).divideScalar(3);let R=m(A);y(T,w+0,b,R),y(I,w+2,S,R),y(v,w+4,x,R)}}function y(b,S,x,A){A<0&&b.x===1&&(o[S]=b.x-1),x.x===0&&x.z===0&&(o[S]=A/2/Math.PI+.5)}function m(b){return Math.atan2(b.z,-b.x)}function p(b){return Math.atan2(-b.y,Math.sqrt(b.x*b.x+b.z*b.z))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.vertices,e.indices,e.radius,e.detail)}};var Dn=class{constructor(){this.type="Curve",this.arcLengthDivisions=200,this.needsUpdate=!1,this.cacheArcLengths=null}getPoint(){We("Curve: .getPoint() not implemented.")}getPointAt(e,t){let n=this.getUtoTmapping(e);return this.getPoint(n,t)}getPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return t}getSpacedPoints(e=5){let t=[];for(let n=0;n<=e;n++)t.push(this.getPointAt(n/e));return t}getLength(){let e=this.getLengths();return e[e.length-1]}getLengths(e=this.arcLengthDivisions){if(this.cacheArcLengths&&this.cacheArcLengths.length===e+1&&!this.needsUpdate)return this.cacheArcLengths;this.needsUpdate=!1;let t=[],n,i=this.getPoint(0),r=0;t.push(0);for(let o=1;o<=e;o++)n=this.getPoint(o/e),r+=n.distanceTo(i),t.push(r),i=n;return this.cacheArcLengths=t,t}updateArcLengths(){this.needsUpdate=!0,this.getLengths()}getUtoTmapping(e,t=null){let n=this.getLengths(),i=0,r=n.length,o;t?o=t:o=e*n[r-1];let a=0,l=r-1,c;for(;a<=l;)if(i=Math.floor(a+(l-a)/2),c=n[i]-o,c<0)a=i+1;else if(c>0)l=i-1;else{l=i;break}if(i=l,n[i]===o)return i/(r-1);let h=n[i],d=n[i+1]-h,f=(o-h)/d;return(i+f)/(r-1)}getTangent(e,t){let i=e-1e-4,r=e+1e-4;i<0&&(i=0),r>1&&(r=1);let o=this.getPoint(i),a=this.getPoint(r),l=t||(o.isVector2?new _e:new B);return l.copy(a).sub(o).normalize(),l}getTangentAt(e,t){let n=this.getUtoTmapping(e);return this.getTangent(n,t)}computeFrenetFrames(e,t=!1){let n=new B,i=[],r=[],o=[],a=new B,l=new Qe;for(let f=0;f<=e;f++){let g=f/e;i[f]=this.getTangentAt(g,new B)}r[0]=new B,o[0]=new B;let c=Number.MAX_VALUE,h=Math.abs(i[0].x),u=Math.abs(i[0].y),d=Math.abs(i[0].z);h<=c&&(c=h,n.set(1,0,0)),u<=c&&(c=u,n.set(0,1,0)),d<=c&&n.set(0,0,1),a.crossVectors(i[0],n).normalize(),r[0].crossVectors(i[0],a),o[0].crossVectors(i[0],r[0]);for(let f=1;f<=e;f++){if(r[f]=r[f-1].clone(),o[f]=o[f-1].clone(),a.crossVectors(i[f-1],i[f]),a.length()>Number.EPSILON){a.normalize();let g=Math.acos(at(i[f-1].dot(i[f]),-1,1));r[f].applyMatrix4(l.makeRotationAxis(a,g))}o[f].crossVectors(i[f],r[f])}if(t===!0){let f=Math.acos(at(r[0].dot(r[e]),-1,1));f/=e,i[0].dot(a.crossVectors(r[0],r[e]))>0&&(f=-f);for(let g=1;g<=e;g++)r[g].applyMatrix4(l.makeRotationAxis(i[g],f*g)),o[g].crossVectors(i[g],r[g])}return{tangents:i,normals:r,binormals:o}}clone(){return new this.constructor().copy(this)}copy(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}toJSON(){let e={metadata:{version:4.7,type:"Curve",generator:"Curve.toJSON"}};return e.arcLengthDivisions=this.arcLengthDivisions,e.type=this.type,e}fromJSON(e){return this.arcLengthDivisions=e.arcLengthDivisions,this}},uo=class extends Dn{constructor(e=0,t=0,n=1,i=1,r=0,o=Math.PI*2,a=!1,l=0){super(),this.isEllipseCurve=!0,this.type="EllipseCurve",this.aX=e,this.aY=t,this.xRadius=n,this.yRadius=i,this.aStartAngle=r,this.aEndAngle=o,this.aClockwise=a,this.aRotation=l}getPoint(e,t=new _e){let n=t,i=Math.PI*2,r=this.aEndAngle-this.aStartAngle,o=Math.abs(r)<Number.EPSILON;for(;r<0;)r+=i;for(;r>i;)r-=i;r<Number.EPSILON&&(o?r=0:r=i),this.aClockwise===!0&&!o&&(r===i?r=-i:r=r-i);let a=this.aStartAngle+e*r,l=this.aX+this.xRadius*Math.cos(a),c=this.aY+this.yRadius*Math.sin(a);if(this.aRotation!==0){let h=Math.cos(this.aRotation),u=Math.sin(this.aRotation),d=l-this.aX,f=c-this.aY;l=d*h-f*u+this.aX,c=d*u+f*h+this.aY}return n.set(l,c)}copy(e){return super.copy(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}toJSON(){let e=super.toJSON();return e.aX=this.aX,e.aY=this.aY,e.xRadius=this.xRadius,e.yRadius=this.yRadius,e.aStartAngle=this.aStartAngle,e.aEndAngle=this.aEndAngle,e.aClockwise=this.aClockwise,e.aRotation=this.aRotation,e}fromJSON(e){return super.fromJSON(e),this.aX=e.aX,this.aY=e.aY,this.xRadius=e.xRadius,this.yRadius=e.yRadius,this.aStartAngle=e.aStartAngle,this.aEndAngle=e.aEndAngle,this.aClockwise=e.aClockwise,this.aRotation=e.aRotation,this}},Xa=class extends uo{constructor(e,t,n,i,r,o){super(e,t,n,n,i,r,o),this.isArcCurve=!0,this.type="ArcCurve"}};function Eh(){let s=0,e=0,t=0,n=0;function i(r,o,a,l){s=r,e=a,t=-3*r+3*o-2*a-l,n=2*r-2*o+a+l}return{initCatmullRom:function(r,o,a,l,c){i(o,a,c*(a-r),c*(l-o))},initNonuniformCatmullRom:function(r,o,a,l,c,h,u){let d=(o-r)/c-(a-r)/(c+h)+(a-o)/h,f=(a-o)/h-(l-o)/(h+u)+(l-a)/u;d*=h,f*=h,i(o,a,d,f)},calc:function(r){let o=r*r,a=o*r;return s+e*r+t*o+n*a}}}var ud=new B,dd=new B,Kc=new Eh,jc=new Eh,Jc=new Eh,yr=class extends Dn{constructor(e=[],t=!1,n="centripetal",i=.5){super(),this.isCatmullRomCurve3=!0,this.type="CatmullRomCurve3",this.points=e,this.closed=t,this.curveType=n,this.tension=i}getPoint(e,t=new B){let n=t,i=this.points,r=i.length,o=(r-(this.closed?0:1))*e,a=Math.floor(o),l=o-a;this.closed?a+=a>0?0:(Math.floor(Math.abs(a)/r)+1)*r:l===0&&a===r-1&&(a=r-2,l=1);let c,h;this.closed||a>0?c=i[(a-1)%r]:(dd.subVectors(i[0],i[1]).add(i[0]),c=dd);let u=i[a%r],d=i[(a+1)%r];if(this.closed||a+2<r?h=i[(a+2)%r]:(ud.subVectors(i[r-1],i[r-2]).add(i[r-1]),h=ud),this.curveType==="centripetal"||this.curveType==="chordal"){let f=this.curveType==="chordal"?.5:.25,g=Math.pow(c.distanceToSquared(u),f),y=Math.pow(u.distanceToSquared(d),f),m=Math.pow(d.distanceToSquared(h),f);y<1e-4&&(y=1),g<1e-4&&(g=y),m<1e-4&&(m=y),Kc.initNonuniformCatmullRom(c.x,u.x,d.x,h.x,g,y,m),jc.initNonuniformCatmullRom(c.y,u.y,d.y,h.y,g,y,m),Jc.initNonuniformCatmullRom(c.z,u.z,d.z,h.z,g,y,m)}else this.curveType==="catmullrom"&&(Kc.initCatmullRom(c.x,u.x,d.x,h.x,this.tension),jc.initCatmullRom(c.y,u.y,d.y,h.y,this.tension),Jc.initCatmullRom(c.z,u.z,d.z,h.z,this.tension));return n.set(Kc.calc(l),jc.calc(l),Jc.calc(l)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e.closed=this.closed,e.curveType=this.curveType,e.tension=this.tension,e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new B().fromArray(i))}return this.closed=e.closed,this.curveType=e.curveType,this.tension=e.tension,this}};function fd(s,e,t,n,i){let r=(n-e)*.5,o=(i-t)*.5,a=s*s,l=s*a;return(2*t-2*n+r+o)*l+(-3*t+3*n-2*r-o)*a+r*s+t}function Mm(s,e){let t=1-s;return t*t*e}function bm(s,e){return 2*(1-s)*s*e}function Sm(s,e){return s*s*e}function Jr(s,e,t,n){return Mm(s,e)+bm(s,t)+Sm(s,n)}function wm(s,e){let t=1-s;return t*t*t*e}function Em(s,e){let t=1-s;return 3*t*t*s*e}function Tm(s,e){return 3*(1-s)*s*s*e}function Am(s,e){return s*s*s*e}function $r(s,e,t,n,i){return wm(s,e)+Em(s,t)+Tm(s,n)+Am(s,i)}var qa=class extends Dn{constructor(e=new _e,t=new _e,n=new _e,i=new _e){super(),this.isCubicBezierCurve=!0,this.type="CubicBezierCurve",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new _e){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set($r(e,i.x,r.x,o.x,a.x),$r(e,i.y,r.y,o.y,a.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Ya=class extends Dn{constructor(e=new B,t=new B,n=new B,i=new B){super(),this.isCubicBezierCurve3=!0,this.type="CubicBezierCurve3",this.v0=e,this.v1=t,this.v2=n,this.v3=i}getPoint(e,t=new B){let n=t,i=this.v0,r=this.v1,o=this.v2,a=this.v3;return n.set($r(e,i.x,r.x,o.x,a.x),$r(e,i.y,r.y,o.y,a.y),$r(e,i.z,r.z,o.z,a.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this.v3.copy(e.v3),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e.v3=this.v3.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this.v3.fromArray(e.v3),this}},Za=class extends Dn{constructor(e=new _e,t=new _e){super(),this.isLineCurve=!0,this.type="LineCurve",this.v1=e,this.v2=t}getPoint(e,t=new _e){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new _e){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Mr=class extends Dn{constructor(e=new B,t=new B){super(),this.isLineCurve3=!0,this.type="LineCurve3",this.v1=e,this.v2=t}getPoint(e,t=new B){let n=t;return e===1?n.copy(this.v2):(n.copy(this.v2).sub(this.v1),n.multiplyScalar(e).add(this.v1)),n}getPointAt(e,t){return this.getPoint(e,t)}getTangent(e,t=new B){return t.subVectors(this.v2,this.v1).normalize()}getTangentAt(e,t){return this.getTangent(e,t)}copy(e){return super.copy(e),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},Ka=class extends Dn{constructor(e=new _e,t=new _e,n=new _e){super(),this.isQuadraticBezierCurve=!0,this.type="QuadraticBezierCurve",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new _e){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(Jr(e,i.x,r.x,o.x),Jr(e,i.y,r.y,o.y)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},ti=class extends Dn{constructor(e=new B,t=new B,n=new B){super(),this.isQuadraticBezierCurve3=!0,this.type="QuadraticBezierCurve3",this.v0=e,this.v1=t,this.v2=n}getPoint(e,t=new B){let n=t,i=this.v0,r=this.v1,o=this.v2;return n.set(Jr(e,i.x,r.x,o.x),Jr(e,i.y,r.y,o.y),Jr(e,i.z,r.z,o.z)),n}copy(e){return super.copy(e),this.v0.copy(e.v0),this.v1.copy(e.v1),this.v2.copy(e.v2),this}toJSON(){let e=super.toJSON();return e.v0=this.v0.toArray(),e.v1=this.v1.toArray(),e.v2=this.v2.toArray(),e}fromJSON(e){return super.fromJSON(e),this.v0.fromArray(e.v0),this.v1.fromArray(e.v1),this.v2.fromArray(e.v2),this}},ja=class extends Dn{constructor(e=[]){super(),this.isSplineCurve=!0,this.type="SplineCurve",this.points=e}getPoint(e,t=new _e){let n=t,i=this.points,r=(i.length-1)*e,o=Math.floor(r),a=r-o,l=i[o===0?o:o-1],c=i[o],h=i[o>i.length-2?i.length-1:o+1],u=i[o>i.length-3?i.length-1:o+2];return n.set(fd(a,l.x,c.x,h.x,u.x),fd(a,l.y,c.y,h.y,u.y)),n}copy(e){super.copy(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(i.clone())}return this}toJSON(){let e=super.toJSON();e.points=[];for(let t=0,n=this.points.length;t<n;t++){let i=this.points[t];e.points.push(i.toArray())}return e}fromJSON(e){super.fromJSON(e),this.points=[];for(let t=0,n=e.points.length;t<n;t++){let i=e.points[t];this.points.push(new _e().fromArray(i))}return this}},sh=Object.freeze({__proto__:null,ArcCurve:Xa,CatmullRomCurve3:yr,CubicBezierCurve:qa,CubicBezierCurve3:Ya,EllipseCurve:uo,LineCurve:Za,LineCurve3:Mr,QuadraticBezierCurve:Ka,QuadraticBezierCurve3:ti,SplineCurve:ja}),fo=class extends Dn{constructor(){super(),this.type="CurvePath",this.curves=[],this.autoClose=!1}add(e){this.curves.push(e)}closePath(){let e=this.curves[0].getPoint(0),t=this.curves[this.curves.length-1].getPoint(1);if(!e.equals(t)){let n=e.isVector2===!0?"LineCurve":"LineCurve3";this.curves.push(new sh[n](t,e))}return this}getPoint(e,t){let n=e*this.getLength(),i=this.getCurveLengths(),r=0;for(;r<i.length;){if(i[r]>=n){let o=i[r]-n,a=this.curves[r],l=a.getLength(),c=l===0?0:1-o/l;return a.getPointAt(c,t)}r++}return null}getLength(){let e=this.getCurveLengths();return e[e.length-1]}updateArcLengths(){this.needsUpdate=!0,this.cacheLengths=null,this.getCurveLengths()}getCurveLengths(){if(this.cacheLengths&&this.cacheLengths.length===this.curves.length)return this.cacheLengths;let e=[],t=0;for(let n=0,i=this.curves.length;n<i;n++)t+=this.curves[n].getLength(),e.push(t);return this.cacheLengths=e,e}getSpacedPoints(e=40){let t=[];for(let n=0;n<=e;n++)t.push(this.getPoint(n/e));return this.autoClose&&t.push(t[0]),t}getPoints(e=12){let t=[],n;for(let i=0,r=this.curves;i<r.length;i++){let o=r[i],a=o.isEllipseCurve?e*2:o.isLineCurve||o.isLineCurve3?1:o.isSplineCurve?e*o.points.length:e,l=o.getPoints(a);for(let c=0;c<l.length;c++){let h=l[c];n&&n.equals(h)||(t.push(h),n=h)}}return this.autoClose&&t.length>1&&!t[t.length-1].equals(t[0])&&t.push(t[0]),t}copy(e){super.copy(e),this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(i.clone())}return this.autoClose=e.autoClose,this}toJSON(){let e=super.toJSON();e.autoClose=this.autoClose,e.curves=[];for(let t=0,n=this.curves.length;t<n;t++){let i=this.curves[t];e.curves.push(i.toJSON())}return e}fromJSON(e){super.fromJSON(e),this.autoClose=e.autoClose,this.curves=[];for(let t=0,n=e.curves.length;t<n;t++){let i=e.curves[t];this.curves.push(new sh[i.type]().fromJSON(i))}return this}};var br=class s extends Wa{constructor(e=1,t=0){let n=(1+Math.sqrt(5))/2,i=[-1,n,0,1,n,0,-1,-n,0,1,-n,0,0,-1,n,0,1,n,0,-1,-n,0,1,-n,n,0,-1,n,0,1,-n,0,-1,-n,0,1],r=[0,11,5,0,5,1,0,1,7,0,7,10,0,10,11,1,5,9,5,11,4,11,10,2,10,7,6,7,1,8,3,9,4,3,4,2,3,2,6,3,6,8,3,8,9,4,9,5,2,4,11,6,2,10,8,6,7,9,8,1];super(i,r,e,t),this.type="IcosahedronGeometry",this.parameters={radius:e,detail:t}}static fromJSON(e){return new s(e.radius,e.detail)}};var ln=class s extends pt{constructor(e=1,t=1,n=1,i=1){super(),this.type="PlaneGeometry",this.parameters={width:e,height:t,widthSegments:n,heightSegments:i};let r=e/2,o=t/2,a=Math.floor(n),l=Math.floor(i),c=a+1,h=l+1,u=e/a,d=t/l,f=[],g=[],y=[],m=[];for(let p=0;p<h;p++){let b=p*d-o;for(let S=0;S<c;S++){let x=S*u-r;g.push(x,-b,0),y.push(0,0,1),m.push(S/a),m.push(1-p/l)}}for(let p=0;p<l;p++)for(let b=0;b<a;b++){let S=b+c*p,x=b+c*(p+1),A=b+1+c*(p+1),T=b+1+c*p;f.push(S,x,T),f.push(x,A,T)}this.setIndex(f),this.setAttribute("position",new nt(g,3)),this.setAttribute("normal",new nt(y,3)),this.setAttribute("uv",new nt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.width,e.height,e.widthSegments,e.heightSegments)}},Ls=class s extends pt{constructor(e=.5,t=1,n=32,i=1,r=0,o=Math.PI*2){super(),this.type="RingGeometry",this.parameters={innerRadius:e,outerRadius:t,thetaSegments:n,phiSegments:i,thetaStart:r,thetaLength:o},n=Math.max(3,n),i=Math.max(1,i);let a=[],l=[],c=[],h=[],u=e,d=(t-e)/i,f=new B,g=new _e;for(let y=0;y<=i;y++){for(let m=0;m<=n;m++){let p=r+m/n*o;f.x=u*Math.cos(p),f.y=u*Math.sin(p),l.push(f.x,f.y,f.z),c.push(0,0,1),g.x=(f.x/t+1)/2,g.y=(f.y/t+1)/2,h.push(g.x,g.y)}u+=d}for(let y=0;y<i;y++){let m=y*(n+1);for(let p=0;p<n;p++){let b=p+m,S=b,x=b+n+1,A=b+n+2,T=b+1;a.push(S,x,T),a.push(x,A,T)}}this.setIndex(a),this.setAttribute("position",new nt(l,3)),this.setAttribute("normal",new nt(c,3)),this.setAttribute("uv",new nt(h,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.innerRadius,e.outerRadius,e.thetaSegments,e.phiSegments,e.thetaStart,e.thetaLength)}};var gi=class s extends pt{constructor(e=1,t=32,n=16,i=0,r=Math.PI*2,o=0,a=Math.PI){super(),this.type="SphereGeometry",this.parameters={radius:e,widthSegments:t,heightSegments:n,phiStart:i,phiLength:r,thetaStart:o,thetaLength:a},t=Math.max(3,Math.floor(t)),n=Math.max(2,Math.floor(n));let l=Math.min(o+a,Math.PI),c=0,h=[],u=new B,d=new B,f=[],g=[],y=[],m=[];for(let p=0;p<=n;p++){let b=[],S=p/n,x=o+S*a,A=e*Math.cos(x),T=Math.sqrt(e*e-A*A),I=0;p===0&&o===0?I=.5/t:p===n&&l===Math.PI&&(I=-.5/t);for(let v=0;v<=t;v++){let D=v/t,w=i+D*r;u.x=-T*Math.cos(w),u.y=A,u.z=T*Math.sin(w),g.push(u.x,u.y,u.z),d.copy(u).normalize(),y.push(d.x,d.y,d.z),m.push(D+I,1-S),b.push(c++)}h.push(b)}for(let p=0;p<n;p++)for(let b=0;b<t;b++){let S=h[p][b+1],x=h[p][b],A=h[p+1][b],T=h[p+1][b+1];(p!==0||o>0)&&f.push(S,x,T),(p!==n-1||l<Math.PI)&&f.push(x,A,T)}this.setIndex(f),this.setAttribute("position",new nt(g,3)),this.setAttribute("normal",new nt(y,3)),this.setAttribute("uv",new nt(m,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.widthSegments,e.heightSegments,e.phiStart,e.phiLength,e.thetaStart,e.thetaLength)}};var Nn=class s extends pt{constructor(e=1,t=.4,n=12,i=48,r=Math.PI*2,o=0,a=Math.PI*2){super(),this.type="TorusGeometry",this.parameters={radius:e,tube:t,radialSegments:n,tubularSegments:i,arc:r,thetaStart:o,thetaLength:a},n=Math.floor(n),i=Math.floor(i);let l=[],c=[],h=[],u=[],d=new B,f=new B,g=new B;for(let y=0;y<=n;y++){let m=o+y/n*a;for(let p=0;p<=i;p++){let b=p/i*r;f.x=(e+t*Math.cos(m))*Math.cos(b),f.y=(e+t*Math.cos(m))*Math.sin(b),f.z=t*Math.sin(m),c.push(f.x,f.y,f.z),d.x=e*Math.cos(b),d.y=e*Math.sin(b),g.subVectors(f,d).normalize(),h.push(g.x,g.y,g.z),u.push(p/i),u.push(y/n)}}for(let y=1;y<=n;y++)for(let m=1;m<=i;m++){let p=(i+1)*y+m-1,b=(i+1)*(y-1)+m-1,S=(i+1)*(y-1)+m,x=(i+1)*y+m;l.push(p,b,x),l.push(b,S,x)}this.setIndex(l),this.setAttribute("position",new nt(c,3)),this.setAttribute("normal",new nt(h,3)),this.setAttribute("uv",new nt(u,2))}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}static fromJSON(e){return new s(e.radius,e.tube,e.radialSegments,e.tubularSegments,e.arc)}};var po=class s extends pt{constructor(e=new ti(new B(-1,-1,0),new B(-1,1,0),new B(1,1,0)),t=64,n=1,i=8,r=!1){super(),this.type="TubeGeometry",this.parameters={path:e,tubularSegments:t,radius:n,radialSegments:i,closed:r};let o=e.computeFrenetFrames(t,r);this.tangents=o.tangents,this.normals=o.normals,this.binormals=o.binormals;let a=new B,l=new B,c=new _e,h=new B,u=[],d=[],f=[],g=[];y(),this.setIndex(g),this.setAttribute("position",new nt(u,3)),this.setAttribute("normal",new nt(d,3)),this.setAttribute("uv",new nt(f,2));function y(){for(let S=0;S<t;S++)m(S);m(r===!1?t:0),b(),p()}function m(S){h=e.getPointAt(S/t,h);let x=o.normals[S],A=o.binormals[S];for(let T=0;T<=i;T++){let I=T/i*Math.PI*2,v=Math.sin(I),D=-Math.cos(I);l.x=D*x.x+v*A.x,l.y=D*x.y+v*A.y,l.z=D*x.z+v*A.z,l.normalize(),d.push(l.x,l.y,l.z),a.x=h.x+n*l.x,a.y=h.y+n*l.y,a.z=h.z+n*l.z,u.push(a.x,a.y,a.z)}}function p(){for(let S=1;S<=t;S++)for(let x=1;x<=i;x++){let A=(i+1)*(S-1)+(x-1),T=(i+1)*S+(x-1),I=(i+1)*S+x,v=(i+1)*(S-1)+x;g.push(A,T,v),g.push(T,I,v)}}function b(){for(let S=0;S<=t;S++)for(let x=0;x<=i;x++)c.x=S/t,c.y=x/i,f.push(c.x,c.y)}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}toJSON(){let e=super.toJSON();return e.path=this.parameters.path.toJSON(),e}static fromJSON(e){return new s(new sh[e.path.type]().fromJSON(e.path),e.tubularSegments,e.radius,e.radialSegments,e.closed)}},mo=class extends pt{constructor(e=null){if(super(),this.type="WireframeGeometry",this.parameters={geometry:e},e!==null){let t=[],n=new Set,i=new B,r=new B;if(e.index!==null){let o=e.attributes.position,a=e.index,l=e.groups;l.length===0&&(l=[{start:0,count:a.count,materialIndex:0}]);for(let c=0,h=l.length;c<h;++c){let u=l[c],d=u.start,f=u.count;for(let g=d,y=d+f;g<y;g+=3)for(let m=0;m<3;m++){let p=a.getX(g+m),b=a.getX(g+(m+1)%3);i.fromBufferAttribute(o,p),r.fromBufferAttribute(o,b),pd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}}else{let o=e.attributes.position;for(let a=0,l=o.count/3;a<l;a++)for(let c=0;c<3;c++){let h=3*a+c,u=3*a+(c+1)%3;i.fromBufferAttribute(o,h),r.fromBufferAttribute(o,u),pd(i,r,n)===!0&&(t.push(i.x,i.y,i.z),t.push(r.x,r.y,r.z))}}this.setAttribute("position",new nt(t,3))}}copy(e){return super.copy(e),this.parameters=Object.assign({},e.parameters),this}};function pd(s,e,t){let n=`${s.x},${s.y},${s.z}-${e.x},${e.y},${e.z}`,i=`${e.x},${e.y},${e.z}-${s.x},${s.y},${s.z}`;return t.has(n)===!0||t.has(i)===!0?!1:(t.add(n),t.add(i),!0)}function Bs(s){let e={};for(let t in s){e[t]={};for(let n in s[t]){let i=s[t][n];if(md(i))i.isRenderTargetTexture?(We("UniformsUtils: Textures of render targets cannot be cloned via cloneUniforms() or mergeUniforms()."),e[t][n]=null):e[t][n]=i.clone();else if(Array.isArray(i))if(md(i[0])){let r=[];for(let o=0,a=i.length;o<a;o++)r[o]=i[o].clone();e[t][n]=r}else e[t][n]=i.slice();else e[t][n]=i}}return e}function fn(s){let e={};for(let t=0;t<s.length;t++){let n=Bs(s[t]);for(let i in n)e[i]=n[i]}return e}function md(s){return s&&(s.isColor||s.isMatrix3||s.isMatrix4||s.isVector2||s.isVector3||s.isVector4||s.isTexture||s.isQuaternion)}function Rm(s){let e=[];for(let t=0;t<s.length;t++)e.push(s[t].clone());return e}function Th(s){let e=s.getRenderTarget();return e===null?s.outputColorSpace:e.isXRRenderTarget===!0?e.texture.colorSpace:ot.workingColorSpace}var oi={clone:Bs,merge:fn},Cm=`void main() {
	gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
}`,Pm=`void main() {
	gl_FragColor = vec4( 1.0, 0.0, 0.0, 1.0 );
}`,mt=class extends xn{constructor(e){super(),this.isShaderMaterial=!0,this.type="ShaderMaterial",this.defines={},this.uniforms={},this.uniformsGroups=[],this.vertexShader=Cm,this.fragmentShader=Pm,this.linewidth=1,this.wireframe=!1,this.wireframeLinewidth=1,this.fog=!1,this.lights=!1,this.clipping=!1,this.forceSinglePass=!0,this.extensions={clipCullDistance:!1,multiDraw:!1},this.defaultAttributeValues={color:[1,1,1],uv:[0,0],uv1:[0,0]},this.index0AttributeName=void 0,this.uniformsNeedUpdate=!1,this.glslVersion=null,e!==void 0&&this.setValues(e)}copy(e){return super.copy(e),this.fragmentShader=e.fragmentShader,this.vertexShader=e.vertexShader,this.uniforms=Bs(e.uniforms),this.uniformsGroups=Rm(e.uniformsGroups),this.defines=Object.assign({},e.defines),this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.fog=e.fog,this.lights=e.lights,this.clipping=e.clipping,this.extensions=Object.assign({},e.extensions),this.glslVersion=e.glslVersion,this.defaultAttributeValues=Object.assign({},e.defaultAttributeValues),this.index0AttributeName=e.index0AttributeName,this.uniformsNeedUpdate=e.uniformsNeedUpdate,this}toJSON(e){let t=super.toJSON(e);t.glslVersion=this.glslVersion,t.uniforms={};for(let i in this.uniforms){let o=this.uniforms[i].value;o&&o.isTexture?t.uniforms[i]={type:"t",value:o.toJSON(e).uuid}:o&&o.isColor?t.uniforms[i]={type:"c",value:o.getHex()}:o&&o.isVector2?t.uniforms[i]={type:"v2",value:o.toArray()}:o&&o.isVector3?t.uniforms[i]={type:"v3",value:o.toArray()}:o&&o.isVector4?t.uniforms[i]={type:"v4",value:o.toArray()}:o&&o.isMatrix3?t.uniforms[i]={type:"m3",value:o.toArray()}:o&&o.isMatrix4?t.uniforms[i]={type:"m4",value:o.toArray()}:t.uniforms[i]={value:o}}Object.keys(this.defines).length>0&&(t.defines=this.defines),t.vertexShader=this.vertexShader,t.fragmentShader=this.fragmentShader,t.lights=this.lights,t.clipping=this.clipping;let n={};for(let i in this.extensions)this.extensions[i]===!0&&(n[i]=!0);return Object.keys(n).length>0&&(t.extensions=n),t}fromJSON(e,t){if(super.fromJSON(e,t),e.uniforms!==void 0)for(let n in e.uniforms){let i=e.uniforms[n];switch(this.uniforms[n]={},i.type){case"t":this.uniforms[n].value=t[i.value]||null;break;case"c":this.uniforms[n].value=new xe().setHex(i.value);break;case"v2":this.uniforms[n].value=new _e().fromArray(i.value);break;case"v3":this.uniforms[n].value=new B().fromArray(i.value);break;case"v4":this.uniforms[n].value=new _t().fromArray(i.value);break;case"m3":this.uniforms[n].value=new et().fromArray(i.value);break;case"m4":this.uniforms[n].value=new Qe().fromArray(i.value);break;default:this.uniforms[n].value=i.value}}if(e.defines!==void 0&&(this.defines=e.defines),e.vertexShader!==void 0&&(this.vertexShader=e.vertexShader),e.fragmentShader!==void 0&&(this.fragmentShader=e.fragmentShader),e.glslVersion!==void 0&&(this.glslVersion=e.glslVersion),e.extensions!==void 0)for(let n in e.extensions)this.extensions[n]=e.extensions[n];return e.lights!==void 0&&(this.lights=e.lights),e.clipping!==void 0&&(this.clipping=e.clipping),this}},Sr=class extends mt{constructor(e){super(e),this.isRawShaderMaterial=!0,this.type="RawShaderMaterial"}},yn=class extends xn{constructor(e){super(),this.isMeshStandardMaterial=!0,this.type="MeshStandardMaterial",this.defines={STANDARD:""},this.color=new xe(16777215),this.roughness=1,this.metalness=0,this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new xe(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Wo,this.normalScale=new _e(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.roughnessMap=null,this.metalnessMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Gn,this.envMapIntensity=1,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.defines={STANDARD:""},this.color.copy(e.color),this.roughness=e.roughness,this.metalness=e.metalness,this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.roughnessMap=e.roughnessMap,this.metalnessMap=e.metalnessMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.envMapIntensity=e.envMapIntensity,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Mn=class extends yn{constructor(e){super(),this.isMeshPhysicalMaterial=!0,this.defines={STANDARD:"",PHYSICAL:""},this.type="MeshPhysicalMaterial",this.anisotropyRotation=0,this.anisotropyMap=null,this.clearcoatMap=null,this.clearcoatRoughness=0,this.clearcoatRoughnessMap=null,this.clearcoatNormalScale=new _e(1,1),this.clearcoatNormalMap=null,this.ior=1.5,Object.defineProperty(this,"reflectivity",{get:function(){return at(2.5*(this.ior-1)/(this.ior+1),0,1)},set:function(t){this.ior=(1+.4*t)/(1-.4*t)}}),this.iridescenceMap=null,this.iridescenceIOR=1.3,this.iridescenceThicknessRange=[100,400],this.iridescenceThicknessMap=null,this.sheenColor=new xe(0),this.sheenColorMap=null,this.sheenRoughness=1,this.sheenRoughnessMap=null,this.transmissionMap=null,this.thickness=0,this.thicknessMap=null,this.attenuationDistance=1/0,this.attenuationColor=new xe(1,1,1),this.specularIntensity=1,this.specularIntensityMap=null,this.specularColor=new xe(1,1,1),this.specularColorMap=null,this._anisotropy=0,this._clearcoat=0,this._dispersion=0,this._iridescence=0,this._sheen=0,this._transmission=0,this.setValues(e)}get anisotropy(){return this._anisotropy}set anisotropy(e){this._anisotropy>0!=e>0&&this.version++,this._anisotropy=e}get clearcoat(){return this._clearcoat}set clearcoat(e){this._clearcoat>0!=e>0&&this.version++,this._clearcoat=e}get iridescence(){return this._iridescence}set iridescence(e){this._iridescence>0!=e>0&&this.version++,this._iridescence=e}get dispersion(){return this._dispersion}set dispersion(e){this._dispersion>0!=e>0&&this.version++,this._dispersion=e}get sheen(){return this._sheen}set sheen(e){this._sheen>0!=e>0&&this.version++,this._sheen=e}get transmission(){return this._transmission}set transmission(e){this._transmission>0!=e>0&&this.version++,this._transmission=e}copy(e){return super.copy(e),this.defines={STANDARD:"",PHYSICAL:""},this.anisotropy=e.anisotropy,this.anisotropyRotation=e.anisotropyRotation,this.anisotropyMap=e.anisotropyMap,this.clearcoat=e.clearcoat,this.clearcoatMap=e.clearcoatMap,this.clearcoatRoughness=e.clearcoatRoughness,this.clearcoatRoughnessMap=e.clearcoatRoughnessMap,this.clearcoatNormalMap=e.clearcoatNormalMap,this.clearcoatNormalScale.copy(e.clearcoatNormalScale),this.dispersion=e.dispersion,this.ior=e.ior,this.iridescence=e.iridescence,this.iridescenceMap=e.iridescenceMap,this.iridescenceIOR=e.iridescenceIOR,this.iridescenceThicknessRange=[...e.iridescenceThicknessRange],this.iridescenceThicknessMap=e.iridescenceThicknessMap,this.sheen=e.sheen,this.sheenColor.copy(e.sheenColor),this.sheenColorMap=e.sheenColorMap,this.sheenRoughness=e.sheenRoughness,this.sheenRoughnessMap=e.sheenRoughnessMap,this.transmission=e.transmission,this.transmissionMap=e.transmissionMap,this.thickness=e.thickness,this.thicknessMap=e.thicknessMap,this.attenuationDistance=e.attenuationDistance,this.attenuationColor.copy(e.attenuationColor),this.specularIntensity=e.specularIntensity,this.specularIntensityMap=e.specularIntensityMap,this.specularColor.copy(e.specularColor),this.specularColorMap=e.specularColorMap,this}};var go=class extends xn{constructor(e){super(),this.isMeshLambertMaterial=!0,this.type="MeshLambertMaterial",this.color=new xe(16777215),this.map=null,this.lightMap=null,this.lightMapIntensity=1,this.aoMap=null,this.aoMapIntensity=1,this.emissive=new xe(0),this.emissiveIntensity=1,this.emissiveMap=null,this.bumpMap=null,this.bumpScale=1,this.normalMap=null,this.normalMapType=Wo,this.normalScale=new _e(1,1),this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.specularMap=null,this.alphaMap=null,this.envMap=null,this.envMapRotation=new Gn,this.combine=hl,this.reflectivity=1,this.envMapIntensity=1,this.refractionRatio=.98,this.wireframe=!1,this.wireframeLinewidth=1,this.wireframeLinecap="round",this.wireframeLinejoin="round",this.flatShading=!1,this.fog=!0,this.setValues(e)}copy(e){return super.copy(e),this.color.copy(e.color),this.map=e.map,this.lightMap=e.lightMap,this.lightMapIntensity=e.lightMapIntensity,this.aoMap=e.aoMap,this.aoMapIntensity=e.aoMapIntensity,this.emissive.copy(e.emissive),this.emissiveMap=e.emissiveMap,this.emissiveIntensity=e.emissiveIntensity,this.bumpMap=e.bumpMap,this.bumpScale=e.bumpScale,this.normalMap=e.normalMap,this.normalMapType=e.normalMapType,this.normalScale.copy(e.normalScale),this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.specularMap=e.specularMap,this.alphaMap=e.alphaMap,this.envMap=e.envMap,this.envMapRotation.copy(e.envMapRotation),this.combine=e.combine,this.reflectivity=e.reflectivity,this.envMapIntensity=e.envMapIntensity,this.refractionRatio=e.refractionRatio,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this.wireframeLinecap=e.wireframeLinecap,this.wireframeLinejoin=e.wireframeLinejoin,this.flatShading=e.flatShading,this.fog=e.fog,this}},Ja=class extends xn{constructor(e){super(),this.isMeshDepthMaterial=!0,this.type="MeshDepthMaterial",this.depthPacking=jd,this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.wireframe=!1,this.wireframeLinewidth=1,this.setValues(e)}copy(e){return super.copy(e),this.depthPacking=e.depthPacking,this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this.wireframe=e.wireframe,this.wireframeLinewidth=e.wireframeLinewidth,this}},$a=class extends xn{constructor(e){super(),this.isMeshDistanceMaterial=!0,this.type="MeshDistanceMaterial",this.map=null,this.alphaMap=null,this.displacementMap=null,this.displacementScale=1,this.displacementBias=0,this.setValues(e)}copy(e){return super.copy(e),this.map=e.map,this.alphaMap=e.alphaMap,this.displacementMap=e.displacementMap,this.displacementScale=e.displacementScale,this.displacementBias=e.displacementBias,this}};function wa(s,e){return!s||s.constructor===e?s:typeof e.BYTES_PER_ELEMENT=="number"?new e(s):Array.prototype.slice.call(s)}function Im(s){function e(i,r){return s[i]-s[r]}let t=s.length,n=new Array(t);for(let i=0;i!==t;++i)n[i]=i;return n.sort(e),n}function gd(s,e,t){let n=s.length,i=new s.constructor(n);for(let r=0,o=0;o!==n;++r){let a=t[r]*e;for(let l=0;l!==e;++l)i[o++]=s[a+l]}return i}function Lm(s,e,t,n){let i=1,r=s[0];for(;r!==void 0&&r[n]===void 0;)r=s[i++];if(r===void 0)return;let o=r[n];if(o!==void 0)if(Array.isArray(o))do o=r[n],o!==void 0&&(e.push(r.time),t.push(...o)),r=s[i++];while(r!==void 0);else if(o.toArray!==void 0)do o=r[n],o!==void 0&&(e.push(r.time),o.toArray(t,t.length)),r=s[i++];while(r!==void 0);else do o=r[n],o!==void 0&&(e.push(r.time),t.push(o)),r=s[i++];while(r!==void 0)}var xi=class{constructor(e,t,n,i){this.parameterPositions=e,this._cachedIndex=0,this.resultBuffer=i!==void 0?i:new t.constructor(n),this.sampleValues=t,this.valueSize=n,this.settings=null,this.DefaultSettings_={}}evaluate(e){let t=this.parameterPositions,n=this._cachedIndex,i=t[n],r=t[n-1];e:{t:{let o;n:{i:if(!(e<i)){for(let a=n+2;;){if(i===void 0){if(e<r)break i;return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}if(n===a)break;if(r=i,i=t[++n],e<i)break t}o=t.length;break n}if(!(e>=r)){let a=t[1];e<a&&(n=2,r=a);for(let l=n-2;;){if(r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(n===l)break;if(i=r,r=t[--n-1],e>=r)break t}o=n,n=0;break n}break e}for(;n<o;){let a=n+o>>>1;e<t[a]?o=a:n=a+1}if(i=t[n],r=t[n-1],r===void 0)return this._cachedIndex=0,this.copySampleValue_(0);if(i===void 0)return n=t.length,this._cachedIndex=n,this.copySampleValue_(n-1)}this._cachedIndex=n,this.intervalChanged_(n,r,i)}return this.interpolate_(n,r,e,i)}getSettings_(){return this.settings||this.DefaultSettings_}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i;for(let o=0;o!==i;++o)t[o]=n[r+o];return t}interpolate_(){throw new Error("THREE.Interpolant: Call to abstract method.")}intervalChanged_(){}},Qa=class extends xi{constructor(e,t,n,i){super(e,t,n,i),this._weightPrev=-0,this._offsetPrev=-0,this._weightNext=-0,this._offsetNext=-0,this.DefaultSettings_={endingStart:ys,endingEnd:ys}}intervalChanged_(e,t,n){let i=this.parameterPositions,r=e-2,o=e+1,a=i[r],l=i[o];if(a===void 0)switch(this.getSettings_().endingStart){case Ms:r=e,a=2*t-n;break;case Qr:r=i.length-2,a=t+i[r]-i[r+1];break;default:r=e,a=n}if(l===void 0)switch(this.getSettings_().endingEnd){case Ms:o=e,l=2*n-t;break;case Qr:o=1,l=n+i[1]-i[0];break;default:o=e-1,l=t}let c=(n-t)*.5,h=this.valueSize;this._weightPrev=c/(t-a),this._weightNext=c/(l-n),this._offsetPrev=r*h,this._offsetNext=o*h}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this._offsetPrev,u=this._offsetNext,d=this._weightPrev,f=this._weightNext,g=(n-t)/(i-t),y=g*g,m=y*g,p=-d*m+2*d*y-d*g,b=(1+d)*m+(-1.5-2*d)*y+(-.5+d)*g+1,S=(-1-f)*m+(1.5+f)*y+.5*g,x=f*m-f*y;for(let A=0;A!==a;++A)r[A]=p*o[h+A]+b*o[c+A]+S*o[l+A]+x*o[u+A];return r}},xo=class extends xi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=(n-t)/(i-t),u=1-h;for(let d=0;d!==a;++d)r[d]=o[c+d]*u+o[l+d]*h;return r}},el=class extends xi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e){return this.copySampleValue_(e-1)}},tl=class extends xi{interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=e*a,c=l-a,h=this.inTangents,u=this.outTangents;if(!h||!u){let g=(n-t)/(i-t),y=1-g;for(let m=0;m!==a;++m)r[m]=o[c+m]*y+o[l+m]*g;return r}let d=a*2,f=e-1;for(let g=0;g!==a;++g){let y=o[c+g],m=o[l+g],p=f*d+g*2,b=u[p],S=u[p+1],x=e*d+g*2,A=h[x],T=h[x+1],I=(n-t)/(i-t),v,D,w,R,L;for(let V=0;V<8;V++){v=I*I,D=v*I,w=1-I,R=w*w,L=R*w;let N=L*t+3*R*I*b+3*w*v*A+D*i-n;if(Math.abs(N)<1e-10)break;let U=3*R*(b-t)+6*w*I*(A-b)+3*v*(i-A);if(Math.abs(U)<1e-10)break;I=I-N/U,I=Math.max(0,Math.min(1,I))}r[g]=L*y+3*R*I*S+3*w*v*T+D*m}return r}},bn=class{constructor(e,t,n,i){if(e===void 0)throw new Error("THREE.KeyframeTrack: track name is undefined");if(t===void 0||t.length===0)throw new Error("THREE.KeyframeTrack: no keyframes in track named "+e);this.name=e,this.times=wa(t,this.TimeBufferType),this.values=wa(n,this.ValueBufferType),this.setInterpolation(i||this.DefaultInterpolation)}static toJSON(e){let t=e.constructor,n;if(t.toJSON!==this.toJSON)n=t.toJSON(e);else{n={name:e.name,times:wa(e.times,Array),values:wa(e.values,Array)};let i=e.getInterpolation();i!==e.DefaultInterpolation&&(n.interpolation=i)}return n.type=e.ValueTypeName,n}InterpolantFactoryMethodDiscrete(e){return new el(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodLinear(e){return new xo(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodSmooth(e){return new Qa(this.times,this.values,this.getValueSize(),e)}InterpolantFactoryMethodBezier(e){let t=new tl(this.times,this.values,this.getValueSize(),e);return this.settings&&(t.inTangents=this.settings.inTangents,t.outTangents=this.settings.outTangents),t}setInterpolation(e){let t;switch(e){case Es:t=this.InterpolantFactoryMethodDiscrete;break;case Ts:t=this.InterpolantFactoryMethodLinear;break;case Aa:t=this.InterpolantFactoryMethodSmooth;break;case th:t=this.InterpolantFactoryMethodBezier;break}if(t===void 0){let n="unsupported interpolation for "+this.ValueTypeName+" keyframe track named "+this.name;if(this.createInterpolant===void 0)if(e!==this.DefaultInterpolation)this.setInterpolation(this.DefaultInterpolation);else throw new Error(n);return We("KeyframeTrack:",n),this}return this.createInterpolant=t,this}getInterpolation(){switch(this.createInterpolant){case this.InterpolantFactoryMethodDiscrete:return Es;case this.InterpolantFactoryMethodLinear:return Ts;case this.InterpolantFactoryMethodSmooth:return Aa;case this.InterpolantFactoryMethodBezier:return th}}getValueSize(){return this.values.length/this.times.length}shift(e){if(e!==0){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]+=e}return this}scale(e){if(e!==1){let t=this.times;for(let n=0,i=t.length;n!==i;++n)t[n]*=e}return this}trim(e,t){let n=this.times,i=n.length,r=0,o=i-1;for(;r!==i&&n[r]<e;)++r;for(;o!==-1&&n[o]>t;)--o;if(++o,r!==0||o!==i){r>=o&&(o=Math.max(o,1),r=o-1);let a=this.getValueSize();this.times=n.slice(r,o),this.values=this.values.slice(r*a,o*a)}return this}validate(){let e=!0,t=this.getValueSize();t-Math.floor(t)!==0&&($e("KeyframeTrack: Invalid value size in track.",this),e=!1);let n=this.times,i=this.values,r=n.length;r===0&&($e("KeyframeTrack: Track is empty.",this),e=!1);let o=null;for(let a=0;a!==r;a++){let l=n[a];if(typeof l=="number"&&isNaN(l)){$e("KeyframeTrack: Time is not a valid number.",this,a,l),e=!1;break}if(o!==null&&o>l){$e("KeyframeTrack: Out of order keys.",this,a,l,o),e=!1;break}o=l}if(i!==void 0&&Op(i))for(let a=0,l=i.length;a!==l;++a){let c=i[a];if(isNaN(c)){$e("KeyframeTrack: Value is not a valid number.",this,a,c),e=!1;break}}return e}optimize(){let e=this.times.slice(),t=this.values.slice(),n=this.getValueSize(),i=this.getInterpolation()===Aa,r=e.length-1,o=1;for(let a=1;a<r;++a){let l=!1,c=e[a],h=e[a+1];if(c!==h&&(a!==1||c!==e[0]))if(i)l=!0;else{let u=a*n,d=u-n,f=u+n;for(let g=0;g!==n;++g){let y=t[u+g];if(y!==t[d+g]||y!==t[f+g]){l=!0;break}}}if(l){if(a!==o){e[o]=e[a];let u=a*n,d=o*n;for(let f=0;f!==n;++f)t[d+f]=t[u+f]}++o}}if(r>0){e[o]=e[r];for(let a=r*n,l=o*n,c=0;c!==n;++c)t[l+c]=t[a+c];++o}return o!==e.length?(this.times=e.slice(0,o),this.values=t.slice(0,o*n)):(this.times=e,this.values=t),this}clone(){let e=this.times.slice(),t=this.values.slice(),n=this.constructor,i=new n(this.name,e,t);return i.createInterpolant=this.createInterpolant,i}};bn.prototype.ValueTypeName="";bn.prototype.TimeBufferType=Float32Array;bn.prototype.ValueBufferType=Float32Array;bn.prototype.DefaultInterpolation=Ts;var Ui=class extends bn{constructor(e,t,n){super(e,t,n)}};Ui.prototype.ValueTypeName="bool";Ui.prototype.ValueBufferType=Array;Ui.prototype.DefaultInterpolation=Es;Ui.prototype.InterpolantFactoryMethodLinear=void 0;Ui.prototype.InterpolantFactoryMethodSmooth=void 0;var _o=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}};_o.prototype.ValueTypeName="color";var Fi=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}};Fi.prototype.ValueTypeName="number";var nl=class extends xi{constructor(e,t,n,i){super(e,t,n,i)}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=(n-t)/(i-t),c=e*a;for(let h=c+a;c!==h;c+=4)Zt.slerpFlat(r,0,o,c-a,o,c,l);return r}},Oi=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}InterpolantFactoryMethodLinear(e){return new nl(this.times,this.values,this.getValueSize(),e)}};Oi.prototype.ValueTypeName="quaternion";Oi.prototype.InterpolantFactoryMethodSmooth=void 0;var Bi=class extends bn{constructor(e,t,n){super(e,t,n)}};Bi.prototype.ValueTypeName="string";Bi.prototype.ValueBufferType=Array;Bi.prototype.DefaultInterpolation=Es;Bi.prototype.InterpolantFactoryMethodLinear=void 0;Bi.prototype.InterpolantFactoryMethodSmooth=void 0;var is=class extends bn{constructor(e,t,n,i){super(e,t,n,i)}};is.prototype.ValueTypeName="vector";var Ds=class{constructor(e="",t=-1,n=[],i=$l){this.name=e,this.tracks=n,this.duration=t,this.blendMode=i,this.uuid=ei(),this.userData={},this.duration<0&&this.resetDuration()}static parse(e){let t=[],n=e.tracks,i=1/(e.fps||1);for(let o=0,a=n.length;o!==a;++o)t.push(Nm(n[o]).scale(i));let r=new this(e.name,e.duration,t,e.blendMode);return r.uuid=e.uuid,r.userData=JSON.parse(e.userData||"{}"),r}static toJSON(e){let t=[],n=e.tracks,i={name:e.name,duration:e.duration,tracks:t,uuid:e.uuid,blendMode:e.blendMode,userData:JSON.stringify(e.userData)};for(let r=0,o=n.length;r!==o;++r)t.push(bn.toJSON(n[r]));return i}static CreateFromMorphTargetSequence(e,t,n,i){let r=t.length,o=[];for(let a=0;a<r;a++){let l=[],c=[];l.push((a+r-1)%r,a,(a+1)%r),c.push(0,1,0);let h=Im(l);l=gd(l,1,h),c=gd(c,1,h),!i&&l[0]===0&&(l.push(r),c.push(c[0])),o.push(new Fi(".morphTargetInfluences["+t[a].name+"]",l,c).scale(1/n))}return new this(e,-1,o)}static findByName(e,t){let n=e;if(!Array.isArray(e)){let i=e;n=i.geometry&&i.geometry.animations||i.animations}for(let i=0;i<n.length;i++)if(n[i].name===t)return n[i];return null}static CreateClipsFromMorphTargetSequences(e,t,n){let i={},r=/^([\w-]*?)([\d]+)$/;for(let a=0,l=e.length;a<l;a++){let c=e[a],h=c.name.match(r);if(h&&h.length>1){let u=h[1],d=i[u];d||(i[u]=d=[]),d.push(c)}}let o=[];for(let a in i)o.push(this.CreateFromMorphTargetSequence(a,i[a],t,n));return o}resetDuration(){let e=this.tracks,t=0;for(let n=0,i=e.length;n!==i;++n){let r=this.tracks[n];t=Math.max(t,r.times[r.times.length-1])}return this.duration=t,this}trim(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].trim(0,this.duration);return this}validate(){let e=!0;for(let t=0;t<this.tracks.length;t++)e=e&&this.tracks[t].validate();return e}optimize(){for(let e=0;e<this.tracks.length;e++)this.tracks[e].optimize();return this}clone(){let e=[];for(let n=0;n<this.tracks.length;n++)e.push(this.tracks[n].clone());let t=new this.constructor(this.name,this.duration,e,this.blendMode);return t.userData=JSON.parse(JSON.stringify(this.userData)),t}toJSON(){return this.constructor.toJSON(this)}};function Dm(s){switch(s.toLowerCase()){case"scalar":case"double":case"float":case"number":case"integer":return Fi;case"vector":case"vector2":case"vector3":case"vector4":return is;case"color":return _o;case"quaternion":return Oi;case"bool":case"boolean":return Ui;case"string":return Bi}throw new Error("THREE.KeyframeTrack: Unsupported typeName: "+s)}function Nm(s){if(s.type===void 0)throw new Error("THREE.KeyframeTrack: track type undefined, can not parse");let e=Dm(s.type);if(s.times===void 0){let t=[],n=[];Lm(s.keys,t,n,"value"),s.times=t,s.values=n}return e.parse!==void 0?e.parse(s):new e(s.name,s.times,s.values,s.interpolation)}var hi={enabled:!1,files:{},add:function(s,e){this.enabled!==!1&&(xd(s)||(this.files[s]=e))},get:function(s){if(this.enabled!==!1&&!xd(s))return this.files[s]},remove:function(s){delete this.files[s]},clear:function(){this.files={}}};function xd(s){try{let e=s.slice(s.indexOf(":")+1);return new URL(e).protocol==="blob:"}catch{return!1}}var il=class{constructor(e,t,n){let i=this,r=!1,o=0,a=0,l,c=[];this.onStart=void 0,this.onLoad=e,this.onProgress=t,this.onError=n,this._abortController=null,this.itemStart=function(h){a++,r===!1&&i.onStart!==void 0&&i.onStart(h,o,a),r=!0},this.itemEnd=function(h){o++,i.onProgress!==void 0&&i.onProgress(h,o,a),o===a&&(r=!1,i.onLoad!==void 0&&i.onLoad())},this.itemError=function(h){i.onError!==void 0&&i.onError(h)},this.resolveURL=function(h){return h=h.normalize("NFC"),l?l(h):h},this.setURLModifier=function(h){return l=h,this},this.addHandler=function(h,u){return c.push(h,u),this},this.removeHandler=function(h){let u=c.indexOf(h);return u!==-1&&c.splice(u,2),this},this.getHandler=function(h){for(let u=0,d=c.length;u<d;u+=2){let f=c[u],g=c[u+1];if(f.global&&(f.lastIndex=0),f.test(h))return g}return null},this.abort=function(){return this.abortController.abort(),this._abortController=null,this}}get abortController(){return this._abortController||(this._abortController=new AbortController),this._abortController}},hf=new il,_i=class{constructor(e){this.manager=e!==void 0?e:hf,this.crossOrigin="anonymous",this.withCredentials=!1,this.path="",this.resourcePath="",this.requestHeader={},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}load(){}loadAsync(e,t){let n=this;return new Promise(function(i,r){n.load(e,i,t,r)})}parse(){}setCrossOrigin(e){return this.crossOrigin=e,this}setWithCredentials(e){return this.withCredentials=e,this}setPath(e){return this.path=e,this}setResourcePath(e){return this.resourcePath=e,this}setRequestHeader(e){return this.requestHeader=e,this}abort(){return this}};_i.DEFAULT_MATERIAL_NAME="__DEFAULT";var Ii={},rh=class extends Error{constructor(e,t){super(e),this.response=t}},wr=class extends _i{constructor(e){super(e),this.mimeType="",this.responseType="",this._abortController=new AbortController}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=hi.get(`file:${e}`);if(r!==void 0){this.manager.itemStart(e),setTimeout(()=>{t&&t(r),this.manager.itemEnd(e)},0);return}if(Ii[e]!==void 0){Ii[e].push({onLoad:t,onProgress:n,onError:i});return}Ii[e]=[],Ii[e].push({onLoad:t,onProgress:n,onError:i});let o=new Request(e,{headers:new Headers(this.requestHeader),credentials:this.withCredentials?"include":"same-origin",signal:typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal}),a=this.mimeType,l=this.responseType;fetch(o).then(c=>{if(c.status===200||c.status===0){if(c.status===0&&We("FileLoader: HTTP Status 0 received."),typeof ReadableStream>"u"||c.body===void 0||c.body.getReader===void 0)return c;let h=Ii[e],u=c.body.getReader(),d=c.headers.get("X-File-Size")||c.headers.get("Content-Length"),f=d?parseInt(d):0,g=f!==0,y=0,m=new ReadableStream({start(p){b();function b(){u.read().then(({done:S,value:x})=>{if(S)p.close();else{y+=x.byteLength;let A=new ProgressEvent("progress",{lengthComputable:g,loaded:y,total:f});for(let T=0,I=h.length;T<I;T++){let v=h[T];v.onProgress&&v.onProgress(A)}p.enqueue(x),b()}},S=>{p.error(S)})}}});return new Response(m)}else throw new rh(`fetch for "${c.url}" responded with ${c.status}: ${c.statusText}`,c)}).then(c=>{switch(l){case"arraybuffer":return c.arrayBuffer();case"blob":return c.blob();case"document":return c.text().then(h=>new DOMParser().parseFromString(h,a));case"json":return c.json();default:if(a==="")return c.text();{let u=/charset="?([^;"\s]*)"?/i.exec(a),d=u&&u[1]?u[1].toLowerCase():void 0,f=new TextDecoder(d);return c.arrayBuffer().then(g=>f.decode(g))}}}).then(c=>{hi.add(`file:${e}`,c);let h=Ii[e];delete Ii[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onLoad&&f.onLoad(c)}}).catch(c=>{let h=Ii[e];if(h===void 0)throw this.manager.itemError(e),c;delete Ii[e];for(let u=0,d=h.length;u<d;u++){let f=h[u];f.onError&&f.onError(c)}this.manager.itemError(e)}).finally(()=>{this.manager.itemEnd(e)}),this.manager.itemStart(e)}setResponseType(e){return this.responseType=e,this}setMimeType(e){return this.mimeType=e,this}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var ir=new WeakMap,sl=class extends _i{constructor(e){super(e)}load(e,t,n,i){this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=hi.get(`image:${e}`);if(o!==void 0){if(o.complete===!0)r.manager.itemStart(e),setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);else{let u=ir.get(o);u===void 0&&(u=[],ir.set(o,u)),u.push({onLoad:t,onError:i})}return o}let a=cr("img");function l(){h(),t&&t(this);let u=ir.get(this)||[];for(let d=0;d<u.length;d++){let f=u[d];f.onLoad&&f.onLoad(this)}ir.delete(this),r.manager.itemEnd(e)}function c(u){h(),i&&i(u),hi.remove(`image:${e}`);let d=ir.get(this)||[];for(let f=0;f<d.length;f++){let g=d[f];g.onError&&g.onError(u)}ir.delete(this),r.manager.itemError(e),r.manager.itemEnd(e)}function h(){a.removeEventListener("load",l,!1),a.removeEventListener("error",c,!1)}return a.addEventListener("load",l,!1),a.addEventListener("error",c,!1),e.slice(0,5)!=="data:"&&this.crossOrigin!==void 0&&(a.crossOrigin=this.crossOrigin),hi.add(`image:${e}`,a),r.manager.itemStart(e),a.src=e,a}};var vo=class extends _i{constructor(e){super(e)}load(e,t,n,i){let r=new jt,o=new sl(this.manager);return o.setCrossOrigin(this.crossOrigin),o.setPath(this.path),o.load(e,function(a){r.image=a,r.needsUpdate=!0,t!==void 0&&t(r)},n,i),r}},Ns=class extends St{constructor(e,t=1){super(),this.isLight=!0,this.type="Light",this.color=new xe(e),this.intensity=t}dispose(){this.dispatchEvent({type:"dispose"})}copy(e,t){return super.copy(e,t),this.color.copy(e.color),this.intensity=e.intensity,this}toJSON(e){let t=super.toJSON(e);return t.object.color=this.color.getHex(),t.object.intensity=this.intensity,t}},yo=class extends Ns{constructor(e,t,n){super(e,n),this.isHemisphereLight=!0,this.type="HemisphereLight",this.position.copy(St.DEFAULT_UP),this.updateMatrix(),this.groundColor=new xe(t)}copy(e,t){return super.copy(e,t),this.groundColor.copy(e.groundColor),this}toJSON(e){let t=super.toJSON(e);return t.object.groundColor=this.groundColor.getHex(),t}},$c=new Qe,_d=new B,vd=new B,Mo=class{constructor(e){this.camera=e,this.intensity=1,this.bias=0,this.biasNode=null,this.normalBias=0,this.radius=1,this.blurSamples=8,this.mapSize=new _e(512,512),this.mapType=Sn,this.map=null,this.mapPass=null,this.matrix=new Qe,this.autoUpdate=!0,this.needsUpdate=!1,this._frustum=new xr,this._frameExtents=new _e(1,1),this._viewportCount=1,this._viewports=[new _t(0,0,1,1)]}getViewportCount(){return this._viewportCount}getFrustum(){return this._frustum}updateMatrices(e){let t=this.camera,n=this.matrix;_d.setFromMatrixPosition(e.matrixWorld),t.position.copy(_d),vd.setFromMatrixPosition(e.target.matrixWorld),t.lookAt(vd),t.updateMatrixWorld(),$c.multiplyMatrices(t.projectionMatrix,t.matrixWorldInverse),this._frustum.setFromProjectionMatrix($c,t.coordinateSystem,t.reversedDepth),t.coordinateSystem===lr||t.reversedDepth?n.set(.5,0,0,.5,0,.5,0,.5,0,0,1,0,0,0,0,1):n.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),n.multiply($c)}getViewport(e){return this._viewports[e]}getFrameExtents(){return this._frameExtents}dispose(){this.map&&this.map.dispose(),this.mapPass&&this.mapPass.dispose()}copy(e){return this.camera=e.camera.clone(),this.intensity=e.intensity,this.bias=e.bias,this.radius=e.radius,this.autoUpdate=e.autoUpdate,this.needsUpdate=e.needsUpdate,this.normalBias=e.normalBias,this.blurSamples=e.blurSamples,this.mapSize.copy(e.mapSize),this.biasNode=e.biasNode,this}clone(){return new this.constructor().copy(this)}toJSON(){let e={};return this.intensity!==1&&(e.intensity=this.intensity),this.bias!==0&&(e.bias=this.bias),this.normalBias!==0&&(e.normalBias=this.normalBias),this.radius!==1&&(e.radius=this.radius),(this.mapSize.x!==512||this.mapSize.y!==512)&&(e.mapSize=this.mapSize.toArray()),e.camera=this.camera.toJSON(!1).object,delete e.camera.matrix,e}},Ea=new B,Ta=new Zt,ci=new B,bo=class extends St{constructor(){super(),this.isCamera=!0,this.type="Camera",this.matrixWorldInverse=new Qe,this.projectionMatrix=new Qe,this.projectionMatrixInverse=new Qe,this.coordinateSystem=Qn,this._reversedDepth=!1}get reversedDepth(){return this._reversedDepth}copy(e,t){return super.copy(e,t),this.matrixWorldInverse.copy(e.matrixWorldInverse),this.projectionMatrix.copy(e.projectionMatrix),this.projectionMatrixInverse.copy(e.projectionMatrixInverse),this.coordinateSystem=e.coordinateSystem,this}getWorldDirection(e){return super.getWorldDirection(e).negate()}updateMatrixWorld(e){super.updateMatrixWorld(e),this.matrixWorld.decompose(Ea,Ta,ci),ci.x===1&&ci.y===1&&ci.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Ea,Ta,ci.set(1,1,1)).invert()}updateWorldMatrix(e,t,n=!1){super.updateWorldMatrix(e,t,n),this.matrixWorld.decompose(Ea,Ta,ci),ci.x===1&&ci.y===1&&ci.z===1?this.matrixWorldInverse.copy(this.matrixWorld).invert():this.matrixWorldInverse.compose(Ea,Ta,ci.set(1,1,1)).invert()}clone(){return new this.constructor().copy(this)}},Ji=new B,yd=new _e,Md=new _e,Qt=class extends bo{constructor(e=50,t=1,n=.1,i=2e3){super(),this.isPerspectiveCamera=!0,this.type="PerspectiveCamera",this.fov=e,this.zoom=1,this.near=n,this.far=i,this.focus=10,this.aspect=t,this.view=null,this.filmGauge=35,this.filmOffset=0,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.fov=e.fov,this.zoom=e.zoom,this.near=e.near,this.far=e.far,this.focus=e.focus,this.aspect=e.aspect,this.view=e.view===null?null:Object.assign({},e.view),this.filmGauge=e.filmGauge,this.filmOffset=e.filmOffset,this}setFocalLength(e){let t=.5*this.getFilmHeight()/e;this.fov=As*2*Math.atan(t),this.updateProjectionMatrix()}getFocalLength(){let e=Math.tan(Kr*.5*this.fov);return .5*this.getFilmHeight()/e}getEffectiveFOV(){return As*2*Math.atan(Math.tan(Kr*.5*this.fov)/this.zoom)}getFilmWidth(){return this.filmGauge*Math.min(this.aspect,1)}getFilmHeight(){return this.filmGauge/Math.max(this.aspect,1)}getViewBounds(e,t,n){Ji.set(-1,-1,.5).applyMatrix4(this.projectionMatrixInverse),t.set(Ji.x,Ji.y).multiplyScalar(-e/Ji.z),Ji.set(1,1,.5).applyMatrix4(this.projectionMatrixInverse),n.set(Ji.x,Ji.y).multiplyScalar(-e/Ji.z)}getViewSize(e,t){return this.getViewBounds(e,yd,Md),t.subVectors(Md,yd)}setViewOffset(e,t,n,i,r,o){this.aspect=e/t,this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=this.near,t=e*Math.tan(Kr*.5*this.fov)/this.zoom,n=2*t,i=this.aspect*n,r=-.5*i,o=this.view;if(this.view!==null&&this.view.enabled){let l=o.fullWidth,c=o.fullHeight;r+=o.offsetX*i/l,t-=o.offsetY*n/c,i*=o.width/l,n*=o.height/c}let a=this.filmOffset;a!==0&&(r+=e*a/this.getFilmWidth()),this.projectionMatrix.makePerspective(r,r+i,t,t-n,e,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.fov=this.fov,t.object.zoom=this.zoom,t.object.near=this.near,t.object.far=this.far,t.object.focus=this.focus,t.object.aspect=this.aspect,this.view!==null&&(t.object.view=Object.assign({},this.view)),t.object.filmGauge=this.filmGauge,t.object.filmOffset=this.filmOffset,t}},oh=class extends Mo{constructor(){super(new Qt(50,1,.5,500)),this.isSpotLightShadow=!0,this.focus=1,this.aspect=1}updateMatrices(e){let t=this.camera,n=As*2*e.angle*this.focus,i=this.mapSize.width/this.mapSize.height*this.aspect,r=e.distance||t.far;(n!==t.fov||i!==t.aspect||r!==t.far)&&(t.fov=n,t.aspect=i,t.far=r,t.updateProjectionMatrix()),super.updateMatrices(e)}copy(e){return super.copy(e),this.focus=e.focus,this}},So=class extends Ns{constructor(e,t,n=0,i=Math.PI/3,r=0,o=2){super(e,t),this.isSpotLight=!0,this.type="SpotLight",this.position.copy(St.DEFAULT_UP),this.updateMatrix(),this.target=new St,this.distance=n,this.angle=i,this.penumbra=r,this.decay=o,this.map=null,this.shadow=new oh}get power(){return this.intensity*Math.PI}set power(e){this.intensity=e/Math.PI}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.angle=e.angle,this.penumbra=e.penumbra,this.decay=e.decay,this.target=e.target.clone(),this.map=e.map,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.angle=this.angle,t.object.decay=this.decay,t.object.penumbra=this.penumbra,t.object.target=this.target.uuid,this.map&&this.map.isTexture&&(t.object.map=this.map.toJSON(e).uuid),t.object.shadow=this.shadow.toJSON(),t}},ah=class extends Mo{constructor(){super(new Qt(90,1,.5,500)),this.isPointLightShadow=!0}},ni=class extends Ns{constructor(e,t,n=0,i=2){super(e,t),this.isPointLight=!0,this.type="PointLight",this.distance=n,this.decay=i,this.shadow=new ah}get power(){return this.intensity*4*Math.PI}set power(e){this.intensity=e/(4*Math.PI)}dispose(){super.dispose(),this.shadow.dispose()}copy(e,t){return super.copy(e,t),this.distance=e.distance,this.decay=e.decay,this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.distance=this.distance,t.object.decay=this.decay,t.object.shadow=this.shadow.toJSON(),t}},vi=class extends bo{constructor(e=-1,t=1,n=1,i=-1,r=.1,o=2e3){super(),this.isOrthographicCamera=!0,this.type="OrthographicCamera",this.zoom=1,this.view=null,this.left=e,this.right=t,this.top=n,this.bottom=i,this.near=r,this.far=o,this.updateProjectionMatrix()}copy(e,t){return super.copy(e,t),this.left=e.left,this.right=e.right,this.top=e.top,this.bottom=e.bottom,this.near=e.near,this.far=e.far,this.zoom=e.zoom,this.view=e.view===null?null:Object.assign({},e.view),this}setViewOffset(e,t,n,i,r,o){this.view===null&&(this.view={enabled:!0,fullWidth:1,fullHeight:1,offsetX:0,offsetY:0,width:1,height:1}),this.view.enabled=!0,this.view.fullWidth=e,this.view.fullHeight=t,this.view.offsetX=n,this.view.offsetY=i,this.view.width=r,this.view.height=o,this.updateProjectionMatrix()}clearViewOffset(){this.view!==null&&(this.view.enabled=!1),this.updateProjectionMatrix()}updateProjectionMatrix(){let e=(this.right-this.left)/(2*this.zoom),t=(this.top-this.bottom)/(2*this.zoom),n=(this.right+this.left)/2,i=(this.top+this.bottom)/2,r=n-e,o=n+e,a=i+t,l=i-t;if(this.view!==null&&this.view.enabled){let c=(this.right-this.left)/this.view.fullWidth/this.zoom,h=(this.top-this.bottom)/this.view.fullHeight/this.zoom;r+=c*this.view.offsetX,o=r+c*this.view.width,a-=h*this.view.offsetY,l=a-h*this.view.height}this.projectionMatrix.makeOrthographic(r,o,a,l,this.near,this.far,this.coordinateSystem,this.reversedDepth),this.projectionMatrixInverse.copy(this.projectionMatrix).invert()}toJSON(e){let t=super.toJSON(e);return t.object.zoom=this.zoom,t.object.left=this.left,t.object.right=this.right,t.object.top=this.top,t.object.bottom=this.bottom,t.object.near=this.near,t.object.far=this.far,this.view!==null&&(t.object.view=Object.assign({},this.view)),t}},lh=class extends Mo{constructor(){super(new vi(-5,5,5,-5,.5,500)),this.isDirectionalLightShadow=!0}},ss=class extends Ns{constructor(e,t){super(e,t),this.isDirectionalLight=!0,this.type="DirectionalLight",this.position.copy(St.DEFAULT_UP),this.updateMatrix(),this.target=new St,this.shadow=new lh}dispose(){super.dispose(),this.shadow.dispose()}copy(e){return super.copy(e),this.target=e.target.clone(),this.shadow=e.shadow.clone(),this}toJSON(e){let t=super.toJSON(e);return t.object.shadow=this.shadow.toJSON(),t.object.target=this.target.uuid,t}};var zi=class{static extractUrlBase(e){let t=e.lastIndexOf("/");return t===-1?"./":e.slice(0,t+1)}static resolveURL(e,t){return typeof e!="string"||e===""?"":(/^https?:\/\//i.test(t)&&/^\//.test(e)&&(t=t.replace(/(^https?:\/\/[^\/]+).*/i,"$1")),/^(https?:)?\/\//i.test(e)||/^data:.*,.*$/i.test(e)||/^blob:.*$/i.test(e)?e:t+e)}};var Qc=new WeakMap,wo=class extends _i{constructor(e){super(e),this.isImageBitmapLoader=!0,typeof createImageBitmap>"u"&&We("ImageBitmapLoader: createImageBitmap() not supported."),typeof fetch>"u"&&We("ImageBitmapLoader: fetch() not supported."),this.options={premultiplyAlpha:"none"},this._abortController=new AbortController}setOptions(e){return this.options=e,this}load(e,t,n,i){e===void 0&&(e=""),this.path!==void 0&&(e=this.path+e),e=this.manager.resolveURL(e);let r=this,o=hi.get(`image-bitmap:${e}`);if(o!==void 0){if(r.manager.itemStart(e),o.then){o.then(c=>{Qc.has(o)===!0?(i&&i(Qc.get(o)),r.manager.itemError(e),r.manager.itemEnd(e)):(t&&t(c),r.manager.itemEnd(e))});return}setTimeout(function(){t&&t(o),r.manager.itemEnd(e)},0);return}let a={};a.credentials=this.crossOrigin==="anonymous"?"same-origin":"include",a.headers=this.requestHeader,a.signal=typeof AbortSignal.any=="function"?AbortSignal.any([this._abortController.signal,this.manager.abortController.signal]):this._abortController.signal;let l=fetch(e,a).then(function(c){return c.blob()}).then(function(c){return createImageBitmap(c,Object.assign(r.options,{colorSpaceConversion:"none"}))}).then(function(c){hi.add(`image-bitmap:${e}`,c),t&&t(c),r.manager.itemEnd(e)}).catch(function(c){i&&i(c),Qc.set(l,c),hi.remove(`image-bitmap:${e}`),r.manager.itemError(e),r.manager.itemEnd(e)});hi.add(`image-bitmap:${e}`,l),r.manager.itemStart(e)}abort(){return this._abortController.abort(),this._abortController=new AbortController,this}};var sr=-90,rr=1,rl=class extends St{constructor(e,t,n){super(),this.type="CubeCamera",this.renderTarget=n,this.coordinateSystem=null,this.activeMipmapLevel=0;let i=new Qt(sr,rr,e,t);i.layers=this.layers,this.add(i);let r=new Qt(sr,rr,e,t);r.layers=this.layers,this.add(r);let o=new Qt(sr,rr,e,t);o.layers=this.layers,this.add(o);let a=new Qt(sr,rr,e,t);a.layers=this.layers,this.add(a);let l=new Qt(sr,rr,e,t);l.layers=this.layers,this.add(l);let c=new Qt(sr,rr,e,t);c.layers=this.layers,this.add(c)}updateCoordinateSystem(){let e=this.coordinateSystem,t=this.children.concat(),[n,i,r,o,a,l]=t;for(let c of t)this.remove(c);if(e===Qn)n.up.set(0,1,0),n.lookAt(1,0,0),i.up.set(0,1,0),i.lookAt(-1,0,0),r.up.set(0,0,-1),r.lookAt(0,1,0),o.up.set(0,0,1),o.lookAt(0,-1,0),a.up.set(0,1,0),a.lookAt(0,0,1),l.up.set(0,1,0),l.lookAt(0,0,-1);else if(e===lr)n.up.set(0,-1,0),n.lookAt(-1,0,0),i.up.set(0,-1,0),i.lookAt(1,0,0),r.up.set(0,0,1),r.lookAt(0,1,0),o.up.set(0,0,-1),o.lookAt(0,-1,0),a.up.set(0,-1,0),a.lookAt(0,0,1),l.up.set(0,-1,0),l.lookAt(0,0,-1);else throw new Error("THREE.CubeCamera.updateCoordinateSystem(): Invalid coordinate system: "+e);for(let c of t)this.add(c),c.updateMatrixWorld()}update(e,t){this.parent===null&&this.updateMatrixWorld();let{renderTarget:n,activeMipmapLevel:i}=this;this.coordinateSystem!==e.coordinateSystem&&(this.coordinateSystem=e.coordinateSystem,this.updateCoordinateSystem());let[r,o,a,l,c,h]=this.children,u=e.getRenderTarget(),d=e.getActiveCubeFace(),f=e.getActiveMipmapLevel(),g=e.xr.enabled;e.xr.enabled=!1;let y=n.texture.generateMipmaps;n.texture.generateMipmaps=!1;let m=!1;e.isWebGLRenderer===!0?m=e.state.buffers.depth.getReversed():m=e.reversedDepthBuffer,e.setRenderTarget(n,0,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,r),e.setRenderTarget(n,1,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,o),e.setRenderTarget(n,2,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,a),e.setRenderTarget(n,3,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,l),e.setRenderTarget(n,4,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,c),n.texture.generateMipmaps=y,e.setRenderTarget(n,5,i),m&&e.autoClear===!1&&e.clearDepth(),e.render(t,h),e.setRenderTarget(u,d,f),e.xr.enabled=g,n.texture.needsPMREMUpdate=!0}},ol=class extends Qt{constructor(e=[]){super(),this.isArrayCamera=!0,this.isMultiViewCamera=!1,this.cameras=e}},Eo=class{constructor(){this._previousTime=0,this._currentTime=0,this._startTime=performance.now(),this._delta=0,this._elapsed=0,this._timescale=1,this._document=null,this._pageVisibilityHandler=null}connect(e){this._document=e,e.hidden!==void 0&&(this._pageVisibilityHandler=Um.bind(this),e.addEventListener("visibilitychange",this._pageVisibilityHandler,!1))}disconnect(){this._pageVisibilityHandler!==null&&(this._document.removeEventListener("visibilitychange",this._pageVisibilityHandler),this._pageVisibilityHandler=null),this._document=null}getDelta(){return this._delta/1e3}getElapsed(){return this._elapsed/1e3}getTimescale(){return this._timescale}setTimescale(e){return this._timescale=e,this}reset(){return this._currentTime=performance.now()-this._startTime,this}dispose(){this.disconnect()}update(e){return this._pageVisibilityHandler!==null&&this._document.hidden===!0?this._delta=0:(this._previousTime=this._currentTime,this._currentTime=(e!==void 0?e:performance.now())-this._startTime,this._delta=(this._currentTime-this._previousTime)*this._timescale,this._elapsed+=this._delta),this}};function Um(){this._document.hidden===!1&&this.reset()}var al=class{constructor(e,t,n){this.binding=e,this.valueSize=n;let i,r,o;switch(t){case"quaternion":i=this._slerp,r=this._slerpAdditive,o=this._setAdditiveIdentityQuaternion,this.buffer=new Float64Array(n*6),this._workIndex=5;break;case"string":case"bool":i=this._select,r=this._select,o=this._setAdditiveIdentityOther,this.buffer=new Array(n*5);break;default:i=this._lerp,r=this._lerpAdditive,o=this._setAdditiveIdentityNumeric,this.buffer=new Float64Array(n*5)}this._mixBufferRegion=i,this._mixBufferRegionAdditive=r,this._setIdentity=o,this._origIndex=3,this._addIndex=4,this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,this.useCount=0,this.referenceCount=0}accumulate(e,t){let n=this.buffer,i=this.valueSize,r=e*i+i,o=this.cumulativeWeight;if(o===0){for(let a=0;a!==i;++a)n[r+a]=n[a];o=t}else{o+=t;let a=t/o;this._mixBufferRegion(n,r,0,a,i)}this.cumulativeWeight=o}accumulateAdditive(e){let t=this.buffer,n=this.valueSize,i=n*this._addIndex;this.cumulativeWeightAdditive===0&&this._setIdentity(),this._mixBufferRegionAdditive(t,i,0,e,n),this.cumulativeWeightAdditive+=e}apply(e){let t=this.valueSize,n=this.buffer,i=e*t+t,r=this.cumulativeWeight,o=this.cumulativeWeightAdditive,a=this.binding;if(this.cumulativeWeight=0,this.cumulativeWeightAdditive=0,r<1){let l=t*this._origIndex;this._mixBufferRegion(n,i,l,1-r,t)}o>0&&this._mixBufferRegionAdditive(n,i,this._addIndex*t,1,t);for(let l=t,c=t+t;l!==c;++l)if(n[l]!==n[l+t]){a.setValue(n,i);break}}saveOriginalState(){let e=this.binding,t=this.buffer,n=this.valueSize,i=n*this._origIndex;e.getValue(t,i);for(let r=n,o=i;r!==o;++r)t[r]=t[i+r%n];this._setIdentity(),this.cumulativeWeight=0,this.cumulativeWeightAdditive=0}restoreOriginalState(){let e=this.valueSize*3;this.binding.setValue(this.buffer,e)}_setAdditiveIdentityNumeric(){let e=this._addIndex*this.valueSize,t=e+this.valueSize;for(let n=e;n<t;n++)this.buffer[n]=0}_setAdditiveIdentityQuaternion(){this._setAdditiveIdentityNumeric(),this.buffer[this._addIndex*this.valueSize+3]=1}_setAdditiveIdentityOther(){let e=this._origIndex*this.valueSize,t=this._addIndex*this.valueSize;for(let n=0;n<this.valueSize;n++)this.buffer[t+n]=this.buffer[e+n]}_select(e,t,n,i,r){if(i>=.5)for(let o=0;o!==r;++o)e[t+o]=e[n+o]}_slerp(e,t,n,i){Zt.slerpFlat(e,t,e,t,e,n,i)}_slerpAdditive(e,t,n,i,r){let o=this._workIndex*r;Zt.multiplyQuaternionsFlat(e,o,e,t,e,n),Zt.slerpFlat(e,t,e,t,e,o,i)}_lerp(e,t,n,i,r){let o=1-i;for(let a=0;a!==r;++a){let l=t+a;e[l]=e[l]*o+e[n+a]*i}}_lerpAdditive(e,t,n,i,r){for(let o=0;o!==r;++o){let a=t+o;e[a]=e[a]+e[n+o]*i}}},Ah="\\[\\]\\.:\\/",Fm=new RegExp("["+Ah+"]","g"),Rh="[^"+Ah+"]",Om="[^"+Ah.replace("\\.","")+"]",Bm=/((?:WC+[\/:])*)/.source.replace("WC",Rh),zm=/(WCOD+)?/.source.replace("WCOD",Om),km=/(?:\.(WC+)(?:\[(.+)\])?)?/.source.replace("WC",Rh),Hm=/\.(WC+)(?:\[(.+)\])?/.source.replace("WC",Rh),Vm=new RegExp("^"+Bm+zm+km+Hm+"$"),Gm=["material","materials","bones","map"],ch=class{constructor(e,t,n){let i=n||Ct.parseTrackName(t);this._targetGroup=e,this._bindings=e.subscribe_(t,i)}getValue(e,t){this.bind();let n=this._targetGroup.nCachedObjects_,i=this._bindings[n];i!==void 0&&i.getValue(e,t)}setValue(e,t){let n=this._bindings;for(let i=this._targetGroup.nCachedObjects_,r=n.length;i!==r;++i)n[i].setValue(e,t)}bind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].bind()}unbind(){let e=this._bindings;for(let t=this._targetGroup.nCachedObjects_,n=e.length;t!==n;++t)e[t].unbind()}},Ct=class s{constructor(e,t,n){this.path=t,this.parsedPath=n||s.parseTrackName(t),this.node=s.findNode(e,this.parsedPath.nodeName),this.rootNode=e,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}static create(e,t,n){return e&&e.isAnimationObjectGroup?new s.Composite(e,t,n):new s(e,t,n)}static sanitizeNodeName(e){return e.replace(/\s/g,"_").replace(Fm,"")}static parseTrackName(e){let t=Vm.exec(e);if(t===null)throw new Error("THREE.PropertyBinding: Cannot parse trackName: "+e);let n={nodeName:t[2],objectName:t[3],objectIndex:t[4],propertyName:t[5],propertyIndex:t[6]},i=n.nodeName&&n.nodeName.lastIndexOf(".");if(i!==void 0&&i!==-1){let r=n.nodeName.substring(i+1);Gm.indexOf(r)!==-1&&(n.nodeName=n.nodeName.substring(0,i),n.objectName=r)}if(n.propertyName===null||n.propertyName.length===0)throw new Error("THREE.PropertyBinding: can not parse propertyName from trackName: "+e);return n}static findNode(e,t){if(t===void 0||t===""||t==="."||t===-1||t===e.name||t===e.uuid)return e;if(e.skeleton){let n=e.skeleton.getBoneByName(t);if(n!==void 0)return n}if(e.children){let n=function(r){for(let o=0;o<r.length;o++){let a=r[o];if(a.name===t||a.uuid===t)return a;let l=n(a.children);if(l)return l}return null},i=n(e.children);if(i)return i}return null}_getValue_unavailable(){}_setValue_unavailable(){}_getValue_direct(e,t){e[t]=this.targetObject[this.propertyName]}_getValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)e[t++]=n[i]}_getValue_arrayElement(e,t){e[t]=this.resolvedProperty[this.propertyIndex]}_getValue_toArray(e,t){this.resolvedProperty.toArray(e,t)}_setValue_direct(e,t){this.targetObject[this.propertyName]=e[t]}_setValue_direct_setNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.needsUpdate=!0}_setValue_direct_setMatrixWorldNeedsUpdate(e,t){this.targetObject[this.propertyName]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_array(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++]}_setValue_array_setNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.needsUpdate=!0}_setValue_array_setMatrixWorldNeedsUpdate(e,t){let n=this.resolvedProperty;for(let i=0,r=n.length;i!==r;++i)n[i]=e[t++];this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_arrayElement(e,t){this.resolvedProperty[this.propertyIndex]=e[t]}_setValue_arrayElement_setNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.needsUpdate=!0}_setValue_arrayElement_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty[this.propertyIndex]=e[t],this.targetObject.matrixWorldNeedsUpdate=!0}_setValue_fromArray(e,t){this.resolvedProperty.fromArray(e,t)}_setValue_fromArray_setNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.needsUpdate=!0}_setValue_fromArray_setMatrixWorldNeedsUpdate(e,t){this.resolvedProperty.fromArray(e,t),this.targetObject.matrixWorldNeedsUpdate=!0}_getValue_unbound(e,t){this.bind(),this.getValue(e,t)}_setValue_unbound(e,t){this.bind(),this.setValue(e,t)}bind(){let e=this.node,t=this.parsedPath,n=t.objectName,i=t.propertyName,r=t.propertyIndex;if(e||(e=s.findNode(this.rootNode,t.nodeName),this.node=e),this.getValue=this._getValue_unavailable,this.setValue=this._setValue_unavailable,!e){We("PropertyBinding: No target node found for track: "+this.path+".");return}if(n){let c=t.objectIndex;switch(n){case"materials":if(!e.material){$e("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.materials){$e("PropertyBinding: Can not bind to material.materials as node.material does not have a materials array.",this);return}e=e.material.materials;break;case"bones":if(!e.skeleton){$e("PropertyBinding: Can not bind to bones as node does not have a skeleton.",this);return}e=e.skeleton.bones;for(let h=0;h<e.length;h++)if(e[h].name===c){c=h;break}break;case"map":if("map"in e){e=e.map;break}if(!e.material){$e("PropertyBinding: Can not bind to material as node does not have a material.",this);return}if(!e.material.map){$e("PropertyBinding: Can not bind to material.map as node.material does not have a map.",this);return}e=e.material.map;break;default:if(e[n]===void 0){$e("PropertyBinding: Can not bind to objectName of node undefined.",this);return}e=e[n]}if(c!==void 0){if(e[c]===void 0){$e("PropertyBinding: Trying to bind to objectIndex of objectName, but is undefined.",this,e);return}e=e[c]}}let o=e[i];if(o===void 0){let c=t.nodeName;$e("PropertyBinding: Trying to update property for track: "+c+"."+i+" but it wasn't found.",e);return}let a=this.Versioning.None;this.targetObject=e,e.isMaterial===!0?a=this.Versioning.NeedsUpdate:e.isObject3D===!0&&(a=this.Versioning.MatrixWorldNeedsUpdate);let l=this.BindingType.Direct;if(r!==void 0){if(i==="morphTargetInfluences"){if(!e.geometry){$e("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.",this);return}if(!e.geometry.morphAttributes){$e("PropertyBinding: Can not bind to morphTargetInfluences because node does not have a geometry.morphAttributes.",this);return}e.morphTargetDictionary[r]!==void 0&&(r=e.morphTargetDictionary[r])}l=this.BindingType.ArrayElement,this.resolvedProperty=o,this.propertyIndex=r}else o.fromArray!==void 0&&o.toArray!==void 0?(l=this.BindingType.HasFromToArray,this.resolvedProperty=o):Array.isArray(o)?(l=this.BindingType.EntireArray,this.resolvedProperty=o):this.propertyName=i;this.getValue=this.GetterByBindingType[l],this.setValue=this.SetterByBindingTypeAndVersioning[l][a]}unbind(){this.node=null,this.getValue=this._getValue_unbound,this.setValue=this._setValue_unbound}};Ct.Composite=ch;Ct.prototype.BindingType={Direct:0,EntireArray:1,ArrayElement:2,HasFromToArray:3};Ct.prototype.Versioning={None:0,NeedsUpdate:1,MatrixWorldNeedsUpdate:2};Ct.prototype.GetterByBindingType=[Ct.prototype._getValue_direct,Ct.prototype._getValue_array,Ct.prototype._getValue_arrayElement,Ct.prototype._getValue_toArray];Ct.prototype.SetterByBindingTypeAndVersioning=[[Ct.prototype._setValue_direct,Ct.prototype._setValue_direct_setNeedsUpdate,Ct.prototype._setValue_direct_setMatrixWorldNeedsUpdate],[Ct.prototype._setValue_array,Ct.prototype._setValue_array_setNeedsUpdate,Ct.prototype._setValue_array_setMatrixWorldNeedsUpdate],[Ct.prototype._setValue_arrayElement,Ct.prototype._setValue_arrayElement_setNeedsUpdate,Ct.prototype._setValue_arrayElement_setMatrixWorldNeedsUpdate],[Ct.prototype._setValue_fromArray,Ct.prototype._setValue_fromArray_setNeedsUpdate,Ct.prototype._setValue_fromArray_setMatrixWorldNeedsUpdate]];var ll=class{constructor(e,t,n=null,i=t.blendMode){this._mixer=e,this._clip=t,this._localRoot=n,this.blendMode=i;let r=t.tracks,o=r.length,a=new Array(o),l={endingStart:ys,endingEnd:ys};for(let c=0;c!==o;++c){let h=r[c].createInterpolant(null);a[c]=h,h.settings=l}this._interpolantSettings=l,this._interpolants=a,this._propertyBindings=new Array(o),this._cacheIndex=null,this._byClipCacheIndex=null,this._timeScaleInterpolant=null,this._restoreTimeScale=null,this._weightInterpolant=null,this.loop=Yd,this._loopCount=-1,this._startTime=null,this.time=0,this.timeScale=1,this._effectiveTimeScale=1,this.weight=1,this._effectiveWeight=1,this.repetitions=1/0,this.paused=!1,this.enabled=!0,this.clampWhenFinished=!1,this.zeroSlopeAtStart=!0,this.zeroSlopeAtEnd=!0}play(){return this._mixer._activateAction(this),this}stop(){return this._mixer._deactivateAction(this),this.reset()}reset(){return this.paused=!1,this.enabled=!0,this.time=0,this._loopCount=-1,this._startTime=null,this.stopFading().stopWarping()}isRunning(){return this.enabled&&!this.paused&&this.timeScale!==0&&this._startTime===null&&this._mixer._isActiveAction(this)}isScheduled(){return this._mixer._isActiveAction(this)}startAt(e){return this._startTime=e,this}setLoop(e,t){return this.loop=e,this.repetitions=t,this}setEffectiveWeight(e){return this.weight=e,this._effectiveWeight=this.enabled?e:0,this.stopFading()}getEffectiveWeight(){return this._effectiveWeight}fadeIn(e){return this._scheduleFading(e,0,1)}fadeOut(e){return this._scheduleFading(e,1,0)}crossFadeFrom(e,t,n=!1){if(e.fadeOut(t),this.fadeIn(t),n===!0){let i=this._clip.duration,r=e._clip.duration,o=r/i,a=i/r;e._restoreTimeScale=e.timeScale,this._restoreTimeScale=this.timeScale,e.warp(1,o,t),this.warp(a,1,t)}return this}crossFadeTo(e,t,n=!1){return e.crossFadeFrom(this,t,n)}stopFading(){let e=this._weightInterpolant;return e!==null&&(this._weightInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this}setEffectiveTimeScale(e){return this.timeScale=e,this._effectiveTimeScale=this.paused?0:e,this.stopWarping()}getEffectiveTimeScale(){return this._effectiveTimeScale}setDuration(e){return this.timeScale=this._clip.duration/e,this.stopWarping()}syncWith(e){return this.time=e.time,this.timeScale=e.timeScale,this.stopWarping()}halt(e){return this.warp(this._effectiveTimeScale,0,e)}warp(e,t,n){let i=this._mixer,r=i.time,o=this.timeScale,a=this._timeScaleInterpolant;a===null&&(a=i._lendControlInterpolant(),this._timeScaleInterpolant=a);let l=a.parameterPositions,c=a.sampleValues;return l[0]=r,l[1]=r+n,c[0]=e/o,c[1]=t/o,this}stopWarping(){let e=this._timeScaleInterpolant;return e!==null&&(this._timeScaleInterpolant=null,this._mixer._takeBackControlInterpolant(e)),this._restoreTimeScale=null,this}getMixer(){return this._mixer}getClip(){return this._clip}getRoot(){return this._localRoot||this._mixer._root}_update(e,t,n,i){if(!this.enabled){this._updateWeight(e);return}let r=this._startTime;if(r!==null){let l=(e-r)*n;l<0||n===0?t=0:(this._startTime=null,t=n*l)}t*=this._updateTimeScale(e);let o=this._updateTime(t),a=this._updateWeight(e);if(a>0){let l=this._interpolants,c=this._propertyBindings;switch(this.blendMode){case Kd:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulateAdditive(a);break;case $l:default:for(let h=0,u=l.length;h!==u;++h)l[h].evaluate(o),c[h].accumulate(i,a)}}}_updateWeight(e){let t=0;if(this.enabled){t=this.weight;let n=this._weightInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(this.stopFading(),i===0&&(this.enabled=!1))}}return this._effectiveWeight=t,t}_updateTimeScale(e){let t=0;if(!this.paused){t=this.timeScale;let n=this._timeScaleInterpolant;if(n!==null){let i=n.evaluate(e)[0];t*=i,e>n.parameterPositions[1]&&(t===0?this.paused=!0:(this._restoreTimeScale!==null&&(t=this._restoreTimeScale),this.timeScale=t),this.stopWarping())}}return this._effectiveTimeScale=t,t}_updateTime(e){let t=this._clip.duration,n=this.loop,i=this.time+e,r=this._loopCount,o=n===Zd;if(e===0)return r===-1?i:o&&(r&1)===1?t-i:i;if(n===Jl){r===-1&&(this._loopCount=0,this._setEndings(!0,!0,!1));e:{if(i>=t)i=t;else if(i<0)i=0;else{this.time=i;break e}this.clampWhenFinished?this.paused=!0:this.enabled=!1,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e<0?-1:1})}}else{if(r===-1&&(e>=0?(r=0,this._setEndings(!0,this.repetitions===0,o)):this._setEndings(this.repetitions===0,!0,o)),i>=t||i<0){let a=Math.floor(i/t);i-=t*a,r+=Math.abs(a);let l=this.repetitions-r;if(l<=0)this.clampWhenFinished?this.paused=!0:this.enabled=!1,i=e>0?t:0,this.time=i,this._mixer.dispatchEvent({type:"finished",action:this,direction:e>0?1:-1});else{if(l===1){let c=e<0;this._setEndings(c,!c,o)}else this._setEndings(!1,!1,o);this._loopCount=r,this.time=i,this._mixer.dispatchEvent({type:"loop",action:this,loopDelta:a})}}else this._loopCount=r,this.time=i;if(o&&(r&1)===1)return t-i}return i}_setEndings(e,t,n){let i=this._interpolantSettings;n?(i.endingStart=Ms,i.endingEnd=Ms):(e?i.endingStart=this.zeroSlopeAtStart?Ms:ys:i.endingStart=Qr,t?i.endingEnd=this.zeroSlopeAtEnd?Ms:ys:i.endingEnd=Qr)}_scheduleFading(e,t,n){let i=this._mixer,r=i.time,o=this._weightInterpolant;o===null&&(o=i._lendControlInterpolant(),this._weightInterpolant=o);let a=o.parameterPositions,l=o.sampleValues;return a[0]=r,l[0]=t,a[1]=r+e,l[1]=n,this}},Wm=new Float32Array(1),To=class extends Vn{constructor(e){super(),this._root=e,this._initMemoryManager(),this._accuIndex=0,this.time=0,this.timeScale=1,typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}_bindAction(e,t){let n=e._localRoot||this._root,i=e._clip.tracks,r=i.length,o=e._propertyBindings,a=e._interpolants,l=n.uuid,c=this._bindingsByRootAndName,h=c[l];h===void 0&&(h={},c[l]=h);for(let u=0;u!==r;++u){let d=i[u],f=d.name,g=h[f];if(g!==void 0)++g.referenceCount,o[u]=g;else{if(g=o[u],g!==void 0){g._cacheIndex===null&&(++g.referenceCount,this._addInactiveBinding(g,l,f));continue}let y=t&&t._propertyBindings[u].binding.parsedPath;g=new al(Ct.create(n,f,y),d.ValueTypeName,d.getValueSize()),++g.referenceCount,this._addInactiveBinding(g,l,f),o[u]=g}a[u].resultBuffer=g.buffer}}_activateAction(e){if(!this._isActiveAction(e)){if(e._cacheIndex===null){let n=(e._localRoot||this._root).uuid,i=e._clip.uuid,r=this._actionsByClip[i];this._bindAction(e,r&&r.knownActions[0]),this._addInactiveAction(e,i,n)}let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];r.useCount++===0&&(this._lendBinding(r),r.saveOriginalState())}this._lendAction(e)}}_deactivateAction(e){if(this._isActiveAction(e)){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.useCount===0&&(r.restoreOriginalState(),this._takeBackBinding(r))}this._takeBackAction(e)}}_initMemoryManager(){this._actions=[],this._nActiveActions=0,this._actionsByClip={},this._bindings=[],this._nActiveBindings=0,this._bindingsByRootAndName={},this._controlInterpolants=[],this._nActiveControlInterpolants=0;let e=this;this.stats={actions:{get total(){return e._actions.length},get inUse(){return e._nActiveActions}},bindings:{get total(){return e._bindings.length},get inUse(){return e._nActiveBindings}},controlInterpolants:{get total(){return e._controlInterpolants.length},get inUse(){return e._nActiveControlInterpolants}}}}_isActiveAction(e){let t=e._cacheIndex;return t!==null&&t<this._nActiveActions}_addInactiveAction(e,t,n){let i=this._actions,r=this._actionsByClip,o=r[t];if(o===void 0)o={knownActions:[e],actionByRoot:{}},e._byClipCacheIndex=0,r[t]=o;else{let a=o.knownActions;e._byClipCacheIndex=a.length,a.push(e)}e._cacheIndex=i.length,i.push(e),o.actionByRoot[n]=e}_removeInactiveAction(e){let t=this._actions,n=t[t.length-1],i=e._cacheIndex;n._cacheIndex=i,t[i]=n,t.pop(),e._cacheIndex=null;let r=e._clip.uuid,o=this._actionsByClip,a=o[r],l=a.knownActions,c=l[l.length-1],h=e._byClipCacheIndex;c._byClipCacheIndex=h,l[h]=c,l.pop(),e._byClipCacheIndex=null;let u=a.actionByRoot,d=(e._localRoot||this._root).uuid;delete u[d],l.length===0&&delete o[r],this._removeInactiveBindingsForAction(e)}_removeInactiveBindingsForAction(e){let t=e._propertyBindings;for(let n=0,i=t.length;n!==i;++n){let r=t[n];--r.referenceCount===0&&this._removeInactiveBinding(r)}}_lendAction(e){let t=this._actions,n=e._cacheIndex,i=this._nActiveActions++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackAction(e){let t=this._actions,n=e._cacheIndex,i=--this._nActiveActions,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_addInactiveBinding(e,t,n){let i=this._bindingsByRootAndName,r=this._bindings,o=i[t];o===void 0&&(o={},i[t]=o),o[n]=e,e._cacheIndex=r.length,r.push(e)}_removeInactiveBinding(e){let t=this._bindings,n=e.binding,i=n.rootNode.uuid,r=n.path,o=this._bindingsByRootAndName,a=o[i],l=t[t.length-1],c=e._cacheIndex;l._cacheIndex=c,t[c]=l,t.pop(),delete a[r],Object.keys(a).length===0&&delete o[i]}_lendBinding(e){let t=this._bindings,n=e._cacheIndex,i=this._nActiveBindings++,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_takeBackBinding(e){let t=this._bindings,n=e._cacheIndex,i=--this._nActiveBindings,r=t[i];e._cacheIndex=i,t[i]=e,r._cacheIndex=n,t[n]=r}_lendControlInterpolant(){let e=this._controlInterpolants,t=this._nActiveControlInterpolants++,n=e[t];return n===void 0&&(n=new xo(new Float32Array(2),new Float32Array(2),1,Wm),n.__cacheIndex=t,e[t]=n),n}_takeBackControlInterpolant(e){let t=this._controlInterpolants,n=e.__cacheIndex,i=--this._nActiveControlInterpolants,r=t[i];e.__cacheIndex=i,t[i]=e,r.__cacheIndex=n,t[n]=r}clipAction(e,t,n){let i=t||this._root,r=i.uuid,o=typeof e=="string"?Ds.findByName(i,e):e,a=o!==null?o.uuid:e,l=this._actionsByClip[a],c=null;if(n===void 0&&(o!==null?n=o.blendMode:n=$l),l!==void 0){let u=l.actionByRoot[r];if(u!==void 0&&u.blendMode===n)return u;c=l.knownActions[0],o===null&&(o=c._clip)}if(o===null)return null;let h=new ll(this,o,t,n);return this._bindAction(h,c),this._addInactiveAction(h,a,r),h}existingAction(e,t){let n=t||this._root,i=n.uuid,r=typeof e=="string"?Ds.findByName(n,e):e,o=r?r.uuid:e,a=this._actionsByClip[o];return a!==void 0&&a.actionByRoot[i]||null}stopAllAction(){let e=this._actions,t=this._nActiveActions;for(let n=t-1;n>=0;--n)e[n].stop();return this}update(e){e*=this.timeScale;let t=this._actions,n=this._nActiveActions,i=this.time+=e,r=Math.sign(e),o=this._accuIndex^=1;for(let c=0;c!==n;++c)t[c]._update(i,e,r,o);let a=this._bindings,l=this._nActiveBindings;for(let c=0;c!==l;++c)a[c].apply(o);return this}setTime(e){this.time=0;for(let t=0;t<this._actions.length;t++)this._actions[t].time=0;return this.update(e)}getRoot(){return this._root}uncacheClip(e){let t=this._actions,n=e.uuid,i=this._actionsByClip,r=i[n];if(r!==void 0){let o=r.knownActions;for(let a=0,l=o.length;a!==l;++a){let c=o[a];this._deactivateAction(c);let h=c._cacheIndex,u=t[t.length-1];c._cacheIndex=null,c._byClipCacheIndex=null,u._cacheIndex=h,t[h]=u,t.pop(),this._removeInactiveBindingsForAction(c)}delete i[n]}}uncacheRoot(e){let t=e.uuid,n=this._actionsByClip;for(let o in n){let a=n[o].actionByRoot,l=a[t];l!==void 0&&(this._deactivateAction(l),this._removeInactiveAction(l))}let i=this._bindingsByRootAndName,r=i[t];if(r!==void 0)for(let o in r){let a=r[o];a.restoreOriginalState(),this._removeInactiveBinding(a)}}uncacheAction(e,t){let n=this.existingAction(e,t);n!==null&&(this._deactivateAction(n),this._removeInactiveAction(n))}};var bd=new Qe,Ao=class{constructor(e,t,n=0,i=1/0){this.ray=new fi(e,t),this.near=n,this.far=i,this.camera=null,this.layers=new dr,this.params={Mesh:{},Line:{threshold:1},LOD:{},Points:{threshold:1},Sprite:{}}}set(e,t){this.ray.set(e,t)}setFromCamera(e,t){t.isPerspectiveCamera?(this.ray.origin.setFromMatrixPosition(t.matrixWorld),this.ray.direction.set(e.x,e.y,.5).unproject(t).sub(this.ray.origin).normalize(),this.camera=t):t.isOrthographicCamera?(this.ray.origin.set(e.x,e.y,t.projectionMatrix.elements[14]).unproject(t),this.ray.direction.set(0,0,-1).transformDirection(t.matrixWorld),this.camera=t):$e("Raycaster: Unsupported camera type: "+t.type)}setFromXRController(e){return bd.identity().extractRotation(e.matrixWorld),this.ray.origin.setFromMatrixPosition(e.matrixWorld),this.ray.direction.set(0,0,-1).applyMatrix4(bd),this}intersectObject(e,t=!0,n=[]){return hh(e,this,n,t),n.sort(Sd),n}intersectObjects(e,t=!0,n=[]){for(let i=0,r=e.length;i<r;i++)hh(e[i],this,n,t);return n.sort(Sd),n}};function Sd(s,e){return s.distance-e.distance}function hh(s,e,t,n){let i=!0;if(s.layers.test(e.layers)&&s.raycast(e,t)===!1&&(i=!1),i===!0&&n===!0){let r=s.children;for(let o=0,a=r.length;o<a;o++)hh(r[o],e,t,!0)}}var Er=class{constructor(e=1,t=0,n=0){this.radius=e,this.phi=t,this.theta=n}set(e,t,n){return this.radius=e,this.phi=t,this.theta=n,this}copy(e){return this.radius=e.radius,this.phi=e.phi,this.theta=e.theta,this}makeSafe(){return this.phi=at(this.phi,1e-6,Math.PI-1e-6),this}setFromVector3(e){return this.setFromCartesianCoords(e.x,e.y,e.z)}setFromCartesianCoords(e,t,n){return this.radius=Math.sqrt(e*e+t*t+n*n),this.radius===0?(this.theta=0,this.phi=0):(this.theta=Math.atan2(e,n),this.phi=Math.acos(at(t/this.radius,-1,1))),this}clone(){return new this.constructor().copy(this)}};var uh=class s{static{s.prototype.isMatrix2=!0}constructor(e,t,n,i){this.elements=[1,0,0,1],e!==void 0&&this.set(e,t,n,i)}identity(){return this.set(1,0,0,1),this}fromArray(e,t=0){for(let n=0;n<4;n++)this.elements[n]=e[n+t];return this}set(e,t,n,i){let r=this.elements;return r[0]=e,r[2]=t,r[1]=n,r[3]=i,this}};var Ro=class extends Vn{constructor(e,t=null){super(),this.object=e,this.domElement=t,this.enabled=!0,this.state=-1,this.keys={},this.mouseButtons={LEFT:null,MIDDLE:null,RIGHT:null},this.touches={ONE:null,TWO:null}}connect(e){if(e===void 0){We("Controls: connect() now requires an element.");return}this.domElement!==null&&this.disconnect(),this.domElement=e}disconnect(){}dispose(){}update(){}};function Ch(s,e,t,n){let i=Xm(n);switch(t){case yh:return s*e;case xl:return s*e/i.components*i.byteLength;case _l:return s*e/i.components*i.byteLength;case cs:return s*e*2/i.components*i.byteLength;case vl:return s*e*2/i.components*i.byteLength;case Mh:return s*e*3/i.components*i.byteLength;case Fn:return s*e*4/i.components*i.byteLength;case yl:return s*e*4/i.components*i.byteLength;case Oo:case Bo:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case zo:case ko:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case bl:case wl:return Math.max(s,16)*Math.max(e,8)/4;case Ml:case Sl:return Math.max(s,8)*Math.max(e,8)/2;case El:case Tl:case Rl:case Cl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*8;case Al:case Ho:case Pl:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Il:return Math.floor((s+3)/4)*Math.floor((e+3)/4)*16;case Ll:return Math.floor((s+4)/5)*Math.floor((e+3)/4)*16;case Dl:return Math.floor((s+4)/5)*Math.floor((e+4)/5)*16;case Nl:return Math.floor((s+5)/6)*Math.floor((e+4)/5)*16;case Ul:return Math.floor((s+5)/6)*Math.floor((e+5)/6)*16;case Fl:return Math.floor((s+7)/8)*Math.floor((e+4)/5)*16;case Ol:return Math.floor((s+7)/8)*Math.floor((e+5)/6)*16;case Bl:return Math.floor((s+7)/8)*Math.floor((e+7)/8)*16;case zl:return Math.floor((s+9)/10)*Math.floor((e+4)/5)*16;case kl:return Math.floor((s+9)/10)*Math.floor((e+5)/6)*16;case Hl:return Math.floor((s+9)/10)*Math.floor((e+7)/8)*16;case Vl:return Math.floor((s+9)/10)*Math.floor((e+9)/10)*16;case Gl:return Math.floor((s+11)/12)*Math.floor((e+9)/10)*16;case Wl:return Math.floor((s+11)/12)*Math.floor((e+11)/12)*16;case Xl:case ql:case Yl:return Math.ceil(s/4)*Math.ceil(e/4)*16;case Zl:case Kl:return Math.ceil(s/4)*Math.ceil(e/4)*8;case Vo:case jl:return Math.ceil(s/4)*Math.ceil(e/4)*16}throw new Error(`Unable to determine texture byte length for ${t} format.`)}function Xm(s){switch(s){case Sn:case gh:return{byteLength:1,components:1};case Rr:case xh:case nn:return{byteLength:2,components:1};case ml:case gl:return{byteLength:2,components:4};case ri:case pl:case Un:return{byteLength:4,components:1};case _h:case vh:return{byteLength:4,components:3}}throw new Error(`THREE.TextureUtils: Unknown texture type ${s}.`)}typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("register",{detail:{revision:"185"}}));typeof window<"u"&&(window.__THREE__?We("WARNING: Multiple instances of Three.js being imported."):window.__THREE__="185");function Df(){let s=null,e=!1,t=null,n=null;function i(r,o){t(r,o),n=s.requestAnimationFrame(i)}return{start:function(){e!==!0&&t!==null&&s!==null&&(n=s.requestAnimationFrame(i),e=!0)},stop:function(){s!==null&&s.cancelAnimationFrame(n),e=!1},setAnimationLoop:function(r){t=r},setContext:function(r){s=r}}}function Ym(s){let e=new WeakMap;function t(a,l){let c=a.array,h=a.usage,u=c.byteLength,d=s.createBuffer();s.bindBuffer(l,d),s.bufferData(l,c,h),a.onUploadCallback();let f;if(c instanceof Float32Array)f=s.FLOAT;else if(typeof Float16Array<"u"&&c instanceof Float16Array)f=s.HALF_FLOAT;else if(c instanceof Uint16Array)a.isFloat16BufferAttribute?f=s.HALF_FLOAT:f=s.UNSIGNED_SHORT;else if(c instanceof Int16Array)f=s.SHORT;else if(c instanceof Uint32Array)f=s.UNSIGNED_INT;else if(c instanceof Int32Array)f=s.INT;else if(c instanceof Int8Array)f=s.BYTE;else if(c instanceof Uint8Array)f=s.UNSIGNED_BYTE;else if(c instanceof Uint8ClampedArray)f=s.UNSIGNED_BYTE;else throw new Error("THREE.WebGLAttributes: Unsupported buffer data format: "+c);return{buffer:d,type:f,bytesPerElement:c.BYTES_PER_ELEMENT,version:a.version,size:u}}function n(a,l,c){let h=l.array,u=l.updateRanges;if(s.bindBuffer(c,a),u.length===0)s.bufferSubData(c,0,h);else{u.sort((f,g)=>f.start-g.start);let d=0;for(let f=1;f<u.length;f++){let g=u[d],y=u[f];y.start<=g.start+g.count+1?g.count=Math.max(g.count,y.start+y.count-g.start):(++d,u[d]=y)}u.length=d+1;for(let f=0,g=u.length;f<g;f++){let y=u[f];s.bufferSubData(c,y.start*h.BYTES_PER_ELEMENT,h,y.start,y.count)}l.clearUpdateRanges()}l.onUploadCallback()}function i(a){return a.isInterleavedBufferAttribute&&(a=a.data),e.get(a)}function r(a){a.isInterleavedBufferAttribute&&(a=a.data);let l=e.get(a);l&&(s.deleteBuffer(l.buffer),e.delete(a))}function o(a,l){if(a.isInterleavedBufferAttribute&&(a=a.data),a.isGLBufferAttribute){let h=e.get(a);(!h||h.version<a.version)&&e.set(a,{buffer:a.buffer,type:a.type,bytesPerElement:a.elementSize,version:a.version});return}let c=e.get(a);if(c===void 0)e.set(a,t(a,l));else if(c.version<a.version){if(c.size!==a.array.byteLength)throw new Error("THREE.WebGLAttributes: The size of the buffer attribute's array buffer does not match the original size. Resizing buffer attributes is not supported.");n(c.buffer,a,l),c.version=a.version}}return{get:i,remove:r,update:o}}var Zm=`#ifdef USE_ALPHAHASH
	if ( diffuseColor.a < getAlphaHashThreshold( vPosition ) ) discard;
#endif`,Km=`#ifdef USE_ALPHAHASH
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
#endif`,jm=`#ifdef USE_ALPHAMAP
	diffuseColor.a *= texture2D( alphaMap, vAlphaMapUv ).g;
#endif`,Jm=`#ifdef USE_ALPHAMAP
	uniform sampler2D alphaMap;
#endif`,$m=`#ifdef USE_ALPHATEST
	#ifdef ALPHA_TO_COVERAGE
	diffuseColor.a = smoothstep( alphaTest, alphaTest + fwidth( diffuseColor.a ), diffuseColor.a );
	if ( diffuseColor.a == 0.0 ) discard;
	#else
	if ( diffuseColor.a < alphaTest ) discard;
	#endif
#endif`,Qm=`#ifdef USE_ALPHATEST
	uniform float alphaTest;
#endif`,eg=`#ifdef USE_AOMAP
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
#endif`,tg=`#ifdef USE_AOMAP
	uniform sampler2D aoMap;
	uniform float aoMapIntensity;
#endif`,ng=`#ifdef USE_BATCHING
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
#endif`,ig=`#ifdef USE_BATCHING
	mat4 batchingMatrix = getBatchingMatrix( getIndirectIndex( gl_DrawID ) );
#endif`,sg=`vec3 transformed = vec3( position );
#ifdef USE_ALPHAHASH
	vPosition = vec3( position );
#endif`,rg=`vec3 objectNormal = vec3( normal );
#ifdef USE_TANGENT
	vec3 objectTangent = vec3( tangent.xyz );
#endif`,og=`float G_BlinnPhong_Implicit( ) {
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
} // validated`,ag=`#ifdef USE_IRIDESCENCE
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
#endif`,lg=`#ifdef USE_BUMPMAP
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
#endif`,cg=`#if NUM_CLIPPING_PLANES > 0
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
#endif`,hg=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
	uniform vec4 clippingPlanes[ NUM_CLIPPING_PLANES ];
#endif`,ug=`#if NUM_CLIPPING_PLANES > 0
	varying vec3 vClipPosition;
#endif`,dg=`#if NUM_CLIPPING_PLANES > 0
	vClipPosition = - mvPosition.xyz;
#endif`,fg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	diffuseColor *= vColor;
#endif`,pg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA )
	varying vec4 vColor;
#endif`,mg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
	varying vec4 vColor;
#endif`,gg=`#if defined( USE_COLOR ) || defined( USE_COLOR_ALPHA ) || defined( USE_INSTANCING_COLOR ) || defined( USE_BATCHING_COLOR )
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
#endif`,xg=`#define PI 3.141592653589793
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
} // validated`,_g=`#ifdef ENVMAP_TYPE_CUBE_UV
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
#endif`,vg=`vec3 transformedNormal = objectNormal;
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
#endif`,yg=`#ifdef USE_DISPLACEMENTMAP
	uniform sampler2D displacementMap;
	uniform float displacementScale;
	uniform float displacementBias;
#endif`,Mg=`#ifdef USE_DISPLACEMENTMAP
	transformed += normalize( objectNormal ) * ( texture2D( displacementMap, vDisplacementMapUv ).x * displacementScale + displacementBias );
#endif`,bg=`#ifdef USE_EMISSIVEMAP
	vec4 emissiveColor = texture2D( emissiveMap, vEmissiveMapUv );
	#ifdef DECODE_VIDEO_TEXTURE_EMISSIVE
		emissiveColor = sRGBTransferEOTF( emissiveColor );
	#endif
	totalEmissiveRadiance *= emissiveColor.rgb;
#endif`,Sg=`#ifdef USE_EMISSIVEMAP
	uniform sampler2D emissiveMap;
#endif`,wg="gl_FragColor = linearToOutputTexel( gl_FragColor );",Eg=`vec4 LinearTransferOETF( in vec4 value ) {
	return value;
}
vec4 sRGBTransferEOTF( in vec4 value ) {
	return vec4( mix( pow( value.rgb * 0.9478672986 + vec3( 0.0521327014 ), vec3( 2.4 ) ), value.rgb * 0.0773993808, vec3( lessThanEqual( value.rgb, vec3( 0.04045 ) ) ) ), value.a );
}
vec4 sRGBTransferOETF( in vec4 value ) {
	return vec4( mix( pow( value.rgb, vec3( 0.41666 ) ) * 1.055 - vec3( 0.055 ), value.rgb * 12.92, vec3( lessThanEqual( value.rgb, vec3( 0.0031308 ) ) ) ), value.a );
}`,Tg=`#ifdef USE_ENVMAP
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
#endif`,Ag=`#ifdef USE_ENVMAP
	uniform float envMapIntensity;
	uniform mat3 envMapRotation;
	#ifdef ENVMAP_TYPE_CUBE
		uniform samplerCube envMap;
	#else
		uniform sampler2D envMap;
	#endif
#endif`,Rg=`#ifdef USE_ENVMAP
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
#endif`,Cg=`#ifdef USE_ENVMAP
	#if defined( USE_BUMPMAP ) || defined( USE_NORMALMAP ) || defined( PHONG ) || defined( LAMBERT )
		#define ENV_WORLDPOS
	#endif
	#ifdef ENV_WORLDPOS
		
		varying vec3 vWorldPosition;
	#else
		varying vec3 vReflect;
		uniform float refractionRatio;
	#endif
#endif`,Pg=`#ifdef USE_ENVMAP
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
#endif`,Ig=`#ifdef USE_FOG
	vFogDepth = - mvPosition.z;
#endif`,Lg=`#ifdef USE_FOG
	varying float vFogDepth;
#endif`,Dg=`#ifdef USE_FOG
	#ifdef FOG_EXP2
		float fogFactor = 1.0 - exp( - fogDensity * fogDensity * vFogDepth * vFogDepth );
	#else
		float fogFactor = smoothstep( fogNear, fogFar, vFogDepth );
	#endif
	gl_FragColor.rgb = mix( gl_FragColor.rgb, fogColor, fogFactor );
#endif`,Ng=`#ifdef USE_FOG
	uniform vec3 fogColor;
	varying float vFogDepth;
	#ifdef FOG_EXP2
		uniform float fogDensity;
	#else
		uniform float fogNear;
		uniform float fogFar;
	#endif
#endif`,Ug=`#ifdef USE_GRADIENTMAP
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
}`,Fg=`#ifdef USE_LIGHTMAP
	uniform sampler2D lightMap;
	uniform float lightMapIntensity;
#endif`,Og=`LambertMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularStrength = specularStrength;`,Bg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Lambert`,zg=`uniform bool receiveShadow;
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
#include <lightprobes_pars_fragment>`,kg=`#ifdef USE_ENVMAP
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
#endif`,Hg=`ToonMaterial material;
material.diffuseColor = diffuseColor.rgb;`,Vg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_Toon`,Gg=`BlinnPhongMaterial material;
material.diffuseColor = diffuseColor.rgb;
material.specularColor = specular;
material.specularShininess = shininess;
material.specularStrength = specularStrength;`,Wg=`varying vec3 vViewPosition;
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
#define RE_IndirectDiffuse		RE_IndirectDiffuse_BlinnPhong`,Xg=`PhysicalMaterial material;
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
#endif`,qg=`uniform sampler2D dfgLUT;
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
}`,Yg=`
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
#endif`,Zg=`#if defined( RE_IndirectDiffuse )
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
#endif`,Kg=`#if defined( RE_IndirectDiffuse )
	#if defined( LAMBERT ) || defined( PHONG )
		irradiance += iblIrradiance;
	#endif
	RE_IndirectDiffuse( irradiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif
#if defined( RE_IndirectSpecular )
	RE_IndirectSpecular( radiance, iblIrradiance, clearcoatRadiance, geometryPosition, geometryNormal, geometryViewDir, geometryClearcoatNormal, material, reflectedLight );
#endif`,jg=`#ifdef USE_LIGHT_PROBES_GRID
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
#endif`,Jg=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	gl_FragDepth = vIsPerspective == 0.0 ? gl_FragCoord.z : log2( vFragDepth ) * logDepthBufFC * 0.5;
#endif`,$g=`#if defined( USE_LOGARITHMIC_DEPTH_BUFFER )
	uniform float logDepthBufFC;
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,Qg=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	varying float vFragDepth;
	varying float vIsPerspective;
#endif`,e0=`#ifdef USE_LOGARITHMIC_DEPTH_BUFFER
	vFragDepth = 1.0 + gl_Position.w;
	vIsPerspective = float( isPerspectiveMatrix( projectionMatrix ) );
#endif`,t0=`#ifdef USE_MAP
	vec4 sampledDiffuseColor = texture2D( map, vMapUv );
	#ifdef DECODE_VIDEO_TEXTURE
		sampledDiffuseColor = sRGBTransferEOTF( sampledDiffuseColor );
	#endif
	diffuseColor *= sampledDiffuseColor;
#endif`,n0=`#ifdef USE_MAP
	uniform sampler2D map;
#endif`,i0=`#if defined( USE_MAP ) || defined( USE_ALPHAMAP )
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
#endif`,s0=`#if defined( USE_POINTS_UV )
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
#endif`,r0=`float metalnessFactor = metalness;
#ifdef USE_METALNESSMAP
	vec4 texelMetalness = texture2D( metalnessMap, vMetalnessMapUv );
	metalnessFactor *= texelMetalness.b;
#endif`,o0=`#ifdef USE_METALNESSMAP
	uniform sampler2D metalnessMap;
#endif`,a0=`#ifdef USE_INSTANCING_MORPH
	float morphTargetInfluences[ MORPHTARGETS_COUNT ];
	float morphTargetBaseInfluence = texelFetch( morphTexture, ivec2( 0, gl_InstanceID ), 0 ).r;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		morphTargetInfluences[i] =  texelFetch( morphTexture, ivec2( i + 1, gl_InstanceID ), 0 ).r;
	}
#endif`,l0=`#if defined( USE_MORPHCOLORS )
	vColor *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		#if defined( USE_COLOR_ALPHA )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ) * morphTargetInfluences[ i ];
		#elif defined( USE_COLOR )
			if ( morphTargetInfluences[ i ] != 0.0 ) vColor += getMorph( gl_VertexID, i, 2 ).rgb * morphTargetInfluences[ i ];
		#endif
	}
#endif`,c0=`#ifdef USE_MORPHNORMALS
	objectNormal *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) objectNormal += getMorph( gl_VertexID, i, 1 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,h0=`#ifdef USE_MORPHTARGETS
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
#endif`,u0=`#ifdef USE_MORPHTARGETS
	transformed *= morphTargetBaseInfluence;
	for ( int i = 0; i < MORPHTARGETS_COUNT; i ++ ) {
		if ( morphTargetInfluences[ i ] != 0.0 ) transformed += getMorph( gl_VertexID, i, 0 ).xyz * morphTargetInfluences[ i ];
	}
#endif`,d0=`float faceDirection = gl_FrontFacing ? 1.0 : - 1.0;
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
vec3 nonPerturbedNormal = normal;`,f0=`#ifdef USE_NORMALMAP_OBJECTSPACE
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
#endif`,p0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,m0=`#ifndef FLAT_SHADED
	varying vec3 vNormal;
	#ifdef USE_TANGENT
		varying vec3 vTangent;
		varying vec3 vBitangent;
	#endif
#endif`,g0=`#ifndef FLAT_SHADED
	vNormal = normalize( transformedNormal );
	#ifdef USE_TANGENT
		vTangent = normalize( transformedTangent );
		vBitangent = normalize( cross( vNormal, vTangent ) * tangent.w );
		#ifdef FLIP_SIDED
			vBitangent = - vBitangent;
		#endif
	#endif
#endif`,x0=`#ifdef USE_NORMALMAP
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
#endif`,_0=`#ifdef USE_CLEARCOAT
	vec3 clearcoatNormal = nonPerturbedNormal;
#endif`,v0=`#ifdef USE_CLEARCOAT_NORMALMAP
	vec3 clearcoatMapN = texture2D( clearcoatNormalMap, vClearcoatNormalMapUv ).xyz * 2.0 - 1.0;
	clearcoatMapN.xy *= clearcoatNormalScale;
	clearcoatNormal = normalize( tbn2 * clearcoatMapN );
#endif`,y0=`#ifdef USE_CLEARCOATMAP
	uniform sampler2D clearcoatMap;
#endif
#ifdef USE_CLEARCOAT_NORMALMAP
	uniform sampler2D clearcoatNormalMap;
	uniform vec2 clearcoatNormalScale;
#endif
#ifdef USE_CLEARCOAT_ROUGHNESSMAP
	uniform sampler2D clearcoatRoughnessMap;
#endif`,M0=`#ifdef USE_IRIDESCENCEMAP
	uniform sampler2D iridescenceMap;
#endif
#ifdef USE_IRIDESCENCE_THICKNESSMAP
	uniform sampler2D iridescenceThicknessMap;
#endif`,b0=`#ifdef OPAQUE
diffuseColor.a = 1.0;
#endif
#ifdef USE_TRANSMISSION
diffuseColor.a *= material.transmissionAlpha;
#endif
gl_FragColor = vec4( outgoingLight, diffuseColor.a );`,S0=`vec3 packNormalToRGB( const in vec3 normal ) {
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
}`,w0=`#ifdef PREMULTIPLIED_ALPHA
	gl_FragColor.rgb *= gl_FragColor.a;
#endif`,E0=`vec4 mvPosition = vec4( transformed, 1.0 );
#ifdef USE_BATCHING
	mvPosition = batchingMatrix * mvPosition;
#endif
#ifdef USE_INSTANCING
	mvPosition = instanceMatrix * mvPosition;
#endif
mvPosition = modelViewMatrix * mvPosition;
gl_Position = projectionMatrix * mvPosition;`,T0=`#ifdef DITHERING
	gl_FragColor.rgb = dithering( gl_FragColor.rgb );
#endif`,A0=`#ifdef DITHERING
	vec3 dithering( vec3 color ) {
		float grid_position = rand( gl_FragCoord.xy );
		vec3 dither_shift_RGB = vec3( 0.25 / 255.0, -0.25 / 255.0, 0.25 / 255.0 );
		dither_shift_RGB = mix( 2.0 * dither_shift_RGB, -2.0 * dither_shift_RGB, grid_position );
		return color + dither_shift_RGB;
	}
#endif`,R0=`float roughnessFactor = roughness;
#ifdef USE_ROUGHNESSMAP
	vec4 texelRoughness = texture2D( roughnessMap, vRoughnessMapUv );
	roughnessFactor *= texelRoughness.g;
#endif`,C0=`#ifdef USE_ROUGHNESSMAP
	uniform sampler2D roughnessMap;
#endif`,P0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,I0=`#if NUM_SPOT_LIGHT_COORDS > 0
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
#endif`,L0=`#if ( defined( USE_SHADOWMAP ) && ( NUM_DIR_LIGHT_SHADOWS > 0 || NUM_POINT_LIGHT_SHADOWS > 0 ) ) || ( NUM_SPOT_LIGHT_COORDS > 0 )
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
#endif`,D0=`float getShadowMask() {
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
}`,N0=`#ifdef USE_SKINNING
	mat4 boneMatX = getBoneMatrix( skinIndex.x );
	mat4 boneMatY = getBoneMatrix( skinIndex.y );
	mat4 boneMatZ = getBoneMatrix( skinIndex.z );
	mat4 boneMatW = getBoneMatrix( skinIndex.w );
#endif`,U0=`#ifdef USE_SKINNING
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
#endif`,F0=`#ifdef USE_SKINNING
	vec4 skinVertex = bindMatrix * vec4( transformed, 1.0 );
	vec4 skinned = vec4( 0.0 );
	skinned += boneMatX * skinVertex * skinWeight.x;
	skinned += boneMatY * skinVertex * skinWeight.y;
	skinned += boneMatZ * skinVertex * skinWeight.z;
	skinned += boneMatW * skinVertex * skinWeight.w;
	transformed = ( bindMatrixInverse * skinned ).xyz;
#endif`,O0=`#ifdef USE_SKINNING
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
#endif`,B0=`float specularStrength;
#ifdef USE_SPECULARMAP
	vec4 texelSpecular = texture2D( specularMap, vSpecularMapUv );
	specularStrength = texelSpecular.r;
#else
	specularStrength = 1.0;
#endif`,z0=`#ifdef USE_SPECULARMAP
	uniform sampler2D specularMap;
#endif`,k0=`#if defined( TONE_MAPPING )
	gl_FragColor.rgb = toneMapping( gl_FragColor.rgb );
#endif`,H0=`#ifndef saturate
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
vec3 CustomToneMapping( vec3 color ) { return color; }`,V0=`#ifdef USE_TRANSMISSION
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
#endif`,G0=`#ifdef USE_TRANSMISSION
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
#endif`,W0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,X0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,q0=`#if defined( USE_UV ) || defined( USE_ANISOTROPY )
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
#endif`,Y0=`#if defined( USE_ENVMAP ) || defined( DISTANCE ) || defined ( USE_SHADOWMAP ) || defined ( USE_TRANSMISSION ) || NUM_SPOT_LIGHT_COORDS > 0
	vec4 worldPosition = vec4( transformed, 1.0 );
	#ifdef USE_BATCHING
		worldPosition = batchingMatrix * worldPosition;
	#endif
	#ifdef USE_INSTANCING
		worldPosition = instanceMatrix * worldPosition;
	#endif
	worldPosition = modelMatrix * worldPosition;
#endif`,Z0=`varying vec2 vUv;
uniform mat3 uvTransform;
void main() {
	vUv = ( uvTransform * vec3( uv, 1 ) ).xy;
	gl_Position = vec4( position.xy, 1.0, 1.0 );
}`,K0=`uniform sampler2D t2D;
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
}`,j0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,J0=`#ifdef ENVMAP_TYPE_CUBE
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
}`,$0=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
	gl_Position.z = gl_Position.w;
}`,Q0=`uniform samplerCube tCube;
uniform float tFlip;
uniform float opacity;
varying vec3 vWorldDirection;
void main() {
	vec4 texColor = textureCube( tCube, vec3( tFlip * vWorldDirection.x, vWorldDirection.yz ) );
	gl_FragColor = texColor;
	gl_FragColor.a *= opacity;
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,ex=`#include <common>
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
}`,tx=`#if DEPTH_PACKING == 3200
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
}`,nx=`#define DISTANCE
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
}`,ix=`#define DISTANCE
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
}`,sx=`varying vec3 vWorldDirection;
#include <common>
void main() {
	vWorldDirection = transformDirection( position, modelMatrix );
	#include <begin_vertex>
	#include <project_vertex>
}`,rx=`uniform sampler2D tEquirect;
varying vec3 vWorldDirection;
#include <common>
void main() {
	vec3 direction = normalize( vWorldDirection );
	vec2 sampleUV = equirectUv( direction );
	gl_FragColor = texture2D( tEquirect, sampleUV );
	#include <tonemapping_fragment>
	#include <colorspace_fragment>
}`,ox=`uniform float scale;
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
}`,ax=`uniform vec3 diffuse;
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
}`,lx=`#include <common>
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
}`,cx=`uniform vec3 diffuse;
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
}`,hx=`#define LAMBERT
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
}`,ux=`#define LAMBERT
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
}`,dx=`#define MATCAP
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
}`,fx=`#define MATCAP
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
}`,px=`#define NORMAL
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
}`,mx=`#define NORMAL
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
}`,gx=`#define PHONG
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
}`,xx=`#define PHONG
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
}`,_x=`#define STANDARD
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
}`,vx=`#define STANDARD
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
}`,yx=`#define TOON
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
}`,Mx=`#define TOON
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
}`,bx=`uniform float size;
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
}`,Sx=`uniform vec3 diffuse;
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
}`,wx=`#include <common>
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
}`,Ex=`uniform vec3 color;
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
}`,Tx=`uniform float rotation;
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
}`,Ax=`uniform vec3 diffuse;
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
}`,ht={alphahash_fragment:Zm,alphahash_pars_fragment:Km,alphamap_fragment:jm,alphamap_pars_fragment:Jm,alphatest_fragment:$m,alphatest_pars_fragment:Qm,aomap_fragment:eg,aomap_pars_fragment:tg,batching_pars_vertex:ng,batching_vertex:ig,begin_vertex:sg,beginnormal_vertex:rg,bsdfs:og,iridescence_fragment:ag,bumpmap_pars_fragment:lg,clipping_planes_fragment:cg,clipping_planes_pars_fragment:hg,clipping_planes_pars_vertex:ug,clipping_planes_vertex:dg,color_fragment:fg,color_pars_fragment:pg,color_pars_vertex:mg,color_vertex:gg,common:xg,cube_uv_reflection_fragment:_g,defaultnormal_vertex:vg,displacementmap_pars_vertex:yg,displacementmap_vertex:Mg,emissivemap_fragment:bg,emissivemap_pars_fragment:Sg,colorspace_fragment:wg,colorspace_pars_fragment:Eg,envmap_fragment:Tg,envmap_common_pars_fragment:Ag,envmap_pars_fragment:Rg,envmap_pars_vertex:Cg,envmap_physical_pars_fragment:kg,envmap_vertex:Pg,fog_vertex:Ig,fog_pars_vertex:Lg,fog_fragment:Dg,fog_pars_fragment:Ng,gradientmap_pars_fragment:Ug,lightmap_pars_fragment:Fg,lights_lambert_fragment:Og,lights_lambert_pars_fragment:Bg,lights_pars_begin:zg,lights_toon_fragment:Hg,lights_toon_pars_fragment:Vg,lights_phong_fragment:Gg,lights_phong_pars_fragment:Wg,lights_physical_fragment:Xg,lights_physical_pars_fragment:qg,lights_fragment_begin:Yg,lights_fragment_maps:Zg,lights_fragment_end:Kg,lightprobes_pars_fragment:jg,logdepthbuf_fragment:Jg,logdepthbuf_pars_fragment:$g,logdepthbuf_pars_vertex:Qg,logdepthbuf_vertex:e0,map_fragment:t0,map_pars_fragment:n0,map_particle_fragment:i0,map_particle_pars_fragment:s0,metalnessmap_fragment:r0,metalnessmap_pars_fragment:o0,morphinstance_vertex:a0,morphcolor_vertex:l0,morphnormal_vertex:c0,morphtarget_pars_vertex:h0,morphtarget_vertex:u0,normal_fragment_begin:d0,normal_fragment_maps:f0,normal_pars_fragment:p0,normal_pars_vertex:m0,normal_vertex:g0,normalmap_pars_fragment:x0,clearcoat_normal_fragment_begin:_0,clearcoat_normal_fragment_maps:v0,clearcoat_pars_fragment:y0,iridescence_pars_fragment:M0,opaque_fragment:b0,packing:S0,premultiplied_alpha_fragment:w0,project_vertex:E0,dithering_fragment:T0,dithering_pars_fragment:A0,roughnessmap_fragment:R0,roughnessmap_pars_fragment:C0,shadowmap_pars_fragment:P0,shadowmap_pars_vertex:I0,shadowmap_vertex:L0,shadowmask_pars_fragment:D0,skinbase_vertex:N0,skinning_pars_vertex:U0,skinning_vertex:F0,skinnormal_vertex:O0,specularmap_fragment:B0,specularmap_pars_fragment:z0,tonemapping_fragment:k0,tonemapping_pars_fragment:H0,transmission_fragment:V0,transmission_pars_fragment:G0,uv_pars_fragment:W0,uv_pars_vertex:X0,uv_vertex:q0,worldpos_vertex:Y0,background_vert:Z0,background_frag:K0,backgroundCube_vert:j0,backgroundCube_frag:J0,cube_vert:$0,cube_frag:Q0,depth_vert:ex,depth_frag:tx,distance_vert:nx,distance_frag:ix,equirect_vert:sx,equirect_frag:rx,linedashed_vert:ox,linedashed_frag:ax,meshbasic_vert:lx,meshbasic_frag:cx,meshlambert_vert:hx,meshlambert_frag:ux,meshmatcap_vert:dx,meshmatcap_frag:fx,meshnormal_vert:px,meshnormal_frag:mx,meshphong_vert:gx,meshphong_frag:xx,meshphysical_vert:_x,meshphysical_frag:vx,meshtoon_vert:yx,meshtoon_frag:Mx,points_vert:bx,points_frag:Sx,shadow_vert:wx,shadow_frag:Ex,sprite_vert:Tx,sprite_frag:Ax},Ie={common:{diffuse:{value:new xe(16777215)},opacity:{value:1},map:{value:null},mapTransform:{value:new et},alphaMap:{value:null},alphaMapTransform:{value:new et},alphaTest:{value:0}},specularmap:{specularMap:{value:null},specularMapTransform:{value:new et}},envmap:{envMap:{value:null},envMapRotation:{value:new et},reflectivity:{value:1},ior:{value:1.5},refractionRatio:{value:.98},dfgLUT:{value:null}},aomap:{aoMap:{value:null},aoMapIntensity:{value:1},aoMapTransform:{value:new et}},lightmap:{lightMap:{value:null},lightMapIntensity:{value:1},lightMapTransform:{value:new et}},bumpmap:{bumpMap:{value:null},bumpMapTransform:{value:new et},bumpScale:{value:1}},normalmap:{normalMap:{value:null},normalMapTransform:{value:new et},normalScale:{value:new _e(1,1)}},displacementmap:{displacementMap:{value:null},displacementMapTransform:{value:new et},displacementScale:{value:1},displacementBias:{value:0}},emissivemap:{emissiveMap:{value:null},emissiveMapTransform:{value:new et}},metalnessmap:{metalnessMap:{value:null},metalnessMapTransform:{value:new et}},roughnessmap:{roughnessMap:{value:null},roughnessMapTransform:{value:new et}},gradientmap:{gradientMap:{value:null}},fog:{fogDensity:{value:25e-5},fogNear:{value:1},fogFar:{value:2e3},fogColor:{value:new xe(16777215)}},lights:{ambientLightColor:{value:[]},lightProbe:{value:[]},directionalLights:{value:[],properties:{direction:{},color:{}}},directionalLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},directionalShadowMatrix:{value:[]},spotLights:{value:[],properties:{color:{},position:{},direction:{},distance:{},coneCos:{},penumbraCos:{},decay:{}}},spotLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{}}},spotLightMap:{value:[]},spotLightMatrix:{value:[]},pointLights:{value:[],properties:{color:{},position:{},decay:{},distance:{}}},pointLightShadows:{value:[],properties:{shadowIntensity:1,shadowBias:{},shadowNormalBias:{},shadowRadius:{},shadowMapSize:{},shadowCameraNear:{},shadowCameraFar:{}}},pointShadowMatrix:{value:[]},hemisphereLights:{value:[],properties:{direction:{},skyColor:{},groundColor:{}}},rectAreaLights:{value:[],properties:{color:{},position:{},width:{},height:{}}},ltc_1:{value:null},ltc_2:{value:null},probesSH:{value:null},probesMin:{value:new B},probesMax:{value:new B},probesResolution:{value:new B}},points:{diffuse:{value:new xe(16777215)},opacity:{value:1},size:{value:1},scale:{value:1},map:{value:null},alphaMap:{value:null},alphaMapTransform:{value:new et},alphaTest:{value:0},uvTransform:{value:new et}},sprite:{diffuse:{value:new xe(16777215)},opacity:{value:1},center:{value:new _e(.5,.5)},rotation:{value:0},map:{value:null},mapTransform:{value:new et},alphaMap:{value:null},alphaMapTransform:{value:new et},alphaTest:{value:0}}},Mi={basic:{uniforms:fn([Ie.common,Ie.specularmap,Ie.envmap,Ie.aomap,Ie.lightmap,Ie.fog]),vertexShader:ht.meshbasic_vert,fragmentShader:ht.meshbasic_frag},lambert:{uniforms:fn([Ie.common,Ie.specularmap,Ie.envmap,Ie.aomap,Ie.lightmap,Ie.emissivemap,Ie.bumpmap,Ie.normalmap,Ie.displacementmap,Ie.fog,Ie.lights,{emissive:{value:new xe(0)},envMapIntensity:{value:1}}]),vertexShader:ht.meshlambert_vert,fragmentShader:ht.meshlambert_frag},phong:{uniforms:fn([Ie.common,Ie.specularmap,Ie.envmap,Ie.aomap,Ie.lightmap,Ie.emissivemap,Ie.bumpmap,Ie.normalmap,Ie.displacementmap,Ie.fog,Ie.lights,{emissive:{value:new xe(0)},specular:{value:new xe(1118481)},shininess:{value:30},envMapIntensity:{value:1}}]),vertexShader:ht.meshphong_vert,fragmentShader:ht.meshphong_frag},standard:{uniforms:fn([Ie.common,Ie.envmap,Ie.aomap,Ie.lightmap,Ie.emissivemap,Ie.bumpmap,Ie.normalmap,Ie.displacementmap,Ie.roughnessmap,Ie.metalnessmap,Ie.fog,Ie.lights,{emissive:{value:new xe(0)},roughness:{value:1},metalness:{value:0},envMapIntensity:{value:1}}]),vertexShader:ht.meshphysical_vert,fragmentShader:ht.meshphysical_frag},toon:{uniforms:fn([Ie.common,Ie.aomap,Ie.lightmap,Ie.emissivemap,Ie.bumpmap,Ie.normalmap,Ie.displacementmap,Ie.gradientmap,Ie.fog,Ie.lights,{emissive:{value:new xe(0)}}]),vertexShader:ht.meshtoon_vert,fragmentShader:ht.meshtoon_frag},matcap:{uniforms:fn([Ie.common,Ie.bumpmap,Ie.normalmap,Ie.displacementmap,Ie.fog,{matcap:{value:null}}]),vertexShader:ht.meshmatcap_vert,fragmentShader:ht.meshmatcap_frag},points:{uniforms:fn([Ie.points,Ie.fog]),vertexShader:ht.points_vert,fragmentShader:ht.points_frag},dashed:{uniforms:fn([Ie.common,Ie.fog,{scale:{value:1},dashSize:{value:1},totalSize:{value:2}}]),vertexShader:ht.linedashed_vert,fragmentShader:ht.linedashed_frag},depth:{uniforms:fn([Ie.common,Ie.displacementmap]),vertexShader:ht.depth_vert,fragmentShader:ht.depth_frag},normal:{uniforms:fn([Ie.common,Ie.bumpmap,Ie.normalmap,Ie.displacementmap,{opacity:{value:1}}]),vertexShader:ht.meshnormal_vert,fragmentShader:ht.meshnormal_frag},sprite:{uniforms:fn([Ie.sprite,Ie.fog]),vertexShader:ht.sprite_vert,fragmentShader:ht.sprite_frag},background:{uniforms:{uvTransform:{value:new et},t2D:{value:null},backgroundIntensity:{value:1}},vertexShader:ht.background_vert,fragmentShader:ht.background_frag},backgroundCube:{uniforms:{envMap:{value:null},backgroundBlurriness:{value:0},backgroundIntensity:{value:1},backgroundRotation:{value:new et}},vertexShader:ht.backgroundCube_vert,fragmentShader:ht.backgroundCube_frag},cube:{uniforms:{tCube:{value:null},tFlip:{value:-1},opacity:{value:1}},vertexShader:ht.cube_vert,fragmentShader:ht.cube_frag},equirect:{uniforms:{tEquirect:{value:null}},vertexShader:ht.equirect_vert,fragmentShader:ht.equirect_frag},distance:{uniforms:fn([Ie.common,Ie.displacementmap,{referencePosition:{value:new B},nearDistance:{value:1},farDistance:{value:1e3}}]),vertexShader:ht.distance_vert,fragmentShader:ht.distance_frag},shadow:{uniforms:fn([Ie.lights,Ie.fog,{color:{value:new xe(0)},opacity:{value:1}}]),vertexShader:ht.shadow_vert,fragmentShader:ht.shadow_frag}};Mi.physical={uniforms:fn([Mi.standard.uniforms,{clearcoat:{value:0},clearcoatMap:{value:null},clearcoatMapTransform:{value:new et},clearcoatNormalMap:{value:null},clearcoatNormalMapTransform:{value:new et},clearcoatNormalScale:{value:new _e(1,1)},clearcoatRoughness:{value:0},clearcoatRoughnessMap:{value:null},clearcoatRoughnessMapTransform:{value:new et},dispersion:{value:0},iridescence:{value:0},iridescenceMap:{value:null},iridescenceMapTransform:{value:new et},iridescenceIOR:{value:1.3},iridescenceThicknessMinimum:{value:100},iridescenceThicknessMaximum:{value:400},iridescenceThicknessMap:{value:null},iridescenceThicknessMapTransform:{value:new et},sheen:{value:0},sheenColor:{value:new xe(0)},sheenColorMap:{value:null},sheenColorMapTransform:{value:new et},sheenRoughness:{value:1},sheenRoughnessMap:{value:null},sheenRoughnessMapTransform:{value:new et},transmission:{value:0},transmissionMap:{value:null},transmissionMapTransform:{value:new et},transmissionSamplerSize:{value:new _e},transmissionSamplerMap:{value:null},thickness:{value:0},thicknessMap:{value:null},thicknessMapTransform:{value:new et},attenuationDistance:{value:0},attenuationColor:{value:new xe(0)},specularColor:{value:new xe(1,1,1)},specularColorMap:{value:null},specularColorMapTransform:{value:new et},specularIntensity:{value:1},specularIntensityMap:{value:null},specularIntensityMapTransform:{value:new et},anisotropyVector:{value:new _e},anisotropyMap:{value:null},anisotropyMapTransform:{value:new et}}]),vertexShader:ht.meshphysical_vert,fragmentShader:ht.meshphysical_frag};var tc={r:0,b:0,g:0},Rx=new Qe,Nf=new et;Nf.set(-1,0,0,0,1,0,0,0,1);function Cx(s,e,t,n,i,r){let o=new xe(0),a=i===!0?0:1,l,c,h=null,u=0,d=null;function f(b){let S=b.isScene===!0?b.background:null;if(S&&S.isTexture){let x=b.backgroundBlurriness>0;S=e.get(S,x)}return S}function g(b){let S=!1,x=f(b);x===null?m(o,a):x&&x.isColor&&(m(x,1),S=!0);let A=s.xr.getEnvironmentBlendMode();A==="additive"?t.buffers.color.setClear(0,0,0,1,r):A==="alpha-blend"&&t.buffers.color.setClear(0,0,0,0,r),(s.autoClear||S)&&(t.buffers.depth.setTest(!0),t.buffers.depth.setMask(!0),t.buffers.color.setMask(!0),s.clear(s.autoClearColor,s.autoClearDepth,s.autoClearStencil))}function y(b,S){let x=f(S);x&&(x.isCubeTexture||x.mapping===Fo)?(c===void 0&&(c=new Ke(new mi(1,1,1),new mt({name:"BackgroundCubeMaterial",uniforms:Bs(Mi.backgroundCube.uniforms),vertexShader:Mi.backgroundCube.vertexShader,fragmentShader:Mi.backgroundCube.fragmentShader,side:tn,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),c.geometry.deleteAttribute("normal"),c.geometry.deleteAttribute("uv"),c.onBeforeRender=function(A,T,I){this.matrixWorld.copyPosition(I.matrixWorld)},Object.defineProperty(c.material,"envMap",{get:function(){return this.uniforms.envMap.value}}),n.update(c)),c.material.uniforms.envMap.value=x,c.material.uniforms.backgroundBlurriness.value=S.backgroundBlurriness,c.material.uniforms.backgroundIntensity.value=S.backgroundIntensity,c.material.uniforms.backgroundRotation.value.setFromMatrix4(Rx.makeRotationFromEuler(S.backgroundRotation)).transpose(),x.isCubeTexture&&x.isRenderTargetTexture===!1&&c.material.uniforms.backgroundRotation.value.premultiply(Nf),c.material.toneMapped=ot.getTransfer(x.colorSpace)!==Mt,(h!==x||u!==x.version||d!==s.toneMapping)&&(c.material.needsUpdate=!0,h=x,u=x.version,d=s.toneMapping),c.layers.enableAll(),b.unshift(c,c.geometry,c.material,0,0,null)):x&&x.isTexture&&(l===void 0&&(l=new Ke(new ln(2,2),new mt({name:"BackgroundMaterial",uniforms:Bs(Mi.background.uniforms),vertexShader:Mi.background.vertexShader,fragmentShader:Mi.background.fragmentShader,side:Ln,depthTest:!1,depthWrite:!1,fog:!1,allowOverride:!1})),l.geometry.deleteAttribute("normal"),Object.defineProperty(l.material,"map",{get:function(){return this.uniforms.t2D.value}}),n.update(l)),l.material.uniforms.t2D.value=x,l.material.uniforms.backgroundIntensity.value=S.backgroundIntensity,l.material.toneMapped=ot.getTransfer(x.colorSpace)!==Mt,x.matrixAutoUpdate===!0&&x.updateMatrix(),l.material.uniforms.uvTransform.value.copy(x.matrix),(h!==x||u!==x.version||d!==s.toneMapping)&&(l.material.needsUpdate=!0,h=x,u=x.version,d=s.toneMapping),l.layers.enableAll(),b.unshift(l,l.geometry,l.material,0,0,null))}function m(b,S){b.getRGB(tc,Th(s)),t.buffers.color.setClear(tc.r,tc.g,tc.b,S,r)}function p(){c!==void 0&&(c.geometry.dispose(),c.material.dispose(),c=void 0),l!==void 0&&(l.geometry.dispose(),l.material.dispose(),l=void 0)}return{getClearColor:function(){return o},setClearColor:function(b,S=1){o.set(b),a=S,m(o,a)},getClearAlpha:function(){return a},setClearAlpha:function(b){a=b,m(o,a)},render:g,addToRenderList:y,dispose:p}}function Px(s,e){let t=s.getParameter(s.MAX_VERTEX_ATTRIBS),n={},i=d(null),r=i,o=!1;function a(R,L,V,C,N){let U=!1,E=u(R,C,V,L);r!==E&&(r=E,c(r.object)),U=f(R,C,V,N),U&&g(R,C,V,N),N!==null&&e.update(N,s.ELEMENT_ARRAY_BUFFER),(U||o)&&(o=!1,x(R,L,V,C),N!==null&&s.bindBuffer(s.ELEMENT_ARRAY_BUFFER,e.get(N).buffer))}function l(){return s.createVertexArray()}function c(R){return s.bindVertexArray(R)}function h(R){return s.deleteVertexArray(R)}function u(R,L,V,C){let N=C.wireframe===!0,U=n[L.id];U===void 0&&(U={},n[L.id]=U);let E=R.isInstancedMesh===!0?R.id:0,G=U[E];G===void 0&&(G={},U[E]=G);let Y=G[V.id];Y===void 0&&(Y={},G[V.id]=Y);let X=Y[N];return X===void 0&&(X=d(l()),Y[N]=X),X}function d(R){let L=[],V=[],C=[];for(let N=0;N<t;N++)L[N]=0,V[N]=0,C[N]=0;return{geometry:null,program:null,wireframe:!1,newAttributes:L,enabledAttributes:V,attributeDivisors:C,object:R,attributes:{},index:null}}function f(R,L,V,C){let N=r.attributes,U=L.attributes,E=0,G=V.getAttributes();for(let Y in G)if(G[Y].location>=0){let ie=N[Y],ce=U[Y];if(ce===void 0&&(Y==="instanceMatrix"&&R.instanceMatrix&&(ce=R.instanceMatrix),Y==="instanceColor"&&R.instanceColor&&(ce=R.instanceColor)),ie===void 0||ie.attribute!==ce||ce&&ie.data!==ce.data)return!0;E++}return r.attributesNum!==E||r.index!==C}function g(R,L,V,C){let N={},U=L.attributes,E=0,G=V.getAttributes();for(let Y in G)if(G[Y].location>=0){let ie=U[Y];ie===void 0&&(Y==="instanceMatrix"&&R.instanceMatrix&&(ie=R.instanceMatrix),Y==="instanceColor"&&R.instanceColor&&(ie=R.instanceColor));let ce={};ce.attribute=ie,ie&&ie.data&&(ce.data=ie.data),N[Y]=ce,E++}r.attributes=N,r.attributesNum=E,r.index=C}function y(){let R=r.newAttributes;for(let L=0,V=R.length;L<V;L++)R[L]=0}function m(R){p(R,0)}function p(R,L){let V=r.newAttributes,C=r.enabledAttributes,N=r.attributeDivisors;V[R]=1,C[R]===0&&(s.enableVertexAttribArray(R),C[R]=1),N[R]!==L&&(s.vertexAttribDivisor(R,L),N[R]=L)}function b(){let R=r.newAttributes,L=r.enabledAttributes;for(let V=0,C=L.length;V<C;V++)L[V]!==R[V]&&(s.disableVertexAttribArray(V),L[V]=0)}function S(R,L,V,C,N,U,E){E===!0?s.vertexAttribIPointer(R,L,V,N,U):s.vertexAttribPointer(R,L,V,C,N,U)}function x(R,L,V,C){y();let N=C.attributes,U=V.getAttributes(),E=L.defaultAttributeValues;for(let G in U){let Y=U[G];if(Y.location>=0){let X=N[G];if(X===void 0&&(G==="instanceMatrix"&&R.instanceMatrix&&(X=R.instanceMatrix),G==="instanceColor"&&R.instanceColor&&(X=R.instanceColor)),X!==void 0){let ie=X.normalized,ce=X.itemSize,be=e.get(X);if(be===void 0)continue;let Le=be.buffer,Re=be.type,oe=be.bytesPerElement,ge=Re===s.INT||Re===s.UNSIGNED_INT||X.gpuType===pl;if(X.isInterleavedBufferAttribute){let ae=X.data,Se=ae.stride,he=X.offset;if(ae.isInstancedInterleavedBuffer){for(let ue=0;ue<Y.locationSize;ue++)p(Y.location+ue,ae.meshPerAttribute);R.isInstancedMesh!==!0&&C._maxInstanceCount===void 0&&(C._maxInstanceCount=ae.meshPerAttribute*ae.count)}else for(let ue=0;ue<Y.locationSize;ue++)m(Y.location+ue);s.bindBuffer(s.ARRAY_BUFFER,Le);for(let ue=0;ue<Y.locationSize;ue++)S(Y.location+ue,ce/Y.locationSize,Re,ie,Se*oe,(he+ce/Y.locationSize*ue)*oe,ge)}else{if(X.isInstancedBufferAttribute){for(let ae=0;ae<Y.locationSize;ae++)p(Y.location+ae,X.meshPerAttribute);R.isInstancedMesh!==!0&&C._maxInstanceCount===void 0&&(C._maxInstanceCount=X.meshPerAttribute*X.count)}else for(let ae=0;ae<Y.locationSize;ae++)m(Y.location+ae);s.bindBuffer(s.ARRAY_BUFFER,Le);for(let ae=0;ae<Y.locationSize;ae++)S(Y.location+ae,ce/Y.locationSize,Re,ie,ce*oe,ce/Y.locationSize*ae*oe,ge)}}else if(E!==void 0){let ie=E[G];if(ie!==void 0)switch(ie.length){case 2:s.vertexAttrib2fv(Y.location,ie);break;case 3:s.vertexAttrib3fv(Y.location,ie);break;case 4:s.vertexAttrib4fv(Y.location,ie);break;default:s.vertexAttrib1fv(Y.location,ie)}}}}b()}function A(){D();for(let R in n){let L=n[R];for(let V in L){let C=L[V];for(let N in C){let U=C[N];for(let E in U)h(U[E].object),delete U[E];delete C[N]}}delete n[R]}}function T(R){if(n[R.id]===void 0)return;let L=n[R.id];for(let V in L){let C=L[V];for(let N in C){let U=C[N];for(let E in U)h(U[E].object),delete U[E];delete C[N]}}delete n[R.id]}function I(R){for(let L in n){let V=n[L];for(let C in V){let N=V[C];if(N[R.id]===void 0)continue;let U=N[R.id];for(let E in U)h(U[E].object),delete U[E];delete N[R.id]}}}function v(R){for(let L in n){let V=n[L],C=R.isInstancedMesh===!0?R.id:0,N=V[C];if(N!==void 0){for(let U in N){let E=N[U];for(let G in E)h(E[G].object),delete E[G];delete N[U]}delete V[C],Object.keys(V).length===0&&delete n[L]}}}function D(){w(),o=!0,r!==i&&(r=i,c(r.object))}function w(){i.geometry=null,i.program=null,i.wireframe=!1}return{setup:a,reset:D,resetDefaultState:w,dispose:A,releaseStatesOfGeometry:T,releaseStatesOfObject:v,releaseStatesOfProgram:I,initAttributes:y,enableAttribute:m,disableUnusedAttributes:b}}function Ix(s,e,t){let n;function i(l){n=l}function r(l,c){s.drawArrays(n,l,c),t.update(c,n,1)}function o(l,c,h){h!==0&&(s.drawArraysInstanced(n,l,c,h),t.update(c,n,h))}function a(l,c,h){if(h===0)return;e.get("WEBGL_multi_draw").multiDrawArraysWEBGL(n,l,0,c,0,h);let d=0;for(let f=0;f<h;f++)d+=c[f];t.update(d,n,1)}this.setMode=i,this.render=r,this.renderInstances=o,this.renderMultiDraw=a}function Lx(s,e,t,n){let i;function r(){if(i!==void 0)return i;if(e.has("EXT_texture_filter_anisotropic")===!0){let I=e.get("EXT_texture_filter_anisotropic");i=s.getParameter(I.MAX_TEXTURE_MAX_ANISOTROPY_EXT)}else i=0;return i}function o(I){return!(I!==Fn&&n.convert(I)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_FORMAT))}function a(I){let v=I===nn&&(e.has("EXT_color_buffer_half_float")||e.has("EXT_color_buffer_float"));return!(I!==Sn&&n.convert(I)!==s.getParameter(s.IMPLEMENTATION_COLOR_READ_TYPE)&&I!==Un&&!v)}function l(I){if(I==="highp"){if(s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.HIGH_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.HIGH_FLOAT).precision>0)return"highp";I="mediump"}return I==="mediump"&&s.getShaderPrecisionFormat(s.VERTEX_SHADER,s.MEDIUM_FLOAT).precision>0&&s.getShaderPrecisionFormat(s.FRAGMENT_SHADER,s.MEDIUM_FLOAT).precision>0?"mediump":"lowp"}let c=t.precision!==void 0?t.precision:"highp",h=l(c);h!==c&&(We("WebGLRenderer:",c,"not supported, using",h,"instead."),c=h);let u=t.logarithmicDepthBuffer===!0,d=t.reversedDepthBuffer===!0&&e.has("EXT_clip_control");t.reversedDepthBuffer===!0&&d===!1&&We("WebGLRenderer: Unable to use reversed depth buffer due to missing EXT_clip_control extension. Fallback to default depth buffer.");let f=s.getParameter(s.MAX_TEXTURE_IMAGE_UNITS),g=s.getParameter(s.MAX_VERTEX_TEXTURE_IMAGE_UNITS),y=s.getParameter(s.MAX_TEXTURE_SIZE),m=s.getParameter(s.MAX_CUBE_MAP_TEXTURE_SIZE),p=s.getParameter(s.MAX_VERTEX_ATTRIBS),b=s.getParameter(s.MAX_VERTEX_UNIFORM_VECTORS),S=s.getParameter(s.MAX_VARYING_VECTORS),x=s.getParameter(s.MAX_FRAGMENT_UNIFORM_VECTORS),A=s.getParameter(s.MAX_SAMPLES),T=s.getParameter(s.SAMPLES);return{isWebGL2:!0,getMaxAnisotropy:r,getMaxPrecision:l,textureFormatReadable:o,textureTypeReadable:a,precision:c,logarithmicDepthBuffer:u,reversedDepthBuffer:d,maxTextures:f,maxVertexTextures:g,maxTextureSize:y,maxCubemapSize:m,maxAttributes:p,maxVertexUniforms:b,maxVaryings:S,maxFragmentUniforms:x,maxSamples:A,samples:T}}function Dx(s){let e=this,t=null,n=0,i=!1,r=!1,o=new kn,a=new et,l={value:null,needsUpdate:!1};this.uniform=l,this.numPlanes=0,this.numIntersection=0,this.init=function(u,d){let f=u.length!==0||d||n!==0||i;return i=d,n=u.length,f},this.beginShadows=function(){r=!0,h(null)},this.endShadows=function(){r=!1},this.setGlobalState=function(u,d){t=h(u,d,0)},this.setState=function(u,d,f){let g=u.clippingPlanes,y=u.clipIntersection,m=u.clipShadows,p=s.get(u);if(!i||g===null||g.length===0||r&&!m)r?h(null):c();else{let b=r?0:n,S=b*4,x=p.clippingState||null;l.value=x,x=h(g,d,S,f);for(let A=0;A!==S;++A)x[A]=t[A];p.clippingState=x,this.numIntersection=y?this.numPlanes:0,this.numPlanes+=b}};function c(){l.value!==t&&(l.value=t,l.needsUpdate=n>0),e.numPlanes=n,e.numIntersection=0}function h(u,d,f,g){let y=u!==null?u.length:0,m=null;if(y!==0){if(m=l.value,g!==!0||m===null){let p=f+y*4,b=d.matrixWorldInverse;a.getNormalMatrix(b),(m===null||m.length<p)&&(m=new Float32Array(p));for(let S=0,x=f;S!==y;++S,x+=4)o.copy(u[S]).applyMatrix4(b,a),o.normal.toArray(m,x),m[x+3]=o.constant}l.value=m,l.needsUpdate=!0}return e.numPlanes=y,e.numIntersection=0,m}}var hs=4,uf=[.125,.215,.35,.446,.526,.582],zs=20,Nx=256,Xo=new vi,df=new xe,Ph=null,Ih=0,Lh=0,Dh=!1,Ux=new B,Dr=class{constructor(e){this._renderer=e,this._pingPongRenderTarget=null,this._lodMax=0,this._cubeSize=0,this._sizeLods=[],this._sigmas=[],this._lodMeshes=[],this._backgroundBox=null,this._cubemapMaterial=null,this._equirectMaterial=null,this._blurMaterial=null,this._ggxMaterial=null}fromScene(e,t=0,n=.1,i=100,r={}){let{size:o=256,position:a=Ux}=r;Ph=this._renderer.getRenderTarget(),Ih=this._renderer.getActiveCubeFace(),Lh=this._renderer.getActiveMipmapLevel(),Dh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1,this._setSize(o);let l=this._allocateTargets();return l.depthBuffer=!0,this._sceneToCubeUV(e,n,i,l,a),t>0&&this._blur(l,0,0,t),this._applyPMREM(l),this._cleanup(l),l}fromEquirectangular(e,t=null){return this._fromTexture(e,t)}fromCubemap(e,t=null){return this._fromTexture(e,t)}compileCubemapShader(){this._cubemapMaterial===null&&(this._cubemapMaterial=mf(),this._compileMaterial(this._cubemapMaterial))}compileEquirectangularShader(){this._equirectMaterial===null&&(this._equirectMaterial=pf(),this._compileMaterial(this._equirectMaterial))}dispose(){this._dispose(),this._cubemapMaterial!==null&&this._cubemapMaterial.dispose(),this._equirectMaterial!==null&&this._equirectMaterial.dispose(),this._backgroundBox!==null&&(this._backgroundBox.geometry.dispose(),this._backgroundBox.material.dispose())}_setSize(e){this._lodMax=Math.floor(Math.log2(e)),this._cubeSize=Math.pow(2,this._lodMax)}_dispose(){this._blurMaterial!==null&&this._blurMaterial.dispose(),this._ggxMaterial!==null&&this._ggxMaterial.dispose(),this._pingPongRenderTarget!==null&&this._pingPongRenderTarget.dispose();for(let e=0;e<this._lodMeshes.length;e++)this._lodMeshes[e].geometry.dispose()}_cleanup(e){this._renderer.setRenderTarget(Ph,Ih,Lh),this._renderer.xr.enabled=Dh,e.scissorTest=!1,Ir(e,0,0,e.width,e.height)}_fromTexture(e,t){e.mapping===as||e.mapping===Fs?this._setSize(e.image.length===0?16:e.image[0].width||e.image[0].image.width):this._setSize(e.image.width/4),Ph=this._renderer.getRenderTarget(),Ih=this._renderer.getActiveCubeFace(),Lh=this._renderer.getActiveMipmapLevel(),Dh=this._renderer.xr.enabled,this._renderer.xr.enabled=!1;let n=t||this._allocateTargets();return this._textureToCubeUV(e,n),this._applyPMREM(n),this._cleanup(n),n}_allocateTargets(){let e=3*Math.max(this._cubeSize,112),t=4*this._cubeSize,n={magFilter:kt,minFilter:kt,generateMipmaps:!1,type:nn,format:Fn,colorSpace:gn,depthBuffer:!1},i=ff(e,t,n);if(this._pingPongRenderTarget===null||this._pingPongRenderTarget.width!==e||this._pingPongRenderTarget.height!==t){this._pingPongRenderTarget!==null&&this._dispose(),this._pingPongRenderTarget=ff(e,t,n);let{_lodMax:r}=this;({lodMeshes:this._lodMeshes,sizeLods:this._sizeLods,sigmas:this._sigmas}=Fx(r)),this._blurMaterial=Bx(r,e,t),this._ggxMaterial=Ox(r,e,t)}return i}_compileMaterial(e){let t=new Ke(new pt,e);this._renderer.compile(t,Xo)}_sceneToCubeUV(e,t,n,i,r){let l=new Qt(90,1,t,n),c=[1,-1,1,1,1,1],h=[1,1,1,-1,-1,-1],u=this._renderer,d=u.autoClear,f=u.toneMapping;u.getClearColor(df),u.toneMapping=ii,u.autoClear=!1,u.state.buffers.depth.getReversed()&&(u.setRenderTarget(i),u.clearDepth(),u.setRenderTarget(null)),this._backgroundBox===null&&(this._backgroundBox=new Ke(new mi,new bt({name:"PMREM.Background",side:tn,depthWrite:!1,depthTest:!1})));let y=this._backgroundBox,m=y.material,p=!1,b=e.background;b?b.isColor&&(m.color.copy(b),e.background=null,p=!0):(m.color.copy(df),p=!0);for(let S=0;S<6;S++){let x=S%3;x===0?(l.up.set(0,c[S],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x+h[S],r.y,r.z)):x===1?(l.up.set(0,0,c[S]),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y+h[S],r.z)):(l.up.set(0,c[S],0),l.position.set(r.x,r.y,r.z),l.lookAt(r.x,r.y,r.z+h[S]));let A=this._cubeSize;Ir(i,x*A,S>2?A:0,A,A),u.setRenderTarget(i),p&&u.render(y,l),u.render(e,l)}u.toneMapping=f,u.autoClear=d,e.background=b}_textureToCubeUV(e,t){let n=this._renderer,i=e.mapping===as||e.mapping===Fs;i?(this._cubemapMaterial===null&&(this._cubemapMaterial=mf()),this._cubemapMaterial.uniforms.flipEnvMap.value=e.isRenderTargetTexture===!1?-1:1):this._equirectMaterial===null&&(this._equirectMaterial=pf());let r=i?this._cubemapMaterial:this._equirectMaterial,o=this._lodMeshes[0];o.material=r;let a=r.uniforms;a.envMap.value=e;let l=this._cubeSize;Ir(t,0,0,3*l,2*l),n.setRenderTarget(t),n.render(o,Xo)}_applyPMREM(e){let t=this._renderer,n=t.autoClear;t.autoClear=!1;let i=this._lodMeshes.length;for(let r=1;r<i;r++)this._applyGGXFilter(e,r-1,r);t.autoClear=n}_applyGGXFilter(e,t,n){let i=this._renderer,r=this._pingPongRenderTarget,o=this._ggxMaterial,a=this._lodMeshes[n];a.material=o;let l=o.uniforms,c=n/(this._lodMeshes.length-1),h=t/(this._lodMeshes.length-1),u=Math.sqrt(c*c-h*h),d=0+c*1.25,f=u*d,{_lodMax:g}=this,y=this._sizeLods[n],m=3*y*(n>g-hs?n-g+hs:0),p=4*(this._cubeSize-y);l.envMap.value=e.texture,l.roughness.value=f,l.mipInt.value=g-t,Ir(r,m,p,3*y,2*y),i.setRenderTarget(r),i.render(a,Xo),l.envMap.value=r.texture,l.roughness.value=0,l.mipInt.value=g-n,Ir(e,m,p,3*y,2*y),i.setRenderTarget(e),i.render(a,Xo)}_blur(e,t,n,i,r){let o=this._pingPongRenderTarget;this._halfBlur(e,o,t,n,i,"latitudinal",r),this._halfBlur(o,e,n,n,i,"longitudinal",r)}_halfBlur(e,t,n,i,r,o,a){let l=this._renderer,c=this._blurMaterial;o!=="latitudinal"&&o!=="longitudinal"&&$e("blur direction must be either latitudinal or longitudinal!");let h=3,u=this._lodMeshes[i];u.material=c;let d=c.uniforms,f=this._sizeLods[n]-1,g=isFinite(r)?Math.PI/(2*f):2*Math.PI/(2*zs-1),y=r/g,m=isFinite(r)?1+Math.floor(h*y):zs;m>zs&&We(`sigmaRadians, ${r}, is too large and will clip, as it requested ${m} samples when the maximum is set to ${zs}`);let p=[],b=0;for(let I=0;I<zs;++I){let v=I/y,D=Math.exp(-v*v/2);p.push(D),I===0?b+=D:I<m&&(b+=2*D)}for(let I=0;I<p.length;I++)p[I]=p[I]/b;d.envMap.value=e.texture,d.samples.value=m,d.weights.value=p,d.latitudinal.value=o==="latitudinal",a&&(d.poleAxis.value=a);let{_lodMax:S}=this;d.dTheta.value=g,d.mipInt.value=S-n;let x=this._sizeLods[i],A=3*x*(i>S-hs?i-S+hs:0),T=4*(this._cubeSize-x);Ir(t,A,T,3*x,2*x),l.setRenderTarget(t),l.render(u,Xo)}};function Fx(s){let e=[],t=[],n=[],i=s,r=s-hs+1+uf.length;for(let o=0;o<r;o++){let a=Math.pow(2,i);e.push(a);let l=1/a;o>s-hs?l=uf[o-s+hs-1]:o===0&&(l=0),t.push(l);let c=1/(a-2),h=-c,u=1+c,d=[h,h,u,h,u,u,h,h,u,u,h,u],f=6,g=6,y=3,m=2,p=1,b=new Float32Array(y*g*f),S=new Float32Array(m*g*f),x=new Float32Array(p*g*f);for(let T=0;T<f;T++){let I=T%3*2/3-1,v=T>2?0:-1,D=[I,v,0,I+2/3,v,0,I+2/3,v+1,0,I,v,0,I+2/3,v+1,0,I,v+1,0];b.set(D,y*g*T),S.set(d,m*g*T);let w=[T,T,T,T,T,T];x.set(w,p*g*T)}let A=new pt;A.setAttribute("position",new xt(b,y)),A.setAttribute("uv",new xt(S,m)),A.setAttribute("faceIndex",new xt(x,p)),n.push(new Ke(A,null)),i>hs&&i--}return{lodMeshes:n,sizeLods:e,sigmas:t}}function ff(s,e,t){let n=new Wt(s,e,t);return n.texture.mapping=Fo,n.texture.name="PMREM.cubeUv",n.scissorTest=!0,n}function Ir(s,e,t,n,i){s.viewport.set(e,t,n,i),s.scissor.set(e,t,n,i)}function Ox(s,e,t){return new mt({name:"PMREMGGXConvolution",defines:{GGX_SAMPLES:Nx,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},roughness:{value:0},mipInt:{value:0}},vertexShader:rc(),fragmentShader:`

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
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function Bx(s,e,t){let n=new Float32Array(zs),i=new B(0,1,0);return new mt({name:"SphericalGaussianBlur",defines:{n:zs,CUBEUV_TEXEL_WIDTH:1/e,CUBEUV_TEXEL_HEIGHT:1/t,CUBEUV_MAX_MIP:`${s}.0`},uniforms:{envMap:{value:null},samples:{value:1},weights:{value:n},latitudinal:{value:!1},dTheta:{value:0},mipInt:{value:0},poleAxis:{value:i}},vertexShader:rc(),fragmentShader:`

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
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function pf(){return new mt({name:"EquirectangularToCubeUV",uniforms:{envMap:{value:null}},vertexShader:rc(),fragmentShader:`

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
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function mf(){return new mt({name:"CubemapToCubeUV",uniforms:{envMap:{value:null},flipEnvMap:{value:-1}},vertexShader:rc(),fragmentShader:`

			precision mediump float;
			precision mediump int;

			uniform float flipEnvMap;

			varying vec3 vOutputDirection;

			uniform samplerCube envMap;

			void main() {

				gl_FragColor = textureCube( envMap, vec3( flipEnvMap * vOutputDirection.x, vOutputDirection.yz ) );

			}
		`,blending:Wn,depthTest:!1,depthWrite:!1})}function rc(){return`

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
	`}var ic=class extends Wt{constructor(e=1,t={}){super(e,e,t),this.isWebGLCubeRenderTarget=!0;let n={width:e,height:e,depth:1},i=[n,n,n,n,n,n];this.texture=new co(i),this._setTextureOptions(t),this.texture.isRenderTargetTexture=!0}fromEquirectangularTexture(e,t){this.texture.type=t.type,this.texture.colorSpace=t.colorSpace,this.texture.generateMipmaps=t.generateMipmaps,this.texture.minFilter=t.minFilter,this.texture.magFilter=t.magFilter;let n={uniforms:{tEquirect:{value:null}},vertexShader:`

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
			`},i=new mi(5,5,5),r=new mt({name:"CubemapFromEquirect",uniforms:Bs(n.uniforms),vertexShader:n.vertexShader,fragmentShader:n.fragmentShader,side:tn,blending:Wn});r.uniforms.tEquirect.value=t;let o=new Ke(i,r),a=t.minFilter;return t.minFilter===si&&(t.minFilter=kt),new rl(1,10,this).update(e,o),t.minFilter=a,o.geometry.dispose(),o.material.dispose(),this}clear(e,t=!0,n=!0,i=!0){let r=e.getRenderTarget();for(let o=0;o<6;o++)e.setRenderTarget(this,o),e.clear(t,n,i);e.setRenderTarget(r)}};function zx(s){let e=new WeakMap,t=new WeakMap,n=null;function i(d,f=!1){return d==null?null:f?o(d):r(d)}function r(d){if(d&&d.isTexture){let f=d.mapping;if(f===ul||f===dl)if(e.has(d)){let g=e.get(d).texture;return a(g,d.mapping)}else{let g=d.image;if(g&&g.height>0){let y=new ic(g.height);return y.fromEquirectangularTexture(s,d),e.set(d,y),d.addEventListener("dispose",c),a(y.texture,d.mapping)}else return null}}return d}function o(d){if(d&&d.isTexture){let f=d.mapping,g=f===ul||f===dl,y=f===as||f===Fs;if(g||y){let m=t.get(d),p=m!==void 0?m.texture.pmremVersion:0;if(d.isRenderTargetTexture&&d.pmremVersion!==p)return n===null&&(n=new Dr(s)),m=g?n.fromEquirectangular(d,m):n.fromCubemap(d,m),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),m.texture;if(m!==void 0)return m.texture;{let b=d.image;return g&&b&&b.height>0||y&&b&&l(b)?(n===null&&(n=new Dr(s)),m=g?n.fromEquirectangular(d):n.fromCubemap(d),m.texture.pmremVersion=d.pmremVersion,t.set(d,m),d.addEventListener("dispose",h),m.texture):null}}}return d}function a(d,f){return f===ul?d.mapping=as:f===dl&&(d.mapping=Fs),d}function l(d){let f=0,g=6;for(let y=0;y<g;y++)d[y]!==void 0&&f++;return f===g}function c(d){let f=d.target;f.removeEventListener("dispose",c);let g=e.get(f);g!==void 0&&(e.delete(f),g.dispose())}function h(d){let f=d.target;f.removeEventListener("dispose",h);let g=t.get(f);g!==void 0&&(t.delete(f),g.dispose())}function u(){e=new WeakMap,t=new WeakMap,n!==null&&(n.dispose(),n=null)}return{get:i,dispose:u}}function kx(s){let e={};function t(n){if(e[n]!==void 0)return e[n];let i=s.getExtension(n);return e[n]=i,i}return{has:function(n){return t(n)!==null},init:function(){t("EXT_color_buffer_float"),t("WEBGL_clip_cull_distance"),t("OES_texture_float_linear"),t("EXT_color_buffer_half_float"),t("WEBGL_multisampled_render_to_texture"),t("WEBGL_render_shared_exponent")},get:function(n){let i=t(n);return i===null&&bs("WebGLRenderer: "+n+" extension not supported."),i}}}function Hx(s,e,t,n){let i={},r=new WeakMap;function o(u){let d=u.target;d.index!==null&&e.remove(d.index);for(let g in d.attributes)e.remove(d.attributes[g]);d.removeEventListener("dispose",o),delete i[d.id];let f=r.get(d);f&&(e.remove(f),r.delete(d)),n.releaseStatesOfGeometry(d),d.isInstancedBufferGeometry===!0&&delete d._maxInstanceCount,t.memory.geometries--}function a(u,d){return i[d.id]===!0||(d.addEventListener("dispose",o),i[d.id]=!0,t.memory.geometries++),d}function l(u){let d=u.attributes;for(let f in d)e.update(d[f],s.ARRAY_BUFFER)}function c(u){let d=[],f=u.index,g=u.attributes.position,y=0;if(g===void 0)return;if(f!==null){let b=f.array;y=f.version;for(let S=0,x=b.length;S<x;S+=3){let A=b[S+0],T=b[S+1],I=b[S+2];d.push(A,T,T,I,I,A)}}else{let b=g.array;y=g.version;for(let S=0,x=b.length/3-1;S<x;S+=3){let A=S+0,T=S+1,I=S+2;d.push(A,T,T,I,I,A)}}let m=new(g.count>=65535?ro:so)(d,1);m.version=y;let p=r.get(u);p&&e.remove(p),r.set(u,m)}function h(u){let d=r.get(u);if(d){let f=u.index;f!==null&&d.version<f.version&&c(u)}else c(u);return r.get(u)}return{get:a,update:l,getWireframeAttribute:h}}function Vx(s,e,t){let n;function i(u){n=u}let r,o;function a(u){r=u.type,o=u.bytesPerElement}function l(u,d){s.drawElements(n,d,r,u*o),t.update(d,n,1)}function c(u,d,f){f!==0&&(s.drawElementsInstanced(n,d,r,u*o,f),t.update(d,n,f))}function h(u,d,f){if(f===0)return;e.get("WEBGL_multi_draw").multiDrawElementsWEBGL(n,d,0,r,u,0,f);let y=0;for(let m=0;m<f;m++)y+=d[m];t.update(y,n,1)}this.setMode=i,this.setIndex=a,this.render=l,this.renderInstances=c,this.renderMultiDraw=h}function Gx(s){let e={geometries:0,textures:0},t={frame:0,calls:0,triangles:0,points:0,lines:0};function n(r,o,a){switch(t.calls++,o){case s.TRIANGLES:t.triangles+=a*(r/3);break;case s.LINES:t.lines+=a*(r/2);break;case s.LINE_STRIP:t.lines+=a*(r-1);break;case s.LINE_LOOP:t.lines+=a*r;break;case s.POINTS:t.points+=a*r;break;default:$e("WebGLInfo: Unknown draw mode:",o);break}}function i(){t.calls=0,t.triangles=0,t.points=0,t.lines=0}return{memory:e,render:t,programs:null,autoReset:!0,reset:i,update:n}}function Wx(s,e,t){let n=new WeakMap,i=new _t;function r(o,a,l){let c=o.morphTargetInfluences,h=a.morphAttributes.position||a.morphAttributes.normal||a.morphAttributes.color,u=h!==void 0?h.length:0,d=n.get(a);if(d===void 0||d.count!==u){let D=function(){I.dispose(),n.delete(a),a.removeEventListener("dispose",D)};d!==void 0&&d.texture.dispose();let f=a.morphAttributes.position!==void 0,g=a.morphAttributes.normal!==void 0,y=a.morphAttributes.color!==void 0,m=a.morphAttributes.position||[],p=a.morphAttributes.normal||[],b=a.morphAttributes.color||[],S=0;f===!0&&(S=1),g===!0&&(S=2),y===!0&&(S=3);let x=a.attributes.position.count*S,A=1;x>e.maxTextureSize&&(A=Math.ceil(x/e.maxTextureSize),x=e.maxTextureSize);let T=new Float32Array(x*A*4*u),I=new no(T,x,A,u);I.type=Un,I.needsUpdate=!0;let v=S*4;for(let w=0;w<u;w++){let R=m[w],L=p[w],V=b[w],C=x*A*4*w;for(let N=0;N<R.count;N++){let U=N*v;f===!0&&(i.fromBufferAttribute(R,N),T[C+U+0]=i.x,T[C+U+1]=i.y,T[C+U+2]=i.z,T[C+U+3]=0),g===!0&&(i.fromBufferAttribute(L,N),T[C+U+4]=i.x,T[C+U+5]=i.y,T[C+U+6]=i.z,T[C+U+7]=0),y===!0&&(i.fromBufferAttribute(V,N),T[C+U+8]=i.x,T[C+U+9]=i.y,T[C+U+10]=i.z,T[C+U+11]=V.itemSize===4?i.w:1)}}d={count:u,texture:I,size:new _e(x,A)},n.set(a,d),a.addEventListener("dispose",D)}if(o.isInstancedMesh===!0&&o.morphTexture!==null)l.getUniforms().setValue(s,"morphTexture",o.morphTexture,t);else{let f=0;for(let y=0;y<c.length;y++)f+=c[y];let g=a.morphTargetsRelative?1:1-f;l.getUniforms().setValue(s,"morphTargetBaseInfluence",g),l.getUniforms().setValue(s,"morphTargetInfluences",c)}l.getUniforms().setValue(s,"morphTargetsTexture",d.texture,t),l.getUniforms().setValue(s,"morphTargetsTextureSize",d.size)}return{update:r}}function Xx(s,e,t,n,i){let r=new WeakMap;function o(c){let h=i.render.frame,u=c.geometry,d=e.get(c,u);if(r.get(d)!==h&&(e.update(d),r.set(d,h)),c.isInstancedMesh&&(c.hasEventListener("dispose",l)===!1&&c.addEventListener("dispose",l),r.get(c)!==h&&(t.update(c.instanceMatrix,s.ARRAY_BUFFER),c.instanceColor!==null&&t.update(c.instanceColor,s.ARRAY_BUFFER),r.set(c,h))),c.isSkinnedMesh){let f=c.skeleton;r.get(f)!==h&&(f.update(),r.set(f,h))}return d}function a(){r=new WeakMap}function l(c){let h=c.target;h.removeEventListener("dispose",l),n.releaseStatesOfObject(h),t.remove(h.instanceMatrix),h.instanceColor!==null&&t.remove(h.instanceColor)}return{update:o,dispose:a}}var qx={[Po]:"LINEAR_TONE_MAPPING",[Io]:"REINHARD_TONE_MAPPING",[Lo]:"CINEON_TONE_MAPPING",[Us]:"ACES_FILMIC_TONE_MAPPING",[No]:"AGX_TONE_MAPPING",[Uo]:"NEUTRAL_TONE_MAPPING",[Do]:"CUSTOM_TONE_MAPPING"};function Yx(s,e,t,n,i,r){let o=new Wt(e,t,{type:s,depthBuffer:i,stencilBuffer:r,samples:n?4:0,depthTexture:i?new Ni(e,t):void 0}),a=new Wt(e,t,{type:nn,depthBuffer:!1,stencilBuffer:!1}),l=new pt;l.setAttribute("position",new nt([-1,3,0,-1,-1,0,3,-1,0],3)),l.setAttribute("uv",new nt([0,2,0,0,2,0],2));let c=new Sr({uniforms:{tDiffuse:{value:null}},vertexShader:`
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
			}`,depthTest:!1,depthWrite:!1}),h=new Ke(l,c),u=new vi(-1,1,1,-1,0,1),d=null,f=null,g=!1,y,m=null,p=[],b=!1;this.setSize=function(S,x){o.setSize(S,x),a.setSize(S,x);for(let A=0;A<p.length;A++){let T=p[A];T.setSize&&T.setSize(S,x)}},this.setEffects=function(S){p=S,b=p.length>0&&p[0].isRenderPass===!0;let x=o.width,A=o.height;for(let T=0;T<p.length;T++){let I=p[T];I.setSize&&I.setSize(x,A)}},this.begin=function(S,x){if(g||S.toneMapping===ii&&p.length===0)return!1;if(m=x,x!==null){let A=x.width,T=x.height;(o.width!==A||o.height!==T)&&this.setSize(A,T)}return b===!1&&S.setRenderTarget(o),y=S.toneMapping,S.toneMapping=ii,!0},this.hasRenderPass=function(){return b},this.end=function(S,x){S.toneMapping=y,g=!0;let A=o,T=a;for(let I=0;I<p.length;I++){let v=p[I];if(v.enabled!==!1&&(v.render(S,T,A,x),v.needsSwap!==!1)){let D=A;A=T,T=D}}if(d!==S.outputColorSpace||f!==S.toneMapping){d=S.outputColorSpace,f=S.toneMapping,c.defines={},ot.getTransfer(d)===Mt&&(c.defines.SRGB_TRANSFER="");let I=qx[f];I&&(c.defines[I]=""),c.needsUpdate=!0}c.uniforms.tDiffuse.value=A.texture,S.setRenderTarget(m),S.render(h,u),m=null,g=!1},this.isCompositing=function(){return g},this.dispose=function(){o.depthTexture&&o.depthTexture.dispose(),o.dispose(),a.dispose(),l.dispose(),c.dispose()}}var Uf=new jt,Fh=new Ni(1,1),Ff=new no,Of=new ka,Bf=new co,gf=[],xf=[],_f=new Float32Array(16),vf=new Float32Array(9),yf=new Float32Array(4);function Nr(s,e,t){let n=s[0];if(n<=0||n>0)return s;let i=e*t,r=gf[i];if(r===void 0&&(r=new Float32Array(i),gf[i]=r),e!==0){n.toArray(r,0);for(let o=1,a=0;o!==e;++o)a+=t,s[o].toArray(r,a)}return r}function sn(s,e){if(s.length!==e.length)return!1;for(let t=0,n=s.length;t<n;t++)if(s[t]!==e[t])return!1;return!0}function rn(s,e){for(let t=0,n=e.length;t<n;t++)s[t]=e[t]}function oc(s,e){let t=xf[e];t===void 0&&(t=new Int32Array(e),xf[e]=t);for(let n=0;n!==e;++n)t[n]=s.allocateTextureUnit();return t}function Zx(s,e){let t=this.cache;t[0]!==e&&(s.uniform1f(this.addr,e),t[0]=e)}function Kx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2f(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(sn(t,e))return;s.uniform2fv(this.addr,e),rn(t,e)}}function jx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3f(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else if(e.r!==void 0)(t[0]!==e.r||t[1]!==e.g||t[2]!==e.b)&&(s.uniform3f(this.addr,e.r,e.g,e.b),t[0]=e.r,t[1]=e.g,t[2]=e.b);else{if(sn(t,e))return;s.uniform3fv(this.addr,e),rn(t,e)}}function Jx(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4f(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(sn(t,e))return;s.uniform4fv(this.addr,e),rn(t,e)}}function $x(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(sn(t,e))return;s.uniformMatrix2fv(this.addr,!1,e),rn(t,e)}else{if(sn(t,n))return;yf.set(n),s.uniformMatrix2fv(this.addr,!1,yf),rn(t,n)}}function Qx(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(sn(t,e))return;s.uniformMatrix3fv(this.addr,!1,e),rn(t,e)}else{if(sn(t,n))return;vf.set(n),s.uniformMatrix3fv(this.addr,!1,vf),rn(t,n)}}function e_(s,e){let t=this.cache,n=e.elements;if(n===void 0){if(sn(t,e))return;s.uniformMatrix4fv(this.addr,!1,e),rn(t,e)}else{if(sn(t,n))return;_f.set(n),s.uniformMatrix4fv(this.addr,!1,_f),rn(t,n)}}function t_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1i(this.addr,e),t[0]=e)}function n_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2i(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(sn(t,e))return;s.uniform2iv(this.addr,e),rn(t,e)}}function i_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3i(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(sn(t,e))return;s.uniform3iv(this.addr,e),rn(t,e)}}function s_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4i(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(sn(t,e))return;s.uniform4iv(this.addr,e),rn(t,e)}}function r_(s,e){let t=this.cache;t[0]!==e&&(s.uniform1ui(this.addr,e),t[0]=e)}function o_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y)&&(s.uniform2ui(this.addr,e.x,e.y),t[0]=e.x,t[1]=e.y);else{if(sn(t,e))return;s.uniform2uiv(this.addr,e),rn(t,e)}}function a_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z)&&(s.uniform3ui(this.addr,e.x,e.y,e.z),t[0]=e.x,t[1]=e.y,t[2]=e.z);else{if(sn(t,e))return;s.uniform3uiv(this.addr,e),rn(t,e)}}function l_(s,e){let t=this.cache;if(e.x!==void 0)(t[0]!==e.x||t[1]!==e.y||t[2]!==e.z||t[3]!==e.w)&&(s.uniform4ui(this.addr,e.x,e.y,e.z,e.w),t[0]=e.x,t[1]=e.y,t[2]=e.z,t[3]=e.w);else{if(sn(t,e))return;s.uniform4uiv(this.addr,e),rn(t,e)}}function c_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i);let r;this.type===s.SAMPLER_2D_SHADOW?(Fh.compareFunction=t.isReversedDepthBuffer()?ec:Ql,r=Fh):r=Uf,t.setTexture2D(e||r,i)}function h_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture3D(e||Of,i)}function u_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTextureCube(e||Bf,i)}function d_(s,e,t){let n=this.cache,i=t.allocateTextureUnit();n[0]!==i&&(s.uniform1i(this.addr,i),n[0]=i),t.setTexture2DArray(e||Ff,i)}function f_(s){switch(s){case 5126:return Zx;case 35664:return Kx;case 35665:return jx;case 35666:return Jx;case 35674:return $x;case 35675:return Qx;case 35676:return e_;case 5124:case 35670:return t_;case 35667:case 35671:return n_;case 35668:case 35672:return i_;case 35669:case 35673:return s_;case 5125:return r_;case 36294:return o_;case 36295:return a_;case 36296:return l_;case 35678:case 36198:case 36298:case 36306:case 35682:return c_;case 35679:case 36299:case 36307:return h_;case 35680:case 36300:case 36308:case 36293:return u_;case 36289:case 36303:case 36311:case 36292:return d_}}function p_(s,e){s.uniform1fv(this.addr,e)}function m_(s,e){let t=Nr(e,this.size,2);s.uniform2fv(this.addr,t)}function g_(s,e){let t=Nr(e,this.size,3);s.uniform3fv(this.addr,t)}function x_(s,e){let t=Nr(e,this.size,4);s.uniform4fv(this.addr,t)}function __(s,e){let t=Nr(e,this.size,4);s.uniformMatrix2fv(this.addr,!1,t)}function v_(s,e){let t=Nr(e,this.size,9);s.uniformMatrix3fv(this.addr,!1,t)}function y_(s,e){let t=Nr(e,this.size,16);s.uniformMatrix4fv(this.addr,!1,t)}function M_(s,e){s.uniform1iv(this.addr,e)}function b_(s,e){s.uniform2iv(this.addr,e)}function S_(s,e){s.uniform3iv(this.addr,e)}function w_(s,e){s.uniform4iv(this.addr,e)}function E_(s,e){s.uniform1uiv(this.addr,e)}function T_(s,e){s.uniform2uiv(this.addr,e)}function A_(s,e){s.uniform3uiv(this.addr,e)}function R_(s,e){s.uniform4uiv(this.addr,e)}function C_(s,e,t){let n=this.cache,i=e.length,r=oc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));let o;this.type===s.SAMPLER_2D_SHADOW?o=Fh:o=Uf;for(let a=0;a!==i;++a)t.setTexture2D(e[a]||o,r[a])}function P_(s,e,t){let n=this.cache,i=e.length,r=oc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));for(let o=0;o!==i;++o)t.setTexture3D(e[o]||Of,r[o])}function I_(s,e,t){let n=this.cache,i=e.length,r=oc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));for(let o=0;o!==i;++o)t.setTextureCube(e[o]||Bf,r[o])}function L_(s,e,t){let n=this.cache,i=e.length,r=oc(t,i);sn(n,r)||(s.uniform1iv(this.addr,r),rn(n,r));for(let o=0;o!==i;++o)t.setTexture2DArray(e[o]||Ff,r[o])}function D_(s){switch(s){case 5126:return p_;case 35664:return m_;case 35665:return g_;case 35666:return x_;case 35674:return __;case 35675:return v_;case 35676:return y_;case 5124:case 35670:return M_;case 35667:case 35671:return b_;case 35668:case 35672:return S_;case 35669:case 35673:return w_;case 5125:return E_;case 36294:return T_;case 36295:return A_;case 36296:return R_;case 35678:case 36198:case 36298:case 36306:case 35682:return C_;case 35679:case 36299:case 36307:return P_;case 35680:case 36300:case 36308:case 36293:return I_;case 36289:case 36303:case 36311:case 36292:return L_}}var Oh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.setValue=f_(t.type)}},Bh=class{constructor(e,t,n){this.id=e,this.addr=n,this.cache=[],this.type=t.type,this.size=t.size,this.setValue=D_(t.type)}},zh=class{constructor(e){this.id=e,this.seq=[],this.map={}}setValue(e,t,n){let i=this.seq;for(let r=0,o=i.length;r!==o;++r){let a=i[r];a.setValue(e,t[a.id],n)}}},Nh=/(\w+)(\])?(\[|\.)?/g;function Mf(s,e){s.seq.push(e),s.map[e.id]=e}function N_(s,e,t){let n=s.name,i=n.length;for(Nh.lastIndex=0;;){let r=Nh.exec(n),o=Nh.lastIndex,a=r[1],l=r[2]==="]",c=r[3];if(l&&(a=a|0),c===void 0||c==="["&&o+2===i){Mf(t,c===void 0?new Oh(a,s,e):new Bh(a,s,e));break}else{let u=t.map[a];u===void 0&&(u=new zh(a),Mf(t,u)),t=u}}}var Lr=class{constructor(e,t){this.seq=[],this.map={};let n=e.getProgramParameter(t,e.ACTIVE_UNIFORMS);for(let o=0;o<n;++o){let a=e.getActiveUniform(t,o),l=e.getUniformLocation(t,a.name);N_(a,l,this)}let i=[],r=[];for(let o of this.seq)o.type===e.SAMPLER_2D_SHADOW||o.type===e.SAMPLER_CUBE_SHADOW||o.type===e.SAMPLER_2D_ARRAY_SHADOW?i.push(o):r.push(o);i.length>0&&(this.seq=i.concat(r))}setValue(e,t,n,i){let r=this.map[t];r!==void 0&&r.setValue(e,n,i)}setOptional(e,t,n){let i=t[n];i!==void 0&&this.setValue(e,n,i)}static upload(e,t,n,i){for(let r=0,o=t.length;r!==o;++r){let a=t[r],l=n[a.id];l.needsUpdate!==!1&&a.setValue(e,l.value,i)}}static seqWithValue(e,t){let n=[];for(let i=0,r=e.length;i!==r;++i){let o=e[i];o.id in t&&n.push(o)}return n}};function bf(s,e,t){let n=s.createShader(e);return s.shaderSource(n,t),s.compileShader(n),n}var U_=37297,F_=0;function O_(s,e){let t=s.split(`
`),n=[],i=Math.max(e-6,0),r=Math.min(e+6,t.length);for(let o=i;o<r;o++){let a=o+1;n.push(`${a===e?">":" "} ${a}: ${t[o]}`)}return n.join(`
`)}var Sf=new et;function B_(s){ot._getMatrix(Sf,ot.workingColorSpace,s);let e=`mat3( ${Sf.elements.map(t=>t.toFixed(4))} )`;switch(ot.getTransfer(s)){case eo:return[e,"LinearTransferOETF"];case Mt:return[e,"sRGBTransferOETF"];default:return We("WebGLProgram: Unsupported color space: ",s),[e,"LinearTransferOETF"]}}function wf(s,e,t){let n=s.getShaderParameter(e,s.COMPILE_STATUS),r=(s.getShaderInfoLog(e)||"").trim();if(n&&r==="")return"";let o=/ERROR: 0:(\d+)/.exec(r);if(o){let a=parseInt(o[1]);return t.toUpperCase()+`

`+r+`

`+O_(s.getShaderSource(e),a)}else return r}function z_(s,e){let t=B_(e);return[`vec4 ${s}( vec4 value ) {`,`	return ${t[1]}( vec4( value.rgb * ${t[0]}, value.a ) );`,"}"].join(`
`)}var k_={[Po]:"Linear",[Io]:"Reinhard",[Lo]:"Cineon",[Us]:"ACESFilmic",[No]:"AgX",[Uo]:"Neutral",[Do]:"Custom"};function H_(s,e){let t=k_[e];return t===void 0?(We("WebGLProgram: Unsupported toneMapping:",e),"vec3 "+s+"( vec3 color ) { return LinearToneMapping( color ); }"):"vec3 "+s+"( vec3 color ) { return "+t+"ToneMapping( color ); }"}var nc=new B;function V_(){ot.getLuminanceCoefficients(nc);let s=nc.x.toFixed(4),e=nc.y.toFixed(4),t=nc.z.toFixed(4);return["float luminance( const in vec3 rgb ) {",`	const vec3 weights = vec3( ${s}, ${e}, ${t} );`,"	return dot( weights, rgb );","}"].join(`
`)}function G_(s){return[s.extensionClipCullDistance?"#extension GL_ANGLE_clip_cull_distance : require":"",s.extensionMultiDraw?"#extension GL_ANGLE_multi_draw : require":""].filter(Yo).join(`
`)}function W_(s){let e=[];for(let t in s){let n=s[t];n!==!1&&e.push("#define "+t+" "+n)}return e.join(`
`)}function X_(s,e){let t={},n=s.getProgramParameter(e,s.ACTIVE_ATTRIBUTES);for(let i=0;i<n;i++){let r=s.getActiveAttrib(e,i),o=r.name,a=1;r.type===s.FLOAT_MAT2&&(a=2),r.type===s.FLOAT_MAT3&&(a=3),r.type===s.FLOAT_MAT4&&(a=4),t[o]={type:r.type,location:s.getAttribLocation(e,o),locationSize:a}}return t}function Yo(s){return s!==""}function Ef(s,e){let t=e.numSpotLightShadows+e.numSpotLightMaps-e.numSpotLightShadowsWithMaps;return s.replace(/NUM_DIR_LIGHTS/g,e.numDirLights).replace(/NUM_SPOT_LIGHTS/g,e.numSpotLights).replace(/NUM_SPOT_LIGHT_MAPS/g,e.numSpotLightMaps).replace(/NUM_SPOT_LIGHT_COORDS/g,t).replace(/NUM_RECT_AREA_LIGHTS/g,e.numRectAreaLights).replace(/NUM_POINT_LIGHTS/g,e.numPointLights).replace(/NUM_HEMI_LIGHTS/g,e.numHemiLights).replace(/NUM_DIR_LIGHT_SHADOWS/g,e.numDirLightShadows).replace(/NUM_SPOT_LIGHT_SHADOWS_WITH_MAPS/g,e.numSpotLightShadowsWithMaps).replace(/NUM_SPOT_LIGHT_SHADOWS/g,e.numSpotLightShadows).replace(/NUM_POINT_LIGHT_SHADOWS/g,e.numPointLightShadows)}function Tf(s,e){return s.replace(/NUM_CLIPPING_PLANES/g,e.numClippingPlanes).replace(/UNION_CLIPPING_PLANES/g,e.numClippingPlanes-e.numClipIntersection)}var q_=/^[ \t]*#include +<([\w\d./]+)>/gm;function kh(s){return s.replace(q_,Z_)}var Y_=new Map;function Z_(s,e){let t=ht[e];if(t===void 0){let n=Y_.get(e);if(n!==void 0)t=ht[n],We('WebGLRenderer: Shader chunk "%s" has been deprecated. Use "%s" instead.',e,n);else throw new Error("THREE.WebGLProgram: Can not resolve #include <"+e+">")}return kh(t)}var K_=/#pragma unroll_loop_start\s+for\s*\(\s*int\s+i\s*=\s*(\d+)\s*;\s*i\s*<\s*(\d+)\s*;\s*i\s*\+\+\s*\)\s*{([\s\S]+?)}\s+#pragma unroll_loop_end/g;function Af(s){return s.replace(K_,j_)}function j_(s,e,t,n){let i="";for(let r=parseInt(e);r<parseInt(t);r++)i+=n.replace(/\[\s*i\s*\]/g,"[ "+r+" ]").replace(/UNROLLED_LOOP_INDEX/g,r);return i}function Rf(s){let e=`precision ${s.precision} float;
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
#define LOW_PRECISION`),e}var J_={[Co]:"SHADOWMAP_TYPE_PCF",[Tr]:"SHADOWMAP_TYPE_VSM"};function $_(s){return J_[s.shadowMapType]||"SHADOWMAP_TYPE_BASIC"}var Q_={[as]:"ENVMAP_TYPE_CUBE",[Fs]:"ENVMAP_TYPE_CUBE",[Fo]:"ENVMAP_TYPE_CUBE_UV"};function ev(s){return s.envMap===!1?"ENVMAP_TYPE_CUBE":Q_[s.envMapMode]||"ENVMAP_TYPE_CUBE"}var tv={[Fs]:"ENVMAP_MODE_REFRACTION"};function nv(s){return s.envMap===!1?"ENVMAP_MODE_REFLECTION":tv[s.envMapMode]||"ENVMAP_MODE_REFLECTION"}var iv={[hl]:"ENVMAP_BLENDING_MULTIPLY",[Wd]:"ENVMAP_BLENDING_MIX",[Xd]:"ENVMAP_BLENDING_ADD"};function sv(s){return s.envMap===!1?"ENVMAP_BLENDING_NONE":iv[s.combine]||"ENVMAP_BLENDING_NONE"}function rv(s){let e=s.envMapCubeUVHeight;if(e===null)return null;let t=Math.log2(e)-2,n=1/e;return{texelWidth:1/(3*Math.max(Math.pow(2,t),112)),texelHeight:n,maxMip:t}}function ov(s,e,t,n){let i=s.getContext(),r=t.defines,o=t.vertexShader,a=t.fragmentShader,l=$_(t),c=ev(t),h=nv(t),u=sv(t),d=rv(t),f=G_(t),g=W_(r),y=i.createProgram(),m,p,b=t.glslVersion?"#version "+t.glslVersion+`
`:"";t.isRawShaderMaterial?(m=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Yo).join(`
`),m.length>0&&(m+=`
`),p=["#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g].filter(Yo).join(`
`),p.length>0&&(p+=`
`)):(m=[Rf(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.extensionClipCullDistance?"#define USE_CLIP_DISTANCE":"",t.batching?"#define USE_BATCHING":"",t.batchingColor?"#define USE_BATCHING_COLOR":"",t.instancing?"#define USE_INSTANCING":"",t.instancingColor?"#define USE_INSTANCING_COLOR":"",t.instancingMorph?"#define USE_INSTANCING_MORPH":"",t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.map?"#define USE_MAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+h:"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.displacementMap?"#define USE_DISPLACEMENTMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.mapUv?"#define MAP_UV "+t.mapUv:"",t.alphaMapUv?"#define ALPHAMAP_UV "+t.alphaMapUv:"",t.lightMapUv?"#define LIGHTMAP_UV "+t.lightMapUv:"",t.aoMapUv?"#define AOMAP_UV "+t.aoMapUv:"",t.emissiveMapUv?"#define EMISSIVEMAP_UV "+t.emissiveMapUv:"",t.bumpMapUv?"#define BUMPMAP_UV "+t.bumpMapUv:"",t.normalMapUv?"#define NORMALMAP_UV "+t.normalMapUv:"",t.displacementMapUv?"#define DISPLACEMENTMAP_UV "+t.displacementMapUv:"",t.metalnessMapUv?"#define METALNESSMAP_UV "+t.metalnessMapUv:"",t.roughnessMapUv?"#define ROUGHNESSMAP_UV "+t.roughnessMapUv:"",t.anisotropyMapUv?"#define ANISOTROPYMAP_UV "+t.anisotropyMapUv:"",t.clearcoatMapUv?"#define CLEARCOATMAP_UV "+t.clearcoatMapUv:"",t.clearcoatNormalMapUv?"#define CLEARCOAT_NORMALMAP_UV "+t.clearcoatNormalMapUv:"",t.clearcoatRoughnessMapUv?"#define CLEARCOAT_ROUGHNESSMAP_UV "+t.clearcoatRoughnessMapUv:"",t.iridescenceMapUv?"#define IRIDESCENCEMAP_UV "+t.iridescenceMapUv:"",t.iridescenceThicknessMapUv?"#define IRIDESCENCE_THICKNESSMAP_UV "+t.iridescenceThicknessMapUv:"",t.sheenColorMapUv?"#define SHEEN_COLORMAP_UV "+t.sheenColorMapUv:"",t.sheenRoughnessMapUv?"#define SHEEN_ROUGHNESSMAP_UV "+t.sheenRoughnessMapUv:"",t.specularMapUv?"#define SPECULARMAP_UV "+t.specularMapUv:"",t.specularColorMapUv?"#define SPECULAR_COLORMAP_UV "+t.specularColorMapUv:"",t.specularIntensityMapUv?"#define SPECULAR_INTENSITYMAP_UV "+t.specularIntensityMapUv:"",t.transmissionMapUv?"#define TRANSMISSIONMAP_UV "+t.transmissionMapUv:"",t.thicknessMapUv?"#define THICKNESSMAP_UV "+t.thicknessMapUv:"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexNormals?"#define HAS_NORMAL":"",t.vertexColors?"#define USE_COLOR":"",t.vertexAlphas?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.flatShading?"#define FLAT_SHADED":"",t.skinning?"#define USE_SKINNING":"",t.morphTargets?"#define USE_MORPHTARGETS":"",t.morphNormals&&t.flatShading===!1?"#define USE_MORPHNORMALS":"",t.morphColors?"#define USE_MORPHCOLORS":"",t.morphTargetsCount>0?"#define MORPHTARGETS_TEXTURE_STRIDE "+t.morphTextureStride:"",t.morphTargetsCount>0?"#define MORPHTARGETS_COUNT "+t.morphTargetsCount:"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.sizeAttenuation?"#define USE_SIZEATTENUATION":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 modelMatrix;","uniform mat4 modelViewMatrix;","uniform mat4 projectionMatrix;","uniform mat4 viewMatrix;","uniform mat3 normalMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;","#ifdef USE_INSTANCING","	attribute mat4 instanceMatrix;","#endif","#ifdef USE_INSTANCING_COLOR","	attribute vec3 instanceColor;","#endif","#ifdef USE_INSTANCING_MORPH","	uniform sampler2D morphTexture;","#endif","attribute vec3 position;","attribute vec3 normal;","attribute vec2 uv;","#ifdef USE_UV1","	attribute vec2 uv1;","#endif","#ifdef USE_UV2","	attribute vec2 uv2;","#endif","#ifdef USE_UV3","	attribute vec2 uv3;","#endif","#ifdef USE_TANGENT","	attribute vec4 tangent;","#endif","#if defined( USE_COLOR_ALPHA )","	attribute vec4 color;","#elif defined( USE_COLOR )","	attribute vec3 color;","#endif","#ifdef USE_SKINNING","	attribute vec4 skinIndex;","	attribute vec4 skinWeight;","#endif",`
`].filter(Yo).join(`
`),p=[Rf(t),"#define SHADER_TYPE "+t.shaderType,"#define SHADER_NAME "+t.shaderName,g,t.useFog&&t.fog?"#define USE_FOG":"",t.useFog&&t.fogExp2?"#define FOG_EXP2":"",t.alphaToCoverage?"#define ALPHA_TO_COVERAGE":"",t.map?"#define USE_MAP":"",t.matcap?"#define USE_MATCAP":"",t.envMap?"#define USE_ENVMAP":"",t.envMap?"#define "+c:"",t.envMap?"#define "+h:"",t.envMap?"#define "+u:"",d?"#define CUBEUV_TEXEL_WIDTH "+d.texelWidth:"",d?"#define CUBEUV_TEXEL_HEIGHT "+d.texelHeight:"",d?"#define CUBEUV_MAX_MIP "+d.maxMip+".0":"",t.lightMap?"#define USE_LIGHTMAP":"",t.aoMap?"#define USE_AOMAP":"",t.bumpMap?"#define USE_BUMPMAP":"",t.normalMap?"#define USE_NORMALMAP":"",t.normalMapObjectSpace?"#define USE_NORMALMAP_OBJECTSPACE":"",t.normalMapTangentSpace?"#define USE_NORMALMAP_TANGENTSPACE":"",t.packedNormalMap?"#define USE_PACKED_NORMALMAP":"",t.emissiveMap?"#define USE_EMISSIVEMAP":"",t.anisotropy?"#define USE_ANISOTROPY":"",t.anisotropyMap?"#define USE_ANISOTROPYMAP":"",t.clearcoat?"#define USE_CLEARCOAT":"",t.clearcoatMap?"#define USE_CLEARCOATMAP":"",t.clearcoatRoughnessMap?"#define USE_CLEARCOAT_ROUGHNESSMAP":"",t.clearcoatNormalMap?"#define USE_CLEARCOAT_NORMALMAP":"",t.dispersion?"#define USE_DISPERSION":"",t.iridescence?"#define USE_IRIDESCENCE":"",t.iridescenceMap?"#define USE_IRIDESCENCEMAP":"",t.iridescenceThicknessMap?"#define USE_IRIDESCENCE_THICKNESSMAP":"",t.specularMap?"#define USE_SPECULARMAP":"",t.specularColorMap?"#define USE_SPECULAR_COLORMAP":"",t.specularIntensityMap?"#define USE_SPECULAR_INTENSITYMAP":"",t.roughnessMap?"#define USE_ROUGHNESSMAP":"",t.metalnessMap?"#define USE_METALNESSMAP":"",t.alphaMap?"#define USE_ALPHAMAP":"",t.alphaTest?"#define USE_ALPHATEST":"",t.alphaHash?"#define USE_ALPHAHASH":"",t.sheen?"#define USE_SHEEN":"",t.sheenColorMap?"#define USE_SHEEN_COLORMAP":"",t.sheenRoughnessMap?"#define USE_SHEEN_ROUGHNESSMAP":"",t.transmission?"#define USE_TRANSMISSION":"",t.transmissionMap?"#define USE_TRANSMISSIONMAP":"",t.thicknessMap?"#define USE_THICKNESSMAP":"",t.vertexTangents&&t.flatShading===!1?"#define USE_TANGENT":"",t.vertexColors||t.instancingColor?"#define USE_COLOR":"",t.vertexAlphas||t.batchingColor?"#define USE_COLOR_ALPHA":"",t.vertexUv1s?"#define USE_UV1":"",t.vertexUv2s?"#define USE_UV2":"",t.vertexUv3s?"#define USE_UV3":"",t.pointsUvs?"#define USE_POINTS_UV":"",t.gradientMap?"#define USE_GRADIENTMAP":"",t.flatShading?"#define FLAT_SHADED":"",t.doubleSided?"#define DOUBLE_SIDED":"",t.flipSided?"#define FLIP_SIDED":"",t.shadowMapEnabled?"#define USE_SHADOWMAP":"",t.shadowMapEnabled?"#define "+l:"",t.premultipliedAlpha?"#define PREMULTIPLIED_ALPHA":"",t.numLightProbes>0?"#define USE_LIGHT_PROBES":"",t.numLightProbeGrids>0?"#define USE_LIGHT_PROBES_GRID":"",t.decodeVideoTexture?"#define DECODE_VIDEO_TEXTURE":"",t.decodeVideoTextureEmissive?"#define DECODE_VIDEO_TEXTURE_EMISSIVE":"",t.logarithmicDepthBuffer?"#define USE_LOGARITHMIC_DEPTH_BUFFER":"",t.reversedDepthBuffer?"#define USE_REVERSED_DEPTH_BUFFER":"","uniform mat4 viewMatrix;","uniform vec3 cameraPosition;","uniform bool isOrthographic;",t.toneMapping!==ii?"#define TONE_MAPPING":"",t.toneMapping!==ii?ht.tonemapping_pars_fragment:"",t.toneMapping!==ii?H_("toneMapping",t.toneMapping):"",t.dithering?"#define DITHERING":"",t.opaque?"#define OPAQUE":"",ht.colorspace_pars_fragment,z_("linearToOutputTexel",t.outputColorSpace),V_(),t.useDepthPacking?"#define DEPTH_PACKING "+t.depthPacking:"",`
`].filter(Yo).join(`
`)),o=kh(o),o=Ef(o,t),o=Tf(o,t),a=kh(a),a=Ef(a,t),a=Tf(a,t),o=Af(o),a=Af(a),t.isRawShaderMaterial!==!0&&(b=`#version 300 es
`,m=[f,"#define attribute in","#define varying out","#define texture2D texture"].join(`
`)+`
`+m,p=["#define varying in",t.glslVersion===Sh?"":"layout(location = 0) out highp vec4 pc_fragColor;",t.glslVersion===Sh?"":"#define gl_FragColor pc_fragColor","#define gl_FragDepthEXT gl_FragDepth","#define texture2D texture","#define textureCube texture","#define texture2DProj textureProj","#define texture2DLodEXT textureLod","#define texture2DProjLodEXT textureProjLod","#define textureCubeLodEXT textureLod","#define texture2DGradEXT textureGrad","#define texture2DProjGradEXT textureProjGrad","#define textureCubeGradEXT textureGrad"].join(`
`)+`
`+p);let S=b+m+o,x=b+p+a,A=bf(i,i.VERTEX_SHADER,S),T=bf(i,i.FRAGMENT_SHADER,x);i.attachShader(y,A),i.attachShader(y,T),t.index0AttributeName!==void 0?i.bindAttribLocation(y,0,t.index0AttributeName):t.hasPositionAttribute===!0&&i.bindAttribLocation(y,0,"position"),i.linkProgram(y);function I(R){if(s.debug.checkShaderErrors){let L=i.getProgramInfoLog(y)||"",V=i.getShaderInfoLog(A)||"",C=i.getShaderInfoLog(T)||"",N=L.trim(),U=V.trim(),E=C.trim(),G=!0,Y=!0;if(i.getProgramParameter(y,i.LINK_STATUS)===!1)if(G=!1,typeof s.debug.onShaderError=="function")s.debug.onShaderError(i,y,A,T);else{let X=wf(i,A,"vertex"),ie=wf(i,T,"fragment");$e("WebGLProgram: Shader Error "+i.getError()+" - VALIDATE_STATUS "+i.getProgramParameter(y,i.VALIDATE_STATUS)+`

Material Name: `+R.name+`
Material Type: `+R.type+`

Program Info Log: `+N+`
`+X+`
`+ie)}else N!==""?We("WebGLProgram: Program Info Log:",N):(U===""||E==="")&&(Y=!1);Y&&(R.diagnostics={runnable:G,programLog:N,vertexShader:{log:U,prefix:m},fragmentShader:{log:E,prefix:p}})}i.deleteShader(A),i.deleteShader(T),v=new Lr(i,y),D=X_(i,y)}let v;this.getUniforms=function(){return v===void 0&&I(this),v};let D;this.getAttributes=function(){return D===void 0&&I(this),D};let w=t.rendererExtensionParallelShaderCompile===!1;return this.isReady=function(){return w===!1&&(w=i.getProgramParameter(y,U_)),w},this.destroy=function(){n.releaseStatesOfProgram(this),i.deleteProgram(y),this.program=void 0},this.type=t.shaderType,this.name=t.shaderName,this.id=F_++,this.cacheKey=e,this.usedTimes=1,this.program=y,this.vertexShader=A,this.fragmentShader=T,this}var av=0,Hh=class{constructor(){this.shaderCache=new Map,this.materialCache=new Map}update(e,t,n){let i=this._getShaderCacheForMaterial(e);return i.has(t)===!1&&(i.add(t),t.usedTimes++),i.has(n)===!1&&(i.add(n),n.usedTimes++),this}remove(e){let t=this.materialCache.get(e);for(let n of t)n.usedTimes--,n.usedTimes===0&&this.shaderCache.delete(n.code);return this.materialCache.delete(e),this}getVertexShaderStage(e){return this._getShaderStage(e.vertexShader)}getFragmentShaderStage(e){return this._getShaderStage(e.fragmentShader)}dispose(){this.shaderCache.clear(),this.materialCache.clear()}_getShaderCacheForMaterial(e){let t=this.materialCache,n=t.get(e);return n===void 0&&(n=new Set,t.set(e,n)),n}_getShaderStage(e){let t=this.shaderCache,n=t.get(e);return n===void 0&&(n=new Vh(e),t.set(e,n)),n}},Vh=class{constructor(e){this.id=av++,this.code=e,this.usedTimes=0}};function lv(s){return s===cs||s===Ho||s===Vo}function cv(s,e,t,n,i,r){let o=new dr,a=new Hh,l=new Set,c=[],h=new Map,u=n.logarithmicDepthBuffer,d=n.precision,f={MeshDepthMaterial:"depth",MeshDistanceMaterial:"distance",MeshNormalMaterial:"normal",MeshBasicMaterial:"basic",MeshLambertMaterial:"lambert",MeshPhongMaterial:"phong",MeshToonMaterial:"toon",MeshStandardMaterial:"physical",MeshPhysicalMaterial:"physical",MeshMatcapMaterial:"matcap",LineBasicMaterial:"basic",LineDashedMaterial:"dashed",PointsMaterial:"points",ShadowMaterial:"shadow",SpriteMaterial:"sprite"};function g(v){return l.add(v),v===0?"uv":`uv${v}`}function y(v,D,w,R,L,V){let C=R.fog,N=L.geometry,U=v.isMeshStandardMaterial||v.isMeshLambertMaterial||v.isMeshPhongMaterial?R.environment:null,E=v.isMeshStandardMaterial||v.isMeshLambertMaterial&&!v.envMap||v.isMeshPhongMaterial&&!v.envMap,G=e.get(v.envMap||U,E),Y=G&&G.mapping===Fo?G.image.height:null,X=f[v.type];v.precision!==null&&(d=n.getMaxPrecision(v.precision),d!==v.precision&&We("WebGLProgram.getParameters:",v.precision,"not supported, using",d,"instead."));let ie=N.morphAttributes.position||N.morphAttributes.normal||N.morphAttributes.color,ce=ie!==void 0?ie.length:0,be=0;N.morphAttributes.position!==void 0&&(be=1),N.morphAttributes.normal!==void 0&&(be=2),N.morphAttributes.color!==void 0&&(be=3);let Le,Re,oe,ge;if(X){let ze=Mi[X];Le=ze.vertexShader,Re=ze.fragmentShader}else{Le=v.vertexShader,Re=v.fragmentShader;let ze=a.getVertexShaderStage(v),Bt=a.getFragmentShaderStage(v);a.update(v,ze,Bt),oe=ze.id,ge=Bt.id}let ae=s.getRenderTarget(),Se=s.state.buffers.depth.getReversed(),he=L.isInstancedMesh===!0,ue=L.isBatchedMesh===!0,H=!!v.map,Z=!!v.matcap,W=!!G,J=!!v.aoMap,te=!!v.lightMap,pe=!!v.bumpMap&&v.wireframe===!1,we=!!v.normalMap,Pe=!!v.displacementMap,je=!!v.emissiveMap,Xe=!!v.metalnessMap,Je=!!v.roughnessMap,q=v.anisotropy>0,it=v.clearcoat>0,Ue=v.dispersion>0,M=v.iridescence>0,_=v.sheen>0,O=v.transmission>0,k=q&&!!v.anisotropyMap,P=it&&!!v.clearcoatMap,z=it&&!!v.clearcoatNormalMap,$=it&&!!v.clearcoatRoughnessMap,j=M&&!!v.iridescenceMap,ne=M&&!!v.iridescenceThicknessMap,de=_&&!!v.sheenColorMap,Ce=_&&!!v.sheenRoughnessMap,Me=!!v.specularMap,ve=!!v.specularColorMap,Be=!!v.specularIntensityMap,Ee=O&&!!v.transmissionMap,Ve=O&&!!v.thicknessMap,K=!!v.gradientMap,ye=!!v.alphaMap,fe=v.alphaTest>0,Ae=!!v.alphaHash,Te=!!v.extensions,me=ii;v.toneMapped&&(ae===null||ae.isXRRenderTarget===!0)&&(me=s.toneMapping);let Ge={shaderID:X,shaderType:v.type,shaderName:v.name,vertexShader:Le,fragmentShader:Re,defines:v.defines,customVertexShaderID:oe,customFragmentShaderID:ge,isRawShaderMaterial:v.isRawShaderMaterial===!0,glslVersion:v.glslVersion,precision:d,batching:ue,batchingColor:ue&&L._colorsTexture!==null,instancing:he,instancingColor:he&&L.instanceColor!==null,instancingMorph:he&&L.morphTexture!==null,outputColorSpace:ae===null?s.outputColorSpace:ae.isXRRenderTarget===!0?ae.texture.colorSpace:ot.workingColorSpace,alphaToCoverage:!!v.alphaToCoverage,map:H,matcap:Z,envMap:W,envMapMode:W&&G.mapping,envMapCubeUVHeight:Y,aoMap:J,lightMap:te,bumpMap:pe,normalMap:we,displacementMap:Pe,emissiveMap:je,normalMapObjectSpace:we&&v.normalMapType===Jd,normalMapTangentSpace:we&&v.normalMapType===Wo,packedNormalMap:we&&v.normalMapType===Wo&&lv(v.normalMap.format),metalnessMap:Xe,roughnessMap:Je,anisotropy:q,anisotropyMap:k,clearcoat:it,clearcoatMap:P,clearcoatNormalMap:z,clearcoatRoughnessMap:$,dispersion:Ue,iridescence:M,iridescenceMap:j,iridescenceThicknessMap:ne,sheen:_,sheenColorMap:de,sheenRoughnessMap:Ce,specularMap:Me,specularColorMap:ve,specularIntensityMap:Be,transmission:O,transmissionMap:Ee,thicknessMap:Ve,gradientMap:K,opaque:v.transparent===!1&&v.blending===Ss&&v.alphaToCoverage===!1,alphaMap:ye,alphaTest:fe,alphaHash:Ae,combine:v.combine,mapUv:H&&g(v.map.channel),aoMapUv:J&&g(v.aoMap.channel),lightMapUv:te&&g(v.lightMap.channel),bumpMapUv:pe&&g(v.bumpMap.channel),normalMapUv:we&&g(v.normalMap.channel),displacementMapUv:Pe&&g(v.displacementMap.channel),emissiveMapUv:je&&g(v.emissiveMap.channel),metalnessMapUv:Xe&&g(v.metalnessMap.channel),roughnessMapUv:Je&&g(v.roughnessMap.channel),anisotropyMapUv:k&&g(v.anisotropyMap.channel),clearcoatMapUv:P&&g(v.clearcoatMap.channel),clearcoatNormalMapUv:z&&g(v.clearcoatNormalMap.channel),clearcoatRoughnessMapUv:$&&g(v.clearcoatRoughnessMap.channel),iridescenceMapUv:j&&g(v.iridescenceMap.channel),iridescenceThicknessMapUv:ne&&g(v.iridescenceThicknessMap.channel),sheenColorMapUv:de&&g(v.sheenColorMap.channel),sheenRoughnessMapUv:Ce&&g(v.sheenRoughnessMap.channel),specularMapUv:Me&&g(v.specularMap.channel),specularColorMapUv:ve&&g(v.specularColorMap.channel),specularIntensityMapUv:Be&&g(v.specularIntensityMap.channel),transmissionMapUv:Ee&&g(v.transmissionMap.channel),thicknessMapUv:Ve&&g(v.thicknessMap.channel),alphaMapUv:ye&&g(v.alphaMap.channel),vertexTangents:!!N.attributes.tangent&&(we||q),vertexNormals:!!N.attributes.normal,vertexColors:v.vertexColors,vertexAlphas:v.vertexColors===!0&&!!N.attributes.color&&N.attributes.color.itemSize===4,pointsUvs:L.isPoints===!0&&!!N.attributes.uv&&(H||ye),fog:!!C,useFog:v.fog===!0,fogExp2:!!C&&C.isFogExp2,flatShading:v.wireframe===!1&&(v.flatShading===!0||N.attributes.normal===void 0&&we===!1&&(v.isMeshLambertMaterial||v.isMeshPhongMaterial||v.isMeshStandardMaterial||v.isMeshPhysicalMaterial)),sizeAttenuation:v.sizeAttenuation===!0,logarithmicDepthBuffer:u,reversedDepthBuffer:Se,skinning:L.isSkinnedMesh===!0,hasPositionAttribute:N.attributes.position!==void 0,morphTargets:N.morphAttributes.position!==void 0,morphNormals:N.morphAttributes.normal!==void 0,morphColors:N.morphAttributes.color!==void 0,morphTargetsCount:ce,morphTextureStride:be,numDirLights:D.directional.length,numPointLights:D.point.length,numSpotLights:D.spot.length,numSpotLightMaps:D.spotLightMap.length,numRectAreaLights:D.rectArea.length,numHemiLights:D.hemi.length,numDirLightShadows:D.directionalShadowMap.length,numPointLightShadows:D.pointShadowMap.length,numSpotLightShadows:D.spotShadowMap.length,numSpotLightShadowsWithMaps:D.numSpotLightShadowsWithMaps,numLightProbes:D.numLightProbes,numLightProbeGrids:V.length,numClippingPlanes:r.numPlanes,numClipIntersection:r.numIntersection,dithering:v.dithering,shadowMapEnabled:s.shadowMap.enabled&&w.length>0,shadowMapType:s.shadowMap.type,toneMapping:me,decodeVideoTexture:H&&v.map.isVideoTexture===!0&&ot.getTransfer(v.map.colorSpace)===Mt,decodeVideoTextureEmissive:je&&v.emissiveMap.isVideoTexture===!0&&ot.getTransfer(v.emissiveMap.colorSpace)===Mt,premultipliedAlpha:v.premultipliedAlpha,doubleSided:v.side===Nt,flipSided:v.side===tn,useDepthPacking:v.depthPacking>=0,depthPacking:v.depthPacking||0,index0AttributeName:v.index0AttributeName,extensionClipCullDistance:Te&&v.extensions.clipCullDistance===!0&&t.has("WEBGL_clip_cull_distance"),extensionMultiDraw:(Te&&v.extensions.multiDraw===!0||ue)&&t.has("WEBGL_multi_draw"),rendererExtensionParallelShaderCompile:t.has("KHR_parallel_shader_compile"),customProgramCacheKey:v.customProgramCacheKey()};return Ge.vertexUv1s=l.has(1),Ge.vertexUv2s=l.has(2),Ge.vertexUv3s=l.has(3),l.clear(),Ge}function m(v){let D=[];if(v.shaderID?D.push(v.shaderID):(D.push(v.customVertexShaderID),D.push(v.customFragmentShaderID)),v.defines!==void 0)for(let w in v.defines)D.push(w),D.push(v.defines[w]);return v.isRawShaderMaterial===!1&&(p(D,v),b(D,v),D.push(s.outputColorSpace)),D.push(v.customProgramCacheKey),D.join()}function p(v,D){v.push(D.precision),v.push(D.outputColorSpace),v.push(D.envMapMode),v.push(D.envMapCubeUVHeight),v.push(D.mapUv),v.push(D.alphaMapUv),v.push(D.lightMapUv),v.push(D.aoMapUv),v.push(D.bumpMapUv),v.push(D.normalMapUv),v.push(D.displacementMapUv),v.push(D.emissiveMapUv),v.push(D.metalnessMapUv),v.push(D.roughnessMapUv),v.push(D.anisotropyMapUv),v.push(D.clearcoatMapUv),v.push(D.clearcoatNormalMapUv),v.push(D.clearcoatRoughnessMapUv),v.push(D.iridescenceMapUv),v.push(D.iridescenceThicknessMapUv),v.push(D.sheenColorMapUv),v.push(D.sheenRoughnessMapUv),v.push(D.specularMapUv),v.push(D.specularColorMapUv),v.push(D.specularIntensityMapUv),v.push(D.transmissionMapUv),v.push(D.thicknessMapUv),v.push(D.combine),v.push(D.fogExp2),v.push(D.sizeAttenuation),v.push(D.morphTargetsCount),v.push(D.morphAttributeCount),v.push(D.numDirLights),v.push(D.numPointLights),v.push(D.numSpotLights),v.push(D.numSpotLightMaps),v.push(D.numHemiLights),v.push(D.numRectAreaLights),v.push(D.numDirLightShadows),v.push(D.numPointLightShadows),v.push(D.numSpotLightShadows),v.push(D.numSpotLightShadowsWithMaps),v.push(D.numLightProbes),v.push(D.shadowMapType),v.push(D.toneMapping),v.push(D.numClippingPlanes),v.push(D.numClipIntersection),v.push(D.depthPacking)}function b(v,D){o.disableAll(),D.instancing&&o.enable(0),D.instancingColor&&o.enable(1),D.instancingMorph&&o.enable(2),D.matcap&&o.enable(3),D.envMap&&o.enable(4),D.normalMapObjectSpace&&o.enable(5),D.normalMapTangentSpace&&o.enable(6),D.clearcoat&&o.enable(7),D.iridescence&&o.enable(8),D.alphaTest&&o.enable(9),D.vertexColors&&o.enable(10),D.vertexAlphas&&o.enable(11),D.vertexUv1s&&o.enable(12),D.vertexUv2s&&o.enable(13),D.vertexUv3s&&o.enable(14),D.vertexTangents&&o.enable(15),D.anisotropy&&o.enable(16),D.alphaHash&&o.enable(17),D.batching&&o.enable(18),D.dispersion&&o.enable(19),D.batchingColor&&o.enable(20),D.gradientMap&&o.enable(21),D.packedNormalMap&&o.enable(22),D.vertexNormals&&o.enable(23),v.push(o.mask),o.disableAll(),D.fog&&o.enable(0),D.useFog&&o.enable(1),D.flatShading&&o.enable(2),D.logarithmicDepthBuffer&&o.enable(3),D.reversedDepthBuffer&&o.enable(4),D.skinning&&o.enable(5),D.morphTargets&&o.enable(6),D.morphNormals&&o.enable(7),D.morphColors&&o.enable(8),D.premultipliedAlpha&&o.enable(9),D.shadowMapEnabled&&o.enable(10),D.doubleSided&&o.enable(11),D.flipSided&&o.enable(12),D.useDepthPacking&&o.enable(13),D.dithering&&o.enable(14),D.transmission&&o.enable(15),D.sheen&&o.enable(16),D.opaque&&o.enable(17),D.pointsUvs&&o.enable(18),D.decodeVideoTexture&&o.enable(19),D.decodeVideoTextureEmissive&&o.enable(20),D.alphaToCoverage&&o.enable(21),D.numLightProbeGrids>0&&o.enable(22),D.hasPositionAttribute&&o.enable(23),v.push(o.mask)}function S(v){let D=f[v.type],w;if(D){let R=Mi[D];w=oi.clone(R.uniforms)}else w=v.uniforms;return w}function x(v,D){let w=h.get(D);return w!==void 0?++w.usedTimes:(w=new ov(s,D,v,i),c.push(w),h.set(D,w)),w}function A(v){if(--v.usedTimes===0){let D=c.indexOf(v);c[D]=c[c.length-1],c.pop(),h.delete(v.cacheKey),v.destroy()}}function T(v){a.remove(v)}function I(){a.dispose()}return{getParameters:y,getProgramCacheKey:m,getUniforms:S,acquireProgram:x,releaseProgram:A,releaseShaderCache:T,programs:c,dispose:I}}function hv(){let s=new WeakMap;function e(o){return s.has(o)}function t(o){let a=s.get(o);return a===void 0&&(a={},s.set(o,a)),a}function n(o){s.delete(o)}function i(o,a,l){s.get(o)[a]=l}function r(){s=new WeakMap}return{has:e,get:t,remove:n,update:i,dispose:r}}function uv(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.material.id!==e.material.id?s.material.id-e.material.id:s.materialVariant!==e.materialVariant?s.materialVariant-e.materialVariant:s.z!==e.z?s.z-e.z:s.id-e.id}function Cf(s,e){return s.groupOrder!==e.groupOrder?s.groupOrder-e.groupOrder:s.renderOrder!==e.renderOrder?s.renderOrder-e.renderOrder:s.z!==e.z?e.z-s.z:s.id-e.id}function Pf(){let s=[],e=0,t=[],n=[],i=[];function r(){e=0,t.length=0,n.length=0,i.length=0}function o(d){let f=0;return d.isInstancedMesh&&(f+=2),d.isSkinnedMesh&&(f+=1),f}function a(d,f,g,y,m,p){let b=s[e];return b===void 0?(b={id:d.id,object:d,geometry:f,material:g,materialVariant:o(d),groupOrder:y,renderOrder:d.renderOrder,z:m,group:p},s[e]=b):(b.id=d.id,b.object=d,b.geometry=f,b.material=g,b.materialVariant=o(d),b.groupOrder=y,b.renderOrder=d.renderOrder,b.z=m,b.group=p),e++,b}function l(d,f,g,y,m,p){let b=a(d,f,g,y,m,p);g.transmission>0?n.push(b):g.transparent===!0?i.push(b):t.push(b)}function c(d,f,g,y,m,p){let b=a(d,f,g,y,m,p);g.transmission>0?n.unshift(b):g.transparent===!0?i.unshift(b):t.unshift(b)}function h(d,f,g){t.length>1&&t.sort(d||uv),n.length>1&&n.sort(f||Cf),i.length>1&&i.sort(f||Cf),g&&(t.reverse(),n.reverse(),i.reverse())}function u(){for(let d=e,f=s.length;d<f;d++){let g=s[d];if(g.id===null)break;g.id=null,g.object=null,g.geometry=null,g.material=null,g.group=null}}return{opaque:t,transmissive:n,transparent:i,init:r,push:l,unshift:c,finish:u,sort:h}}function dv(){let s=new WeakMap;function e(n,i){let r=s.get(n),o;return r===void 0?(o=new Pf,s.set(n,[o])):i>=r.length?(o=new Pf,r.push(o)):o=r[i],o}function t(){s=new WeakMap}return{get:e,dispose:t}}function fv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={direction:new B,color:new xe};break;case"SpotLight":t={position:new B,direction:new B,color:new xe,distance:0,coneCos:0,penumbraCos:0,decay:0};break;case"PointLight":t={position:new B,color:new xe,distance:0,decay:0};break;case"HemisphereLight":t={direction:new B,skyColor:new xe,groundColor:new xe};break;case"RectAreaLight":t={color:new xe,position:new B,halfWidth:new B,halfHeight:new B};break}return s[e.id]=t,t}}}function pv(){let s={};return{get:function(e){if(s[e.id]!==void 0)return s[e.id];let t;switch(e.type){case"DirectionalLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new _e};break;case"SpotLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new _e};break;case"PointLight":t={shadowIntensity:1,shadowBias:0,shadowNormalBias:0,shadowRadius:1,shadowMapSize:new _e,shadowCameraNear:1,shadowCameraFar:1e3};break}return s[e.id]=t,t}}}var mv=0;function gv(s,e){return(e.castShadow?2:0)-(s.castShadow?2:0)+(e.map?1:0)-(s.map?1:0)}function xv(s){let e=new fv,t=pv(),n={version:0,hash:{directionalLength:-1,pointLength:-1,spotLength:-1,rectAreaLength:-1,hemiLength:-1,numDirectionalShadows:-1,numPointShadows:-1,numSpotShadows:-1,numSpotMaps:-1,numLightProbes:-1},ambient:[0,0,0],probe:[],directional:[],directionalShadow:[],directionalShadowMap:[],directionalShadowMatrix:[],spot:[],spotLightMap:[],spotShadow:[],spotShadowMap:[],spotLightMatrix:[],rectArea:[],rectAreaLTC1:null,rectAreaLTC2:null,point:[],pointShadow:[],pointShadowMap:[],pointShadowMatrix:[],hemi:[],numSpotLightShadowsWithMaps:0,numLightProbes:0};for(let c=0;c<9;c++)n.probe.push(new B);let i=new B,r=new Qe,o=new Qe;function a(c){let h=0,u=0,d=0;for(let D=0;D<9;D++)n.probe[D].set(0,0,0);let f=0,g=0,y=0,m=0,p=0,b=0,S=0,x=0,A=0,T=0,I=0;c.sort(gv);for(let D=0,w=c.length;D<w;D++){let R=c[D],L=R.color,V=R.intensity,C=R.distance,N=null;if(R.shadow&&R.shadow.map&&(R.shadow.map.texture.format===cs?N=R.shadow.map.texture:N=R.shadow.map.depthTexture||R.shadow.map.texture),R.isAmbientLight)h+=L.r*V,u+=L.g*V,d+=L.b*V;else if(R.isLightProbe){for(let U=0;U<9;U++)n.probe[U].addScaledVector(R.sh.coefficients[U],V);I++}else if(R.isDirectionalLight){let U=e.get(R);if(U.color.copy(R.color).multiplyScalar(R.intensity),R.castShadow){let E=R.shadow,G=t.get(R);G.shadowIntensity=E.intensity,G.shadowBias=E.bias,G.shadowNormalBias=E.normalBias,G.shadowRadius=E.radius,G.shadowMapSize=E.mapSize,n.directionalShadow[f]=G,n.directionalShadowMap[f]=N,n.directionalShadowMatrix[f]=R.shadow.matrix,b++}n.directional[f]=U,f++}else if(R.isSpotLight){let U=e.get(R);U.position.setFromMatrixPosition(R.matrixWorld),U.color.copy(L).multiplyScalar(V),U.distance=C,U.coneCos=Math.cos(R.angle),U.penumbraCos=Math.cos(R.angle*(1-R.penumbra)),U.decay=R.decay,n.spot[y]=U;let E=R.shadow;if(R.map&&(n.spotLightMap[A]=R.map,A++,E.updateMatrices(R),R.castShadow&&T++),n.spotLightMatrix[y]=E.matrix,R.castShadow){let G=t.get(R);G.shadowIntensity=E.intensity,G.shadowBias=E.bias,G.shadowNormalBias=E.normalBias,G.shadowRadius=E.radius,G.shadowMapSize=E.mapSize,n.spotShadow[y]=G,n.spotShadowMap[y]=N,x++}y++}else if(R.isRectAreaLight){let U=e.get(R);U.color.copy(L).multiplyScalar(V),U.halfWidth.set(R.width*.5,0,0),U.halfHeight.set(0,R.height*.5,0),n.rectArea[m]=U,m++}else if(R.isPointLight){let U=e.get(R);if(U.color.copy(R.color).multiplyScalar(R.intensity),U.distance=R.distance,U.decay=R.decay,R.castShadow){let E=R.shadow,G=t.get(R);G.shadowIntensity=E.intensity,G.shadowBias=E.bias,G.shadowNormalBias=E.normalBias,G.shadowRadius=E.radius,G.shadowMapSize=E.mapSize,G.shadowCameraNear=E.camera.near,G.shadowCameraFar=E.camera.far,n.pointShadow[g]=G,n.pointShadowMap[g]=N,n.pointShadowMatrix[g]=R.shadow.matrix,S++}n.point[g]=U,g++}else if(R.isHemisphereLight){let U=e.get(R);U.skyColor.copy(R.color).multiplyScalar(V),U.groundColor.copy(R.groundColor).multiplyScalar(V),n.hemi[p]=U,p++}}m>0&&(s.has("OES_texture_float_linear")===!0?(n.rectAreaLTC1=Ie.LTC_FLOAT_1,n.rectAreaLTC2=Ie.LTC_FLOAT_2):(n.rectAreaLTC1=Ie.LTC_HALF_1,n.rectAreaLTC2=Ie.LTC_HALF_2)),n.ambient[0]=h,n.ambient[1]=u,n.ambient[2]=d;let v=n.hash;(v.directionalLength!==f||v.pointLength!==g||v.spotLength!==y||v.rectAreaLength!==m||v.hemiLength!==p||v.numDirectionalShadows!==b||v.numPointShadows!==S||v.numSpotShadows!==x||v.numSpotMaps!==A||v.numLightProbes!==I)&&(n.directional.length=f,n.spot.length=y,n.rectArea.length=m,n.point.length=g,n.hemi.length=p,n.directionalShadow.length=b,n.directionalShadowMap.length=b,n.pointShadow.length=S,n.pointShadowMap.length=S,n.spotShadow.length=x,n.spotShadowMap.length=x,n.directionalShadowMatrix.length=b,n.pointShadowMatrix.length=S,n.spotLightMatrix.length=x+A-T,n.spotLightMap.length=A,n.numSpotLightShadowsWithMaps=T,n.numLightProbes=I,v.directionalLength=f,v.pointLength=g,v.spotLength=y,v.rectAreaLength=m,v.hemiLength=p,v.numDirectionalShadows=b,v.numPointShadows=S,v.numSpotShadows=x,v.numSpotMaps=A,v.numLightProbes=I,n.version=mv++)}function l(c,h){let u=0,d=0,f=0,g=0,y=0,m=h.matrixWorldInverse;for(let p=0,b=c.length;p<b;p++){let S=c[p];if(S.isDirectionalLight){let x=n.directional[u];x.direction.setFromMatrixPosition(S.matrixWorld),i.setFromMatrixPosition(S.target.matrixWorld),x.direction.sub(i),x.direction.transformDirection(m),u++}else if(S.isSpotLight){let x=n.spot[f];x.position.setFromMatrixPosition(S.matrixWorld),x.position.applyMatrix4(m),x.direction.setFromMatrixPosition(S.matrixWorld),i.setFromMatrixPosition(S.target.matrixWorld),x.direction.sub(i),x.direction.transformDirection(m),f++}else if(S.isRectAreaLight){let x=n.rectArea[g];x.position.setFromMatrixPosition(S.matrixWorld),x.position.applyMatrix4(m),o.identity(),r.copy(S.matrixWorld),r.premultiply(m),o.extractRotation(r),x.halfWidth.set(S.width*.5,0,0),x.halfHeight.set(0,S.height*.5,0),x.halfWidth.applyMatrix4(o),x.halfHeight.applyMatrix4(o),g++}else if(S.isPointLight){let x=n.point[d];x.position.setFromMatrixPosition(S.matrixWorld),x.position.applyMatrix4(m),d++}else if(S.isHemisphereLight){let x=n.hemi[y];x.direction.setFromMatrixPosition(S.matrixWorld),x.direction.transformDirection(m),y++}}}return{setup:a,setupView:l,state:n}}function If(s){let e=new xv(s),t=[],n=[],i=[];function r(d){u.camera=d,t.length=0,n.length=0,i.length=0}function o(d){t.push(d)}function a(d){n.push(d)}function l(d){i.push(d)}function c(){e.setup(t)}function h(d){e.setupView(t,d)}let u={lightsArray:t,shadowsArray:n,lightProbeGridArray:i,camera:null,lights:e,transmissionRenderTarget:{},textureUnits:0};return{init:r,state:u,setupLights:c,setupLightsView:h,pushLight:o,pushShadow:a,pushLightProbeGrid:l}}function _v(s){let e=new WeakMap;function t(i,r=0){let o=e.get(i),a;return o===void 0?(a=new If(s),e.set(i,[a])):r>=o.length?(a=new If(s),o.push(a)):a=o[r],a}function n(){e=new WeakMap}return{get:t,dispose:n}}var vv=`void main() {
	gl_Position = vec4( position, 1.0 );
}`,yv=`uniform sampler2D shadow_pass;
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
}`,Mv=[new B(1,0,0),new B(-1,0,0),new B(0,1,0),new B(0,-1,0),new B(0,0,1),new B(0,0,-1)],bv=[new B(0,-1,0),new B(0,-1,0),new B(0,0,1),new B(0,0,-1),new B(0,-1,0),new B(0,-1,0)],Lf=new Qe,qo=new B,Uh=new B;function Sv(s,e,t){let n=new xr,i=new _e,r=new _e,o=new _t,a=new Ja,l=new $a,c={},h=t.maxTextureSize,u={[Ln]:tn,[tn]:Ln,[Nt]:Nt},d=new mt({defines:{VSM_SAMPLES:8},uniforms:{shadow_pass:{value:null},resolution:{value:new _e},radius:{value:4}},vertexShader:vv,fragmentShader:yv}),f=d.clone();f.defines.HORIZONTAL_PASS=1;let g=new pt;g.setAttribute("position",new xt(new Float32Array([-1,-1,.5,3,-1,.5,-1,3,.5]),3));let y=new Ke(g,d),m=this;this.enabled=!1,this.autoUpdate=!0,this.needsUpdate=!1,this.type=Co;let p=this.type;this.render=function(T,I,v){if(m.enabled===!1||m.autoUpdate===!1&&m.needsUpdate===!1||T.length===0)return;this.type===cl&&(We("WebGLShadowMap: PCFSoftShadowMap has been deprecated. Using PCFShadowMap instead."),this.type=Co);let D=s.getRenderTarget(),w=s.getActiveCubeFace(),R=s.getActiveMipmapLevel(),L=s.state;L.setBlending(Wn),L.buffers.depth.getReversed()===!0?L.buffers.color.setClear(0,0,0,0):L.buffers.color.setClear(1,1,1,1),L.buffers.depth.setTest(!0),L.setScissorTest(!1);let V=p!==this.type;V&&I.traverse(function(C){C.material&&(Array.isArray(C.material)?C.material.forEach(N=>N.needsUpdate=!0):C.material.needsUpdate=!0)});for(let C=0,N=T.length;C<N;C++){let U=T[C],E=U.shadow;if(E===void 0){We("WebGLShadowMap:",U,"has no shadow.");continue}if(E.autoUpdate===!1&&E.needsUpdate===!1)continue;i.copy(E.mapSize);let G=E.getFrameExtents();i.multiply(G),r.copy(E.mapSize),(i.x>h||i.y>h)&&(i.x>h&&(r.x=Math.floor(h/G.x),i.x=r.x*G.x,E.mapSize.x=r.x),i.y>h&&(r.y=Math.floor(h/G.y),i.y=r.y*G.y,E.mapSize.y=r.y));let Y=s.state.buffers.depth.getReversed();if(E.camera._reversedDepth=Y,E.map===null||V===!0){if(E.map!==null&&(E.map.depthTexture!==null&&(E.map.depthTexture.dispose(),E.map.depthTexture=null),E.map.dispose()),this.type===Tr){if(U.isPointLight){We("WebGLShadowMap: VSM shadow maps are not supported for PointLights. Use PCF or BasicShadowMap instead.");continue}E.map=new Wt(i.x,i.y,{format:cs,type:nn,minFilter:kt,magFilter:kt,generateMipmaps:!1}),E.map.texture.name=U.name+".shadowMap",E.map.depthTexture=new Ni(i.x,i.y,Un),E.map.depthTexture.name=U.name+".shadowMapDepth",E.map.depthTexture.format=di,E.map.depthTexture.compareFunction=null,E.map.depthTexture.minFilter=Kt,E.map.depthTexture.magFilter=Kt}else U.isPointLight?(E.map=new ic(i.x),E.map.depthTexture=new Ga(i.x,ri)):(E.map=new Wt(i.x,i.y),E.map.depthTexture=new Ni(i.x,i.y,ri)),E.map.depthTexture.name=U.name+".shadowMap",E.map.depthTexture.format=di,this.type===Co?(E.map.depthTexture.compareFunction=Y?ec:Ql,E.map.depthTexture.minFilter=kt,E.map.depthTexture.magFilter=kt):(E.map.depthTexture.compareFunction=null,E.map.depthTexture.minFilter=Kt,E.map.depthTexture.magFilter=Kt);E.camera.updateProjectionMatrix()}let X=E.map.isWebGLCubeRenderTarget?6:1;for(let ie=0;ie<X;ie++){if(E.map.isWebGLCubeRenderTarget)s.setRenderTarget(E.map,ie),s.clear();else{ie===0&&(s.setRenderTarget(E.map),s.clear());let ce=E.getViewport(ie);o.set(r.x*ce.x,r.y*ce.y,r.x*ce.z,r.y*ce.w),L.viewport(o)}if(U.isPointLight){let ce=E.camera,be=E.matrix,Le=U.distance||ce.far;Le!==ce.far&&(ce.far=Le,ce.updateProjectionMatrix()),qo.setFromMatrixPosition(U.matrixWorld),ce.position.copy(qo),Uh.copy(ce.position),Uh.add(Mv[ie]),ce.up.copy(bv[ie]),ce.lookAt(Uh),ce.updateMatrixWorld(),be.makeTranslation(-qo.x,-qo.y,-qo.z),Lf.multiplyMatrices(ce.projectionMatrix,ce.matrixWorldInverse),E._frustum.setFromProjectionMatrix(Lf,ce.coordinateSystem,ce.reversedDepth)}else E.updateMatrices(U);n=E.getFrustum(),x(I,v,E.camera,U,this.type)}E.isPointLightShadow!==!0&&this.type===Tr&&b(E,v),E.needsUpdate=!1}p=this.type,m.needsUpdate=!1,s.setRenderTarget(D,w,R)};function b(T,I){let v=e.update(y);d.defines.VSM_SAMPLES!==T.blurSamples&&(d.defines.VSM_SAMPLES=T.blurSamples,f.defines.VSM_SAMPLES=T.blurSamples,d.needsUpdate=!0,f.needsUpdate=!0),T.mapPass===null&&(T.mapPass=new Wt(i.x,i.y,{format:cs,type:nn})),d.uniforms.shadow_pass.value=T.map.depthTexture,d.uniforms.resolution.value=T.mapSize,d.uniforms.radius.value=T.radius,s.setRenderTarget(T.mapPass),s.clear(),s.renderBufferDirect(I,null,v,d,y,null),f.uniforms.shadow_pass.value=T.mapPass.texture,f.uniforms.resolution.value=T.mapSize,f.uniforms.radius.value=T.radius,s.setRenderTarget(T.map),s.clear(),s.renderBufferDirect(I,null,v,f,y,null)}function S(T,I,v,D){let w=null,R=v.isPointLight===!0?T.customDistanceMaterial:T.customDepthMaterial;if(R!==void 0)w=R;else if(w=v.isPointLight===!0?l:a,s.localClippingEnabled&&I.clipShadows===!0&&Array.isArray(I.clippingPlanes)&&I.clippingPlanes.length!==0||I.displacementMap&&I.displacementScale!==0||I.alphaMap&&I.alphaTest>0||I.map&&I.alphaTest>0||I.alphaToCoverage===!0){let L=w.uuid,V=I.uuid,C=c[L];C===void 0&&(C={},c[L]=C);let N=C[V];N===void 0&&(N=w.clone(),C[V]=N,I.addEventListener("dispose",A)),w=N}if(w.visible=I.visible,w.wireframe=I.wireframe,D===Tr?w.side=I.shadowSide!==null?I.shadowSide:I.side:w.side=I.shadowSide!==null?I.shadowSide:u[I.side],w.alphaMap=I.alphaMap,w.alphaTest=I.alphaToCoverage===!0?.5:I.alphaTest,w.map=I.map,w.clipShadows=I.clipShadows,w.clippingPlanes=I.clippingPlanes,w.clipIntersection=I.clipIntersection,w.displacementMap=I.displacementMap,w.displacementScale=I.displacementScale,w.displacementBias=I.displacementBias,w.wireframeLinewidth=I.wireframeLinewidth,w.linewidth=I.linewidth,v.isPointLight===!0&&w.isMeshDistanceMaterial===!0){let L=s.properties.get(w);L.light=v}return w}function x(T,I,v,D,w){if(T.visible===!1)return;if(T.layers.test(I.layers)&&(T.isMesh||T.isLine||T.isPoints)&&(T.castShadow||T.receiveShadow&&w===Tr)&&(!T.frustumCulled||n.intersectsObject(T))){T.modelViewMatrix.multiplyMatrices(v.matrixWorldInverse,T.matrixWorld);let V=e.update(T),C=T.material;if(Array.isArray(C)){let N=V.groups;for(let U=0,E=N.length;U<E;U++){let G=N[U],Y=C[G.materialIndex];if(Y&&Y.visible){let X=S(T,Y,D,w);T.onBeforeShadow(s,T,I,v,V,X,G),s.renderBufferDirect(v,null,V,X,T,G),T.onAfterShadow(s,T,I,v,V,X,G)}}}else if(C.visible){let N=S(T,C,D,w);T.onBeforeShadow(s,T,I,v,V,N,null),s.renderBufferDirect(v,null,V,N,T,null),T.onAfterShadow(s,T,I,v,V,N,null)}}let L=T.children;for(let V=0,C=L.length;V<C;V++)x(L[V],I,v,D,w)}function A(T){T.target.removeEventListener("dispose",A);for(let v in c){let D=c[v],w=T.target.uuid;w in D&&(D[w].dispose(),delete D[w])}}}function wv(s,e){function t(){let K=!1,ye=new _t,fe=null,Ae=new _t(0,0,0,0);return{setMask:function(Te){fe!==Te&&!K&&(s.colorMask(Te,Te,Te,Te),fe=Te)},setLocked:function(Te){K=Te},setClear:function(Te,me,Ge,ze,Bt){Bt===!0&&(Te*=ze,me*=ze,Ge*=ze),ye.set(Te,me,Ge,ze),Ae.equals(ye)===!1&&(s.clearColor(Te,me,Ge,ze),Ae.copy(ye))},reset:function(){K=!1,fe=null,Ae.set(-1,0,0,0)}}}function n(){let K=!1,ye=!1,fe=null,Ae=null,Te=null;return{setReversed:function(me){if(ye!==me){let Ge=e.get("EXT_clip_control");me?Ge.clipControlEXT(Ge.LOWER_LEFT_EXT,Ge.ZERO_TO_ONE_EXT):Ge.clipControlEXT(Ge.LOWER_LEFT_EXT,Ge.NEGATIVE_ONE_TO_ONE_EXT),ye=me;let ze=Te;Te=null,this.setClear(ze)}},getReversed:function(){return ye},setTest:function(me){me?ae(s.DEPTH_TEST):Se(s.DEPTH_TEST)},setMask:function(me){fe!==me&&!K&&(s.depthMask(me),fe=me)},setFunc:function(me){if(ye&&(me=lf[me]),Ae!==me){switch(me){case Pa:s.depthFunc(s.NEVER);break;case Ia:s.depthFunc(s.ALWAYS);break;case La:s.depthFunc(s.LESS);break;case ws:s.depthFunc(s.LEQUAL);break;case Da:s.depthFunc(s.EQUAL);break;case Na:s.depthFunc(s.GEQUAL);break;case Ua:s.depthFunc(s.GREATER);break;case Fa:s.depthFunc(s.NOTEQUAL);break;default:s.depthFunc(s.LEQUAL)}Ae=me}},setLocked:function(me){K=me},setClear:function(me){Te!==me&&(Te=me,ye&&(me=1-me),s.clearDepth(me))},reset:function(){K=!1,fe=null,Ae=null,Te=null,ye=!1}}}function i(){let K=!1,ye=null,fe=null,Ae=null,Te=null,me=null,Ge=null,ze=null,Bt=null;return{setTest:function(Tt){K||(Tt?ae(s.STENCIL_TEST):Se(s.STENCIL_TEST))},setMask:function(Tt){ye!==Tt&&!K&&(s.stencilMask(Tt),ye=Tt)},setFunc:function(Tt,An,Rn){(fe!==Tt||Ae!==An||Te!==Rn)&&(s.stencilFunc(Tt,An,Rn),fe=Tt,Ae=An,Te=Rn)},setOp:function(Tt,An,Rn){(me!==Tt||Ge!==An||ze!==Rn)&&(s.stencilOp(Tt,An,Rn),me=Tt,Ge=An,ze=Rn)},setLocked:function(Tt){K=Tt},setClear:function(Tt){Bt!==Tt&&(s.clearStencil(Tt),Bt=Tt)},reset:function(){K=!1,ye=null,fe=null,Ae=null,Te=null,me=null,Ge=null,ze=null,Bt=null}}}let r=new t,o=new n,a=new i,l=new WeakMap,c=new WeakMap,h={},u={},d={},f=new WeakMap,g=[],y=null,m=!1,p=null,b=null,S=null,x=null,A=null,T=null,I=null,v=new xe(0,0,0),D=0,w=!1,R=null,L=null,V=null,C=null,N=null,U=s.getParameter(s.MAX_COMBINED_TEXTURE_IMAGE_UNITS),E=!1,G=0,Y=s.getParameter(s.VERSION);Y.indexOf("WebGL")!==-1?(G=parseFloat(/^WebGL (\d)/.exec(Y)[1]),E=G>=1):Y.indexOf("OpenGL ES")!==-1&&(G=parseFloat(/^OpenGL ES (\d)/.exec(Y)[1]),E=G>=2);let X=null,ie={},ce=s.getParameter(s.SCISSOR_BOX),be=s.getParameter(s.VIEWPORT),Le=new _t().fromArray(ce),Re=new _t().fromArray(be);function oe(K,ye,fe,Ae){let Te=new Uint8Array(4),me=s.createTexture();s.bindTexture(K,me),s.texParameteri(K,s.TEXTURE_MIN_FILTER,s.NEAREST),s.texParameteri(K,s.TEXTURE_MAG_FILTER,s.NEAREST);for(let Ge=0;Ge<fe;Ge++)K===s.TEXTURE_3D||K===s.TEXTURE_2D_ARRAY?s.texImage3D(ye,0,s.RGBA,1,1,Ae,0,s.RGBA,s.UNSIGNED_BYTE,Te):s.texImage2D(ye+Ge,0,s.RGBA,1,1,0,s.RGBA,s.UNSIGNED_BYTE,Te);return me}let ge={};ge[s.TEXTURE_2D]=oe(s.TEXTURE_2D,s.TEXTURE_2D,1),ge[s.TEXTURE_CUBE_MAP]=oe(s.TEXTURE_CUBE_MAP,s.TEXTURE_CUBE_MAP_POSITIVE_X,6),ge[s.TEXTURE_2D_ARRAY]=oe(s.TEXTURE_2D_ARRAY,s.TEXTURE_2D_ARRAY,1,1),ge[s.TEXTURE_3D]=oe(s.TEXTURE_3D,s.TEXTURE_3D,1,1),r.setClear(0,0,0,1),o.setClear(1),a.setClear(0),ae(s.DEPTH_TEST),o.setFunc(ws),pe(!1),we(dh),ae(s.CULL_FACE),J(Wn);function ae(K){h[K]!==!0&&(s.enable(K),h[K]=!0)}function Se(K){h[K]!==!1&&(s.disable(K),h[K]=!1)}function he(K,ye){return d[K]!==ye?(s.bindFramebuffer(K,ye),d[K]=ye,K===s.DRAW_FRAMEBUFFER&&(d[s.FRAMEBUFFER]=ye),K===s.FRAMEBUFFER&&(d[s.DRAW_FRAMEBUFFER]=ye),!0):!1}function ue(K,ye){let fe=g,Ae=!1;if(K){fe=f.get(ye),fe===void 0&&(fe=[],f.set(ye,fe));let Te=K.textures;if(fe.length!==Te.length||fe[0]!==s.COLOR_ATTACHMENT0){for(let me=0,Ge=Te.length;me<Ge;me++)fe[me]=s.COLOR_ATTACHMENT0+me;fe.length=Te.length,Ae=!0}}else fe[0]!==s.BACK&&(fe[0]=s.BACK,Ae=!0);Ae&&s.drawBuffers(fe)}function H(K){return y!==K?(s.useProgram(K),y=K,!0):!1}let Z={[Qi]:s.FUNC_ADD,[Ad]:s.FUNC_SUBTRACT,[Rd]:s.FUNC_REVERSE_SUBTRACT};Z[Cd]=s.MIN,Z[Pd]=s.MAX;let W={[Id]:s.ZERO,[Ld]:s.ONE,[Dd]:s.SRC_COLOR,[Ra]:s.SRC_ALPHA,[zd]:s.SRC_ALPHA_SATURATE,[Od]:s.DST_COLOR,[Ud]:s.DST_ALPHA,[Nd]:s.ONE_MINUS_SRC_COLOR,[Ca]:s.ONE_MINUS_SRC_ALPHA,[Bd]:s.ONE_MINUS_DST_COLOR,[Fd]:s.ONE_MINUS_DST_ALPHA,[kd]:s.CONSTANT_COLOR,[Hd]:s.ONE_MINUS_CONSTANT_COLOR,[Vd]:s.CONSTANT_ALPHA,[Gd]:s.ONE_MINUS_CONSTANT_ALPHA};function J(K,ye,fe,Ae,Te,me,Ge,ze,Bt,Tt){if(K===Wn){m===!0&&(Se(s.BLEND),m=!1);return}if(m===!1&&(ae(s.BLEND),m=!0),K!==Td){if(K!==p||Tt!==w){if((b!==Qi||A!==Qi)&&(s.blendEquation(s.FUNC_ADD),b=Qi,A=Qi),Tt)switch(K){case Ss:s.blendFuncSeparate(s.ONE,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case Xt:s.blendFunc(s.ONE,s.ONE);break;case fh:s.blendFuncSeparate(s.ZERO,s.ONE_MINUS_SRC_COLOR,s.ZERO,s.ONE);break;case ph:s.blendFuncSeparate(s.DST_COLOR,s.ONE_MINUS_SRC_ALPHA,s.ZERO,s.ONE);break;default:$e("WebGLState: Invalid blending: ",K);break}else switch(K){case Ss:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE_MINUS_SRC_ALPHA,s.ONE,s.ONE_MINUS_SRC_ALPHA);break;case Xt:s.blendFuncSeparate(s.SRC_ALPHA,s.ONE,s.ONE,s.ONE);break;case fh:$e("WebGLState: SubtractiveBlending requires material.premultipliedAlpha = true");break;case ph:$e("WebGLState: MultiplyBlending requires material.premultipliedAlpha = true");break;default:$e("WebGLState: Invalid blending: ",K);break}S=null,x=null,T=null,I=null,v.set(0,0,0),D=0,p=K,w=Tt}return}Te=Te||ye,me=me||fe,Ge=Ge||Ae,(ye!==b||Te!==A)&&(s.blendEquationSeparate(Z[ye],Z[Te]),b=ye,A=Te),(fe!==S||Ae!==x||me!==T||Ge!==I)&&(s.blendFuncSeparate(W[fe],W[Ae],W[me],W[Ge]),S=fe,x=Ae,T=me,I=Ge),(ze.equals(v)===!1||Bt!==D)&&(s.blendColor(ze.r,ze.g,ze.b,Bt),v.copy(ze),D=Bt),p=K,w=!1}function te(K,ye){K.side===Nt?Se(s.CULL_FACE):ae(s.CULL_FACE);let fe=K.side===tn;ye&&(fe=!fe),pe(fe),K.blending===Ss&&K.transparent===!1?J(Wn):J(K.blending,K.blendEquation,K.blendSrc,K.blendDst,K.blendEquationAlpha,K.blendSrcAlpha,K.blendDstAlpha,K.blendColor,K.blendAlpha,K.premultipliedAlpha),o.setFunc(K.depthFunc),o.setTest(K.depthTest),o.setMask(K.depthWrite),r.setMask(K.colorWrite);let Ae=K.stencilWrite;a.setTest(Ae),Ae&&(a.setMask(K.stencilWriteMask),a.setFunc(K.stencilFunc,K.stencilRef,K.stencilFuncMask),a.setOp(K.stencilFail,K.stencilZFail,K.stencilZPass)),je(K.polygonOffset,K.polygonOffsetFactor,K.polygonOffsetUnits),K.alphaToCoverage===!0?ae(s.SAMPLE_ALPHA_TO_COVERAGE):Se(s.SAMPLE_ALPHA_TO_COVERAGE)}function pe(K){R!==K&&(K?s.frontFace(s.CW):s.frontFace(s.CCW),R=K)}function we(K){K!==wd?(ae(s.CULL_FACE),K!==L&&(K===dh?s.cullFace(s.BACK):K===Ed?s.cullFace(s.FRONT):s.cullFace(s.FRONT_AND_BACK))):Se(s.CULL_FACE),L=K}function Pe(K){K!==V&&(E&&s.lineWidth(K),V=K)}function je(K,ye,fe){K?(ae(s.POLYGON_OFFSET_FILL),(C!==ye||N!==fe)&&(C=ye,N=fe,o.getReversed()&&(ye=-ye),s.polygonOffset(ye,fe))):Se(s.POLYGON_OFFSET_FILL)}function Xe(K){K?ae(s.SCISSOR_TEST):Se(s.SCISSOR_TEST)}function Je(K){K===void 0&&(K=s.TEXTURE0+U-1),X!==K&&(s.activeTexture(K),X=K)}function q(K,ye,fe){fe===void 0&&(X===null?fe=s.TEXTURE0+U-1:fe=X);let Ae=ie[fe];Ae===void 0&&(Ae={type:void 0,texture:void 0},ie[fe]=Ae),(Ae.type!==K||Ae.texture!==ye)&&(X!==fe&&(s.activeTexture(fe),X=fe),s.bindTexture(K,ye||ge[K]),Ae.type=K,Ae.texture=ye)}function it(){let K=ie[X];K!==void 0&&K.type!==void 0&&(s.bindTexture(K.type,null),K.type=void 0,K.texture=void 0)}function Ue(){try{s.compressedTexImage2D(...arguments)}catch(K){$e("WebGLState:",K)}}function M(){try{s.compressedTexImage3D(...arguments)}catch(K){$e("WebGLState:",K)}}function _(){try{s.texSubImage2D(...arguments)}catch(K){$e("WebGLState:",K)}}function O(){try{s.texSubImage3D(...arguments)}catch(K){$e("WebGLState:",K)}}function k(){try{s.compressedTexSubImage2D(...arguments)}catch(K){$e("WebGLState:",K)}}function P(){try{s.compressedTexSubImage3D(...arguments)}catch(K){$e("WebGLState:",K)}}function z(){try{s.texStorage2D(...arguments)}catch(K){$e("WebGLState:",K)}}function $(){try{s.texStorage3D(...arguments)}catch(K){$e("WebGLState:",K)}}function j(){try{s.texImage2D(...arguments)}catch(K){$e("WebGLState:",K)}}function ne(){try{s.texImage3D(...arguments)}catch(K){$e("WebGLState:",K)}}function de(K){return u[K]!==void 0?u[K]:s.getParameter(K)}function Ce(K,ye){u[K]!==ye&&(s.pixelStorei(K,ye),u[K]=ye)}function Me(K){Le.equals(K)===!1&&(s.scissor(K.x,K.y,K.z,K.w),Le.copy(K))}function ve(K){Re.equals(K)===!1&&(s.viewport(K.x,K.y,K.z,K.w),Re.copy(K))}function Be(K,ye){let fe=c.get(ye);fe===void 0&&(fe=new WeakMap,c.set(ye,fe));let Ae=fe.get(K);Ae===void 0&&(Ae=s.getUniformBlockIndex(ye,K.name),fe.set(K,Ae))}function Ee(K,ye){let Ae=c.get(ye).get(K);l.get(ye)!==Ae&&(s.uniformBlockBinding(ye,Ae,K.__bindingPointIndex),l.set(ye,Ae))}function Ve(){s.disable(s.BLEND),s.disable(s.CULL_FACE),s.disable(s.DEPTH_TEST),s.disable(s.POLYGON_OFFSET_FILL),s.disable(s.SCISSOR_TEST),s.disable(s.STENCIL_TEST),s.disable(s.SAMPLE_ALPHA_TO_COVERAGE),s.blendEquation(s.FUNC_ADD),s.blendFunc(s.ONE,s.ZERO),s.blendFuncSeparate(s.ONE,s.ZERO,s.ONE,s.ZERO),s.blendColor(0,0,0,0),s.colorMask(!0,!0,!0,!0),s.clearColor(0,0,0,0),s.depthMask(!0),s.depthFunc(s.LESS),o.setReversed(!1),s.clearDepth(1),s.stencilMask(4294967295),s.stencilFunc(s.ALWAYS,0,4294967295),s.stencilOp(s.KEEP,s.KEEP,s.KEEP),s.clearStencil(0),s.cullFace(s.BACK),s.frontFace(s.CCW),s.polygonOffset(0,0),s.activeTexture(s.TEXTURE0),s.bindFramebuffer(s.FRAMEBUFFER,null),s.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),s.bindFramebuffer(s.READ_FRAMEBUFFER,null),s.useProgram(null),s.lineWidth(1),s.scissor(0,0,s.canvas.width,s.canvas.height),s.viewport(0,0,s.canvas.width,s.canvas.height),s.pixelStorei(s.PACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_ALIGNMENT,4),s.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,!1),s.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,!1),s.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,s.BROWSER_DEFAULT_WEBGL),s.pixelStorei(s.PACK_ROW_LENGTH,0),s.pixelStorei(s.PACK_SKIP_PIXELS,0),s.pixelStorei(s.PACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_ROW_LENGTH,0),s.pixelStorei(s.UNPACK_IMAGE_HEIGHT,0),s.pixelStorei(s.UNPACK_SKIP_PIXELS,0),s.pixelStorei(s.UNPACK_SKIP_ROWS,0),s.pixelStorei(s.UNPACK_SKIP_IMAGES,0),h={},u={},X=null,ie={},d={},f=new WeakMap,g=[],y=null,m=!1,p=null,b=null,S=null,x=null,A=null,T=null,I=null,v=new xe(0,0,0),D=0,w=!1,R=null,L=null,V=null,C=null,N=null,Le.set(0,0,s.canvas.width,s.canvas.height),Re.set(0,0,s.canvas.width,s.canvas.height),r.reset(),o.reset(),a.reset()}return{buffers:{color:r,depth:o,stencil:a},enable:ae,disable:Se,bindFramebuffer:he,drawBuffers:ue,useProgram:H,setBlending:J,setMaterial:te,setFlipSided:pe,setCullFace:we,setLineWidth:Pe,setPolygonOffset:je,setScissorTest:Xe,activeTexture:Je,bindTexture:q,unbindTexture:it,compressedTexImage2D:Ue,compressedTexImage3D:M,texImage2D:j,texImage3D:ne,pixelStorei:Ce,getParameter:de,updateUBOMapping:Be,uniformBlockBinding:Ee,texStorage2D:z,texStorage3D:$,texSubImage2D:_,texSubImage3D:O,compressedTexSubImage2D:k,compressedTexSubImage3D:P,scissor:Me,viewport:ve,reset:Ve}}function Ev(s,e,t,n,i,r,o){let a=e.has("WEBGL_multisampled_render_to_texture")?e.get("WEBGL_multisampled_render_to_texture"):null,l=typeof navigator>"u"?!1:/OculusBrowser/g.test(navigator.userAgent),c=new _e,h=new WeakMap,u=new Set,d,f=new WeakMap,g=!1;try{g=typeof OffscreenCanvas<"u"&&new OffscreenCanvas(1,1).getContext("2d")!==null}catch{}function y(M,_){return g?new OffscreenCanvas(M,_):cr("canvas")}function m(M,_,O){let k=1,P=Ue(M);if((P.width>O||P.height>O)&&(k=O/Math.max(P.width,P.height)),k<1)if(typeof HTMLImageElement<"u"&&M instanceof HTMLImageElement||typeof HTMLCanvasElement<"u"&&M instanceof HTMLCanvasElement||typeof ImageBitmap<"u"&&M instanceof ImageBitmap||typeof VideoFrame<"u"&&M instanceof VideoFrame){let z=Math.floor(k*P.width),$=Math.floor(k*P.height);d===void 0&&(d=y(z,$));let j=_?y(z,$):d;return j.width=z,j.height=$,j.getContext("2d").drawImage(M,0,0,z,$),We("WebGLRenderer: Texture has been resized from ("+P.width+"x"+P.height+") to ("+z+"x"+$+")."),j}else return"data"in M&&We("WebGLRenderer: Image in DataTexture is too big ("+P.width+"x"+P.height+")."),M;return M}function p(M){return M.generateMipmaps}function b(M){s.generateMipmap(M)}function S(M){return M.isWebGLCubeRenderTarget?s.TEXTURE_CUBE_MAP:M.isWebGL3DRenderTarget?s.TEXTURE_3D:M.isWebGLArrayRenderTarget||M.isCompressedArrayTexture?s.TEXTURE_2D_ARRAY:s.TEXTURE_2D}function x(M,_,O,k,P,z=!1){if(M!==null){if(s[M]!==void 0)return s[M];We("WebGLRenderer: Attempt to use non-existing WebGL internal format '"+M+"'")}let $;k&&($=e.get("EXT_texture_norm16"),$||We("WebGLRenderer: Unable to use normalized textures without EXT_texture_norm16 extension"));let j=_;if(_===s.RED&&(O===s.FLOAT&&(j=s.R32F),O===s.HALF_FLOAT&&(j=s.R16F),O===s.UNSIGNED_BYTE&&(j=s.R8),O===s.UNSIGNED_SHORT&&$&&(j=$.R16_EXT),O===s.SHORT&&$&&(j=$.R16_SNORM_EXT)),_===s.RED_INTEGER&&(O===s.UNSIGNED_BYTE&&(j=s.R8UI),O===s.UNSIGNED_SHORT&&(j=s.R16UI),O===s.UNSIGNED_INT&&(j=s.R32UI),O===s.BYTE&&(j=s.R8I),O===s.SHORT&&(j=s.R16I),O===s.INT&&(j=s.R32I)),_===s.RG&&(O===s.FLOAT&&(j=s.RG32F),O===s.HALF_FLOAT&&(j=s.RG16F),O===s.UNSIGNED_BYTE&&(j=s.RG8),O===s.UNSIGNED_SHORT&&$&&(j=$.RG16_EXT),O===s.SHORT&&$&&(j=$.RG16_SNORM_EXT)),_===s.RG_INTEGER&&(O===s.UNSIGNED_BYTE&&(j=s.RG8UI),O===s.UNSIGNED_SHORT&&(j=s.RG16UI),O===s.UNSIGNED_INT&&(j=s.RG32UI),O===s.BYTE&&(j=s.RG8I),O===s.SHORT&&(j=s.RG16I),O===s.INT&&(j=s.RG32I)),_===s.RGB_INTEGER&&(O===s.UNSIGNED_BYTE&&(j=s.RGB8UI),O===s.UNSIGNED_SHORT&&(j=s.RGB16UI),O===s.UNSIGNED_INT&&(j=s.RGB32UI),O===s.BYTE&&(j=s.RGB8I),O===s.SHORT&&(j=s.RGB16I),O===s.INT&&(j=s.RGB32I)),_===s.RGBA_INTEGER&&(O===s.UNSIGNED_BYTE&&(j=s.RGBA8UI),O===s.UNSIGNED_SHORT&&(j=s.RGBA16UI),O===s.UNSIGNED_INT&&(j=s.RGBA32UI),O===s.BYTE&&(j=s.RGBA8I),O===s.SHORT&&(j=s.RGBA16I),O===s.INT&&(j=s.RGBA32I)),_===s.RGB&&(O===s.UNSIGNED_SHORT&&$&&(j=$.RGB16_EXT),O===s.SHORT&&$&&(j=$.RGB16_SNORM_EXT),O===s.UNSIGNED_INT_5_9_9_9_REV&&(j=s.RGB9_E5),O===s.UNSIGNED_INT_10F_11F_11F_REV&&(j=s.R11F_G11F_B10F)),_===s.RGBA){let ne=z?eo:ot.getTransfer(P);O===s.FLOAT&&(j=s.RGBA32F),O===s.HALF_FLOAT&&(j=s.RGBA16F),O===s.UNSIGNED_BYTE&&(j=ne===Mt?s.SRGB8_ALPHA8:s.RGBA8),O===s.UNSIGNED_SHORT&&$&&(j=$.RGBA16_EXT),O===s.SHORT&&$&&(j=$.RGBA16_SNORM_EXT),O===s.UNSIGNED_SHORT_4_4_4_4&&(j=s.RGBA4),O===s.UNSIGNED_SHORT_5_5_5_1&&(j=s.RGB5_A1)}return(j===s.R16F||j===s.R32F||j===s.RG16F||j===s.RG32F||j===s.RGBA16F||j===s.RGBA32F)&&e.get("EXT_color_buffer_float"),j}function A(M,_){let O;return M?_===null||_===ri||_===Cr?O=s.DEPTH24_STENCIL8:_===Un?O=s.DEPTH32F_STENCIL8:_===Rr&&(O=s.DEPTH24_STENCIL8,We("DepthTexture: 16 bit depth attachment is not supported with stencil. Using 24-bit attachment.")):_===null||_===ri||_===Cr?O=s.DEPTH_COMPONENT24:_===Un?O=s.DEPTH_COMPONENT32F:_===Rr&&(O=s.DEPTH_COMPONENT16),O}function T(M,_){return p(M)===!0||M.isFramebufferTexture&&M.minFilter!==Kt&&M.minFilter!==kt?Math.log2(Math.max(_.width,_.height))+1:M.mipmaps!==void 0&&M.mipmaps.length>0?M.mipmaps.length:M.isCompressedTexture&&Array.isArray(M.image)?_.mipmaps.length:1}function I(M){let _=M.target;_.removeEventListener("dispose",I),D(_),_.isVideoTexture&&h.delete(_),_.isHTMLTexture&&u.delete(_)}function v(M){let _=M.target;_.removeEventListener("dispose",v),R(_)}function D(M){let _=n.get(M);if(_.__webglInit===void 0)return;let O=M.source,k=f.get(O);if(k){let P=k[_.__cacheKey];P.usedTimes--,P.usedTimes===0&&w(M),Object.keys(k).length===0&&f.delete(O)}n.remove(M)}function w(M){let _=n.get(M);s.deleteTexture(_.__webglTexture);let O=M.source,k=f.get(O);delete k[_.__cacheKey],o.memory.textures--}function R(M){let _=n.get(M);if(M.depthTexture&&(M.depthTexture.dispose(),n.remove(M.depthTexture)),M.isWebGLCubeRenderTarget)for(let k=0;k<6;k++){if(Array.isArray(_.__webglFramebuffer[k]))for(let P=0;P<_.__webglFramebuffer[k].length;P++)s.deleteFramebuffer(_.__webglFramebuffer[k][P]);else s.deleteFramebuffer(_.__webglFramebuffer[k]);_.__webglDepthbuffer&&s.deleteRenderbuffer(_.__webglDepthbuffer[k])}else{if(Array.isArray(_.__webglFramebuffer))for(let k=0;k<_.__webglFramebuffer.length;k++)s.deleteFramebuffer(_.__webglFramebuffer[k]);else s.deleteFramebuffer(_.__webglFramebuffer);if(_.__webglDepthbuffer&&s.deleteRenderbuffer(_.__webglDepthbuffer),_.__webglMultisampledFramebuffer&&s.deleteFramebuffer(_.__webglMultisampledFramebuffer),_.__webglColorRenderbuffer)for(let k=0;k<_.__webglColorRenderbuffer.length;k++)_.__webglColorRenderbuffer[k]&&s.deleteRenderbuffer(_.__webglColorRenderbuffer[k]);_.__webglDepthRenderbuffer&&s.deleteRenderbuffer(_.__webglDepthRenderbuffer)}let O=M.textures;for(let k=0,P=O.length;k<P;k++){let z=n.get(O[k]);z.__webglTexture&&(s.deleteTexture(z.__webglTexture),o.memory.textures--),n.remove(O[k])}n.remove(M)}let L=0;function V(){L=0}function C(){return L}function N(M){L=M}function U(){let M=L;return M>=i.maxTextures&&We("WebGLTextures: Trying to use "+M+" texture units while this GPU supports only "+i.maxTextures),L+=1,M}function E(M){let _=[];return _.push(M.wrapS),_.push(M.wrapT),_.push(M.wrapR||0),_.push(M.magFilter),_.push(M.minFilter),_.push(M.anisotropy),_.push(M.internalFormat),_.push(M.format),_.push(M.type),_.push(M.generateMipmaps),_.push(M.premultiplyAlpha),_.push(M.flipY),_.push(M.unpackAlignment),_.push(M.colorSpace),_.join()}function G(M,_){let O=n.get(M);if(M.isVideoTexture&&q(M),M.isRenderTargetTexture===!1&&M.isExternalTexture!==!0&&M.version>0&&O.__version!==M.version){let k=M.image;if(k===null)We("WebGLRenderer: Texture marked for update but no image data found.");else if(k.complete===!1)We("WebGLRenderer: Texture marked for update but image is incomplete");else{Se(O,M,_);return}}else M.isExternalTexture&&(O.__webglTexture=M.sourceTexture?M.sourceTexture:null);t.bindTexture(s.TEXTURE_2D,O.__webglTexture,s.TEXTURE0+_)}function Y(M,_){let O=n.get(M);if(M.isRenderTargetTexture===!1&&M.version>0&&O.__version!==M.version){Se(O,M,_);return}else M.isExternalTexture&&(O.__webglTexture=M.sourceTexture?M.sourceTexture:null);t.bindTexture(s.TEXTURE_2D_ARRAY,O.__webglTexture,s.TEXTURE0+_)}function X(M,_){let O=n.get(M);if(M.isRenderTargetTexture===!1&&M.version>0&&O.__version!==M.version){Se(O,M,_);return}t.bindTexture(s.TEXTURE_3D,O.__webglTexture,s.TEXTURE0+_)}function ie(M,_){let O=n.get(M);if(M.isCubeDepthTexture!==!0&&M.version>0&&O.__version!==M.version){he(O,M,_);return}t.bindTexture(s.TEXTURE_CUBE_MAP,O.__webglTexture,s.TEXTURE0+_)}let ce={[ui]:s.REPEAT,[Hn]:s.CLAMP_TO_EDGE,[ar]:s.MIRRORED_REPEAT},be={[Kt]:s.NEAREST,[fl]:s.NEAREST_MIPMAP_NEAREST,[Os]:s.NEAREST_MIPMAP_LINEAR,[kt]:s.LINEAR,[Ar]:s.LINEAR_MIPMAP_NEAREST,[si]:s.LINEAR_MIPMAP_LINEAR},Le={[$d]:s.NEVER,[sf]:s.ALWAYS,[Qd]:s.LESS,[Ql]:s.LEQUAL,[ef]:s.EQUAL,[ec]:s.GEQUAL,[tf]:s.GREATER,[nf]:s.NOTEQUAL};function Re(M,_){if(_.type===Un&&e.has("OES_texture_float_linear")===!1&&(_.magFilter===kt||_.magFilter===Ar||_.magFilter===Os||_.magFilter===si||_.minFilter===kt||_.minFilter===Ar||_.minFilter===Os||_.minFilter===si)&&We("WebGLRenderer: Unable to use linear filtering with floating point textures. OES_texture_float_linear not supported on this device."),s.texParameteri(M,s.TEXTURE_WRAP_S,ce[_.wrapS]),s.texParameteri(M,s.TEXTURE_WRAP_T,ce[_.wrapT]),(M===s.TEXTURE_3D||M===s.TEXTURE_2D_ARRAY)&&s.texParameteri(M,s.TEXTURE_WRAP_R,ce[_.wrapR]),s.texParameteri(M,s.TEXTURE_MAG_FILTER,be[_.magFilter]),s.texParameteri(M,s.TEXTURE_MIN_FILTER,be[_.minFilter]),_.compareFunction&&(s.texParameteri(M,s.TEXTURE_COMPARE_MODE,s.COMPARE_REF_TO_TEXTURE),s.texParameteri(M,s.TEXTURE_COMPARE_FUNC,Le[_.compareFunction])),e.has("EXT_texture_filter_anisotropic")===!0){if(_.magFilter===Kt||_.minFilter!==Os&&_.minFilter!==si||_.type===Un&&e.has("OES_texture_float_linear")===!1)return;if(_.anisotropy>1||n.get(_).__currentAnisotropy){let O=e.get("EXT_texture_filter_anisotropic");s.texParameterf(M,O.TEXTURE_MAX_ANISOTROPY_EXT,Math.min(_.anisotropy,i.getMaxAnisotropy())),n.get(_).__currentAnisotropy=_.anisotropy}}}function oe(M,_){let O=!1;M.__webglInit===void 0&&(M.__webglInit=!0,_.addEventListener("dispose",I));let k=_.source,P=f.get(k);P===void 0&&(P={},f.set(k,P));let z=E(_);if(z!==M.__cacheKey){P[z]===void 0&&(P[z]={texture:s.createTexture(),usedTimes:0},o.memory.textures++,O=!0),P[z].usedTimes++;let $=P[M.__cacheKey];$!==void 0&&(P[M.__cacheKey].usedTimes--,$.usedTimes===0&&w(_)),M.__cacheKey=z,M.__webglTexture=P[z].texture}return O}function ge(M,_,O){return Math.floor(Math.floor(M/O)/_)}function ae(M,_,O,k){let z=M.updateRanges;if(z.length===0)t.texSubImage2D(s.TEXTURE_2D,0,0,0,_.width,_.height,O,k,_.data);else{z.sort((Ce,Me)=>Ce.start-Me.start);let $=0;for(let Ce=1;Ce<z.length;Ce++){let Me=z[$],ve=z[Ce],Be=Me.start+Me.count,Ee=ge(ve.start,_.width,4),Ve=ge(Me.start,_.width,4);ve.start<=Be+1&&Ee===Ve&&ge(ve.start+ve.count-1,_.width,4)===Ee?Me.count=Math.max(Me.count,ve.start+ve.count-Me.start):(++$,z[$]=ve)}z.length=$+1;let j=t.getParameter(s.UNPACK_ROW_LENGTH),ne=t.getParameter(s.UNPACK_SKIP_PIXELS),de=t.getParameter(s.UNPACK_SKIP_ROWS);t.pixelStorei(s.UNPACK_ROW_LENGTH,_.width);for(let Ce=0,Me=z.length;Ce<Me;Ce++){let ve=z[Ce],Be=Math.floor(ve.start/4),Ee=Math.ceil(ve.count/4),Ve=Be%_.width,K=Math.floor(Be/_.width),ye=Ee,fe=1;t.pixelStorei(s.UNPACK_SKIP_PIXELS,Ve),t.pixelStorei(s.UNPACK_SKIP_ROWS,K),t.texSubImage2D(s.TEXTURE_2D,0,Ve,K,ye,fe,O,k,_.data)}M.clearUpdateRanges(),t.pixelStorei(s.UNPACK_ROW_LENGTH,j),t.pixelStorei(s.UNPACK_SKIP_PIXELS,ne),t.pixelStorei(s.UNPACK_SKIP_ROWS,de)}}function Se(M,_,O){let k=s.TEXTURE_2D;(_.isDataArrayTexture||_.isCompressedArrayTexture)&&(k=s.TEXTURE_2D_ARRAY),_.isData3DTexture&&(k=s.TEXTURE_3D);let P=oe(M,_),z=_.source;t.bindTexture(k,M.__webglTexture,s.TEXTURE0+O);let $=n.get(z);if(z.version!==$.__version||P===!0){if(t.activeTexture(s.TEXTURE0+O),(typeof ImageBitmap<"u"&&_.image instanceof ImageBitmap)===!1){let fe=ot.getPrimaries(ot.workingColorSpace),Ae=_.colorSpace===Xn?null:ot.getPrimaries(_.colorSpace),Te=_.colorSpace===Xn||fe===Ae?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,_.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,_.premultiplyAlpha),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,Te)}t.pixelStorei(s.UNPACK_ALIGNMENT,_.unpackAlignment);let ne=m(_.image,!1,i.maxTextureSize);ne=it(_,ne);let de=r.convert(_.format,_.colorSpace),Ce=r.convert(_.type),Me=x(_.internalFormat,de,Ce,_.normalized,_.colorSpace,_.isVideoTexture);Re(k,_);let ve,Be=_.mipmaps,Ee=_.isVideoTexture!==!0,Ve=$.__version===void 0||P===!0,K=z.dataReady,ye=T(_,ne);if(_.isDepthTexture)Me=A(_.format===ls,_.type),Ve&&(Ee?t.texStorage2D(s.TEXTURE_2D,1,Me,ne.width,ne.height):t.texImage2D(s.TEXTURE_2D,0,Me,ne.width,ne.height,0,de,Ce,null));else if(_.isDataTexture)if(Be.length>0){Ee&&Ve&&t.texStorage2D(s.TEXTURE_2D,ye,Me,Be[0].width,Be[0].height);for(let fe=0,Ae=Be.length;fe<Ae;fe++)ve=Be[fe],Ee?K&&t.texSubImage2D(s.TEXTURE_2D,fe,0,0,ve.width,ve.height,de,Ce,ve.data):t.texImage2D(s.TEXTURE_2D,fe,Me,ve.width,ve.height,0,de,Ce,ve.data);_.generateMipmaps=!1}else Ee?(Ve&&t.texStorage2D(s.TEXTURE_2D,ye,Me,ne.width,ne.height),K&&ae(_,ne,de,Ce)):t.texImage2D(s.TEXTURE_2D,0,Me,ne.width,ne.height,0,de,Ce,ne.data);else if(_.isCompressedTexture)if(_.isCompressedArrayTexture){Ee&&Ve&&t.texStorage3D(s.TEXTURE_2D_ARRAY,ye,Me,Be[0].width,Be[0].height,ne.depth);for(let fe=0,Ae=Be.length;fe<Ae;fe++)if(ve=Be[fe],_.format!==Fn)if(de!==null)if(Ee){if(K)if(_.layerUpdates.size>0){let Te=Ch(ve.width,ve.height,_.format,_.type);for(let me of _.layerUpdates){let Ge=ve.data.subarray(me*Te/ve.data.BYTES_PER_ELEMENT,(me+1)*Te/ve.data.BYTES_PER_ELEMENT);t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,fe,0,0,me,ve.width,ve.height,1,de,Ge)}_.clearLayerUpdates()}else t.compressedTexSubImage3D(s.TEXTURE_2D_ARRAY,fe,0,0,0,ve.width,ve.height,ne.depth,de,ve.data)}else t.compressedTexImage3D(s.TEXTURE_2D_ARRAY,fe,Me,ve.width,ve.height,ne.depth,0,ve.data,0,0);else We("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()");else Ee?K&&t.texSubImage3D(s.TEXTURE_2D_ARRAY,fe,0,0,0,ve.width,ve.height,ne.depth,de,Ce,ve.data):t.texImage3D(s.TEXTURE_2D_ARRAY,fe,Me,ve.width,ve.height,ne.depth,0,de,Ce,ve.data)}else{Ee&&Ve&&t.texStorage2D(s.TEXTURE_2D,ye,Me,Be[0].width,Be[0].height);for(let fe=0,Ae=Be.length;fe<Ae;fe++)ve=Be[fe],_.format!==Fn?de!==null?Ee?K&&t.compressedTexSubImage2D(s.TEXTURE_2D,fe,0,0,ve.width,ve.height,de,ve.data):t.compressedTexImage2D(s.TEXTURE_2D,fe,Me,ve.width,ve.height,0,ve.data):We("WebGLRenderer: Attempt to load unsupported compressed texture format in .uploadTexture()"):Ee?K&&t.texSubImage2D(s.TEXTURE_2D,fe,0,0,ve.width,ve.height,de,Ce,ve.data):t.texImage2D(s.TEXTURE_2D,fe,Me,ve.width,ve.height,0,de,Ce,ve.data)}else if(_.isDataArrayTexture)if(Ee){if(Ve&&t.texStorage3D(s.TEXTURE_2D_ARRAY,ye,Me,ne.width,ne.height,ne.depth),K)if(_.layerUpdates.size>0){let fe=Ch(ne.width,ne.height,_.format,_.type);for(let Ae of _.layerUpdates){let Te=ne.data.subarray(Ae*fe/ne.data.BYTES_PER_ELEMENT,(Ae+1)*fe/ne.data.BYTES_PER_ELEMENT);t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,Ae,ne.width,ne.height,1,de,Ce,Te)}_.clearLayerUpdates()}else t.texSubImage3D(s.TEXTURE_2D_ARRAY,0,0,0,0,ne.width,ne.height,ne.depth,de,Ce,ne.data)}else t.texImage3D(s.TEXTURE_2D_ARRAY,0,Me,ne.width,ne.height,ne.depth,0,de,Ce,ne.data);else if(_.isData3DTexture)Ee?(Ve&&t.texStorage3D(s.TEXTURE_3D,ye,Me,ne.width,ne.height,ne.depth),K&&t.texSubImage3D(s.TEXTURE_3D,0,0,0,0,ne.width,ne.height,ne.depth,de,Ce,ne.data)):t.texImage3D(s.TEXTURE_3D,0,Me,ne.width,ne.height,ne.depth,0,de,Ce,ne.data);else if(_.isFramebufferTexture){if(Ve)if(Ee)t.texStorage2D(s.TEXTURE_2D,ye,Me,ne.width,ne.height);else{let fe=ne.width,Ae=ne.height;for(let Te=0;Te<ye;Te++)t.texImage2D(s.TEXTURE_2D,Te,Me,fe,Ae,0,de,Ce,null),fe>>=1,Ae>>=1}}else if(_.isHTMLTexture){if("texElementImage2D"in s){let fe=s.canvas;if(fe.hasAttribute("layoutsubtree")||fe.setAttribute("layoutsubtree","true"),ne.parentNode!==fe){fe.appendChild(ne),u.add(_),fe.onpaint=Ae=>{let Te=Ae.changedElements;for(let me of u)Te.includes(me.image)&&(me.needsUpdate=!0)},fe.requestPaint();return}if(s.texElementImage2D.length===3)s.texElementImage2D(s.TEXTURE_2D,s.RGBA8,ne);else{let Te=s.RGBA,me=s.RGBA,Ge=s.UNSIGNED_BYTE;s.texElementImage2D(s.TEXTURE_2D,0,Te,me,Ge,ne)}s.texParameteri(s.TEXTURE_2D,s.TEXTURE_MIN_FILTER,s.LINEAR),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_S,s.CLAMP_TO_EDGE),s.texParameteri(s.TEXTURE_2D,s.TEXTURE_WRAP_T,s.CLAMP_TO_EDGE)}}else if(Be.length>0){if(Ee&&Ve){let fe=Ue(Be[0]);t.texStorage2D(s.TEXTURE_2D,ye,Me,fe.width,fe.height)}for(let fe=0,Ae=Be.length;fe<Ae;fe++)ve=Be[fe],Ee?K&&t.texSubImage2D(s.TEXTURE_2D,fe,0,0,de,Ce,ve):t.texImage2D(s.TEXTURE_2D,fe,Me,de,Ce,ve);_.generateMipmaps=!1}else if(Ee){if(Ve){let fe=Ue(ne);t.texStorage2D(s.TEXTURE_2D,ye,Me,fe.width,fe.height)}K&&t.texSubImage2D(s.TEXTURE_2D,0,0,0,de,Ce,ne)}else t.texImage2D(s.TEXTURE_2D,0,Me,de,Ce,ne);p(_)&&b(k),$.__version=z.version,_.onUpdate&&_.onUpdate(_)}M.__version=_.version}function he(M,_,O){if(_.image.length!==6)return;let k=oe(M,_),P=_.source;t.bindTexture(s.TEXTURE_CUBE_MAP,M.__webglTexture,s.TEXTURE0+O);let z=n.get(P);if(P.version!==z.__version||k===!0){t.activeTexture(s.TEXTURE0+O);let $=ot.getPrimaries(ot.workingColorSpace),j=_.colorSpace===Xn?null:ot.getPrimaries(_.colorSpace),ne=_.colorSpace===Xn||$===j?s.NONE:s.BROWSER_DEFAULT_WEBGL;t.pixelStorei(s.UNPACK_FLIP_Y_WEBGL,_.flipY),t.pixelStorei(s.UNPACK_PREMULTIPLY_ALPHA_WEBGL,_.premultiplyAlpha),t.pixelStorei(s.UNPACK_ALIGNMENT,_.unpackAlignment),t.pixelStorei(s.UNPACK_COLORSPACE_CONVERSION_WEBGL,ne);let de=_.isCompressedTexture||_.image[0].isCompressedTexture,Ce=_.image[0]&&_.image[0].isDataTexture,Me=[];for(let me=0;me<6;me++)!de&&!Ce?Me[me]=m(_.image[me],!0,i.maxCubemapSize):Me[me]=Ce?_.image[me].image:_.image[me],Me[me]=it(_,Me[me]);let ve=Me[0],Be=r.convert(_.format,_.colorSpace),Ee=r.convert(_.type),Ve=x(_.internalFormat,Be,Ee,_.normalized,_.colorSpace),K=_.isVideoTexture!==!0,ye=z.__version===void 0||k===!0,fe=P.dataReady,Ae=T(_,ve);Re(s.TEXTURE_CUBE_MAP,_);let Te;if(de){K&&ye&&t.texStorage2D(s.TEXTURE_CUBE_MAP,Ae,Ve,ve.width,ve.height);for(let me=0;me<6;me++){Te=Me[me].mipmaps;for(let Ge=0;Ge<Te.length;Ge++){let ze=Te[Ge];_.format!==Fn?Be!==null?K?fe&&t.compressedTexSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge,0,0,ze.width,ze.height,Be,ze.data):t.compressedTexImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge,Ve,ze.width,ze.height,0,ze.data):We("WebGLRenderer: Attempt to load unsupported compressed texture format in .setTextureCube()"):K?fe&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge,0,0,ze.width,ze.height,Be,Ee,ze.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge,Ve,ze.width,ze.height,0,Be,Ee,ze.data)}}}else{if(Te=_.mipmaps,K&&ye){Te.length>0&&Ae++;let me=Ue(Me[0]);t.texStorage2D(s.TEXTURE_CUBE_MAP,Ae,Ve,me.width,me.height)}for(let me=0;me<6;me++)if(Ce){K?fe&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,0,0,0,Me[me].width,Me[me].height,Be,Ee,Me[me].data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,0,Ve,Me[me].width,Me[me].height,0,Be,Ee,Me[me].data);for(let Ge=0;Ge<Te.length;Ge++){let Bt=Te[Ge].image[me].image;K?fe&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge+1,0,0,Bt.width,Bt.height,Be,Ee,Bt.data):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge+1,Ve,Bt.width,Bt.height,0,Be,Ee,Bt.data)}}else{K?fe&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,0,0,0,Be,Ee,Me[me]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,0,Ve,Be,Ee,Me[me]);for(let Ge=0;Ge<Te.length;Ge++){let ze=Te[Ge];K?fe&&t.texSubImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge+1,0,0,Be,Ee,ze.image[me]):t.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+me,Ge+1,Ve,Be,Ee,ze.image[me])}}}p(_)&&b(s.TEXTURE_CUBE_MAP),z.__version=P.version,_.onUpdate&&_.onUpdate(_)}M.__version=_.version}function ue(M,_,O,k,P,z){let $=r.convert(O.format,O.colorSpace),j=r.convert(O.type),ne=x(O.internalFormat,$,j,O.normalized,O.colorSpace),de=n.get(_),Ce=n.get(O);if(Ce.__renderTarget=_,!de.__hasExternalTextures){let Me=Math.max(1,_.width>>z),ve=Math.max(1,_.height>>z);P===s.TEXTURE_3D||P===s.TEXTURE_2D_ARRAY?t.texImage3D(P,z,ne,Me,ve,_.depth,0,$,j,null):t.texImage2D(P,z,ne,Me,ve,0,$,j,null)}t.bindFramebuffer(s.FRAMEBUFFER,M),Je(_)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,k,P,Ce.__webglTexture,0,Xe(_)):(P===s.TEXTURE_2D||P>=s.TEXTURE_CUBE_MAP_POSITIVE_X&&P<=s.TEXTURE_CUBE_MAP_NEGATIVE_Z)&&s.framebufferTexture2D(s.FRAMEBUFFER,k,P,Ce.__webglTexture,z),t.bindFramebuffer(s.FRAMEBUFFER,null)}function H(M,_,O){if(s.bindRenderbuffer(s.RENDERBUFFER,M),_.depthBuffer){let k=_.depthTexture,P=k&&k.isDepthTexture?k.type:null,z=A(_.stencilBuffer,P),$=_.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;Je(_)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Xe(_),z,_.width,_.height):O?s.renderbufferStorageMultisample(s.RENDERBUFFER,Xe(_),z,_.width,_.height):s.renderbufferStorage(s.RENDERBUFFER,z,_.width,_.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,$,s.RENDERBUFFER,M)}else{let k=_.textures;for(let P=0;P<k.length;P++){let z=k[P],$=r.convert(z.format,z.colorSpace),j=r.convert(z.type),ne=x(z.internalFormat,$,j,z.normalized,z.colorSpace);Je(_)?a.renderbufferStorageMultisampleEXT(s.RENDERBUFFER,Xe(_),ne,_.width,_.height):O?s.renderbufferStorageMultisample(s.RENDERBUFFER,Xe(_),ne,_.width,_.height):s.renderbufferStorage(s.RENDERBUFFER,ne,_.width,_.height)}}s.bindRenderbuffer(s.RENDERBUFFER,null)}function Z(M,_,O){let k=_.isWebGLCubeRenderTarget===!0;if(t.bindFramebuffer(s.FRAMEBUFFER,M),!(_.depthTexture&&_.depthTexture.isDepthTexture))throw new Error("THREE.WebGLTextures: renderTarget.depthTexture must be an instance of THREE.DepthTexture.");let P=n.get(_.depthTexture);if(P.__renderTarget=_,(!P.__webglTexture||_.depthTexture.image.width!==_.width||_.depthTexture.image.height!==_.height)&&(_.depthTexture.image.width=_.width,_.depthTexture.image.height=_.height,_.depthTexture.needsUpdate=!0),k){if(P.__webglInit===void 0&&(P.__webglInit=!0,_.depthTexture.addEventListener("dispose",I)),P.__webglTexture===void 0){P.__webglTexture=s.createTexture(),t.bindTexture(s.TEXTURE_CUBE_MAP,P.__webglTexture),Re(s.TEXTURE_CUBE_MAP,_.depthTexture);let de=r.convert(_.depthTexture.format),Ce=r.convert(_.depthTexture.type),Me;_.depthTexture.format===di?Me=s.DEPTH_COMPONENT24:_.depthTexture.format===ls&&(Me=s.DEPTH24_STENCIL8);for(let ve=0;ve<6;ve++)s.texImage2D(s.TEXTURE_CUBE_MAP_POSITIVE_X+ve,0,Me,_.width,_.height,0,de,Ce,null)}}else G(_.depthTexture,0);let z=P.__webglTexture,$=Xe(_),j=k?s.TEXTURE_CUBE_MAP_POSITIVE_X+O:s.TEXTURE_2D,ne=_.depthTexture.format===ls?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;if(_.depthTexture.format===di)Je(_)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,ne,j,z,0,$):s.framebufferTexture2D(s.FRAMEBUFFER,ne,j,z,0);else if(_.depthTexture.format===ls)Je(_)?a.framebufferTexture2DMultisampleEXT(s.FRAMEBUFFER,ne,j,z,0,$):s.framebufferTexture2D(s.FRAMEBUFFER,ne,j,z,0);else throw new Error("THREE.WebGLTextures: Unknown depthTexture format.")}function W(M){let _=n.get(M),O=M.isWebGLCubeRenderTarget===!0;if(_.__boundDepthTexture!==M.depthTexture){let k=M.depthTexture;if(_.__depthDisposeCallback&&_.__depthDisposeCallback(),k){let P=()=>{delete _.__boundDepthTexture,delete _.__depthDisposeCallback,k.removeEventListener("dispose",P)};k.addEventListener("dispose",P),_.__depthDisposeCallback=P}_.__boundDepthTexture=k}if(M.depthTexture&&!_.__autoAllocateDepthBuffer)if(O)for(let k=0;k<6;k++)Z(_.__webglFramebuffer[k],M,k);else{let k=M.texture.mipmaps;k&&k.length>0?Z(_.__webglFramebuffer[0],M,0):Z(_.__webglFramebuffer,M,0)}else if(O){_.__webglDepthbuffer=[];for(let k=0;k<6;k++)if(t.bindFramebuffer(s.FRAMEBUFFER,_.__webglFramebuffer[k]),_.__webglDepthbuffer[k]===void 0)_.__webglDepthbuffer[k]=s.createRenderbuffer(),H(_.__webglDepthbuffer[k],M,!1);else{let P=M.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,z=_.__webglDepthbuffer[k];s.bindRenderbuffer(s.RENDERBUFFER,z),s.framebufferRenderbuffer(s.FRAMEBUFFER,P,s.RENDERBUFFER,z)}}else{let k=M.texture.mipmaps;if(k&&k.length>0?t.bindFramebuffer(s.FRAMEBUFFER,_.__webglFramebuffer[0]):t.bindFramebuffer(s.FRAMEBUFFER,_.__webglFramebuffer),_.__webglDepthbuffer===void 0)_.__webglDepthbuffer=s.createRenderbuffer(),H(_.__webglDepthbuffer,M,!1);else{let P=M.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,z=_.__webglDepthbuffer;s.bindRenderbuffer(s.RENDERBUFFER,z),s.framebufferRenderbuffer(s.FRAMEBUFFER,P,s.RENDERBUFFER,z)}}t.bindFramebuffer(s.FRAMEBUFFER,null)}function J(M,_,O){let k=n.get(M);_!==void 0&&ue(k.__webglFramebuffer,M,M.texture,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,0),O!==void 0&&W(M)}function te(M){let _=M.texture,O=n.get(M),k=n.get(_);M.addEventListener("dispose",v);let P=M.textures,z=M.isWebGLCubeRenderTarget===!0,$=P.length>1;if($||(k.__webglTexture===void 0&&(k.__webglTexture=s.createTexture()),k.__version=_.version,o.memory.textures++),z){O.__webglFramebuffer=[];for(let j=0;j<6;j++)if(_.mipmaps&&_.mipmaps.length>0){O.__webglFramebuffer[j]=[];for(let ne=0;ne<_.mipmaps.length;ne++)O.__webglFramebuffer[j][ne]=s.createFramebuffer()}else O.__webglFramebuffer[j]=s.createFramebuffer()}else{if(_.mipmaps&&_.mipmaps.length>0){O.__webglFramebuffer=[];for(let j=0;j<_.mipmaps.length;j++)O.__webglFramebuffer[j]=s.createFramebuffer()}else O.__webglFramebuffer=s.createFramebuffer();if($)for(let j=0,ne=P.length;j<ne;j++){let de=n.get(P[j]);de.__webglTexture===void 0&&(de.__webglTexture=s.createTexture(),o.memory.textures++)}if(M.samples>0&&Je(M)===!1){O.__webglMultisampledFramebuffer=s.createFramebuffer(),O.__webglColorRenderbuffer=[],t.bindFramebuffer(s.FRAMEBUFFER,O.__webglMultisampledFramebuffer);for(let j=0;j<P.length;j++){let ne=P[j];O.__webglColorRenderbuffer[j]=s.createRenderbuffer(),s.bindRenderbuffer(s.RENDERBUFFER,O.__webglColorRenderbuffer[j]);let de=r.convert(ne.format,ne.colorSpace),Ce=r.convert(ne.type),Me=x(ne.internalFormat,de,Ce,ne.normalized,ne.colorSpace,M.isXRRenderTarget===!0),ve=Xe(M);s.renderbufferStorageMultisample(s.RENDERBUFFER,ve,Me,M.width,M.height),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+j,s.RENDERBUFFER,O.__webglColorRenderbuffer[j])}s.bindRenderbuffer(s.RENDERBUFFER,null),M.depthBuffer&&(O.__webglDepthRenderbuffer=s.createRenderbuffer(),H(O.__webglDepthRenderbuffer,M,!0)),t.bindFramebuffer(s.FRAMEBUFFER,null)}}if(z){t.bindTexture(s.TEXTURE_CUBE_MAP,k.__webglTexture),Re(s.TEXTURE_CUBE_MAP,_);for(let j=0;j<6;j++)if(_.mipmaps&&_.mipmaps.length>0)for(let ne=0;ne<_.mipmaps.length;ne++)ue(O.__webglFramebuffer[j][ne],M,_,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+j,ne);else ue(O.__webglFramebuffer[j],M,_,s.COLOR_ATTACHMENT0,s.TEXTURE_CUBE_MAP_POSITIVE_X+j,0);p(_)&&b(s.TEXTURE_CUBE_MAP),t.unbindTexture()}else if($){for(let j=0,ne=P.length;j<ne;j++){let de=P[j],Ce=n.get(de),Me=s.TEXTURE_2D;(M.isWebGL3DRenderTarget||M.isWebGLArrayRenderTarget)&&(Me=M.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(Me,Ce.__webglTexture),Re(Me,de),ue(O.__webglFramebuffer,M,de,s.COLOR_ATTACHMENT0+j,Me,0),p(de)&&b(Me)}t.unbindTexture()}else{let j=s.TEXTURE_2D;if((M.isWebGL3DRenderTarget||M.isWebGLArrayRenderTarget)&&(j=M.isWebGL3DRenderTarget?s.TEXTURE_3D:s.TEXTURE_2D_ARRAY),t.bindTexture(j,k.__webglTexture),Re(j,_),_.mipmaps&&_.mipmaps.length>0)for(let ne=0;ne<_.mipmaps.length;ne++)ue(O.__webglFramebuffer[ne],M,_,s.COLOR_ATTACHMENT0,j,ne);else ue(O.__webglFramebuffer,M,_,s.COLOR_ATTACHMENT0,j,0);p(_)&&b(j),t.unbindTexture()}M.depthBuffer&&W(M)}function pe(M){let _=M.textures;for(let O=0,k=_.length;O<k;O++){let P=_[O];if(p(P)){let z=S(M),$=n.get(P).__webglTexture;t.bindTexture(z,$),b(z),t.unbindTexture()}}}let we=[],Pe=[];function je(M){if(M.samples>0){if(Je(M)===!1){let _=M.textures,O=M.width,k=M.height,P=s.COLOR_BUFFER_BIT,z=M.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT,$=n.get(M),j=_.length>1;if(j)for(let de=0;de<_.length;de++)t.bindFramebuffer(s.FRAMEBUFFER,$.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+de,s.RENDERBUFFER,null),t.bindFramebuffer(s.FRAMEBUFFER,$.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+de,s.TEXTURE_2D,null,0);t.bindFramebuffer(s.READ_FRAMEBUFFER,$.__webglMultisampledFramebuffer);let ne=M.texture.mipmaps;ne&&ne.length>0?t.bindFramebuffer(s.DRAW_FRAMEBUFFER,$.__webglFramebuffer[0]):t.bindFramebuffer(s.DRAW_FRAMEBUFFER,$.__webglFramebuffer);for(let de=0;de<_.length;de++){if(M.resolveDepthBuffer&&(M.depthBuffer&&(P|=s.DEPTH_BUFFER_BIT),M.stencilBuffer&&M.resolveStencilBuffer&&(P|=s.STENCIL_BUFFER_BIT)),j){s.framebufferRenderbuffer(s.READ_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.RENDERBUFFER,$.__webglColorRenderbuffer[de]);let Ce=n.get(_[de]).__webglTexture;s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0,s.TEXTURE_2D,Ce,0)}s.blitFramebuffer(0,0,O,k,0,0,O,k,P,s.NEAREST),l===!0&&(we.length=0,Pe.length=0,we.push(s.COLOR_ATTACHMENT0+de),M.depthBuffer&&M.resolveDepthBuffer===!1&&(we.push(z),Pe.push(z),s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,Pe)),s.invalidateFramebuffer(s.READ_FRAMEBUFFER,we))}if(t.bindFramebuffer(s.READ_FRAMEBUFFER,null),t.bindFramebuffer(s.DRAW_FRAMEBUFFER,null),j)for(let de=0;de<_.length;de++){t.bindFramebuffer(s.FRAMEBUFFER,$.__webglMultisampledFramebuffer),s.framebufferRenderbuffer(s.FRAMEBUFFER,s.COLOR_ATTACHMENT0+de,s.RENDERBUFFER,$.__webglColorRenderbuffer[de]);let Ce=n.get(_[de]).__webglTexture;t.bindFramebuffer(s.FRAMEBUFFER,$.__webglFramebuffer),s.framebufferTexture2D(s.DRAW_FRAMEBUFFER,s.COLOR_ATTACHMENT0+de,s.TEXTURE_2D,Ce,0)}t.bindFramebuffer(s.DRAW_FRAMEBUFFER,$.__webglMultisampledFramebuffer)}else if(M.depthBuffer&&M.resolveDepthBuffer===!1&&l){let _=M.stencilBuffer?s.DEPTH_STENCIL_ATTACHMENT:s.DEPTH_ATTACHMENT;s.invalidateFramebuffer(s.DRAW_FRAMEBUFFER,[_])}}}function Xe(M){return Math.min(i.maxSamples,M.samples)}function Je(M){let _=n.get(M);return M.samples>0&&e.has("WEBGL_multisampled_render_to_texture")===!0&&_.__useRenderToTexture!==!1}function q(M){let _=o.render.frame;h.get(M)!==_&&(h.set(M,_),M.update())}function it(M,_){let O=M.colorSpace,k=M.format,P=M.type;return M.isCompressedTexture===!0||M.isVideoTexture===!0||O!==gn&&O!==Xn&&(ot.getTransfer(O)===Mt?(k!==Fn||P!==Sn)&&We("WebGLTextures: sRGB encoded textures have to use RGBAFormat and UnsignedByteType."):$e("WebGLTextures: Unsupported texture color space:",O)),_}function Ue(M){return typeof HTMLImageElement<"u"&&M instanceof HTMLImageElement?(c.width=M.naturalWidth||M.width,c.height=M.naturalHeight||M.height):typeof VideoFrame<"u"&&M instanceof VideoFrame?(c.width=M.displayWidth,c.height=M.displayHeight):(c.width=M.width,c.height=M.height),c}this.allocateTextureUnit=U,this.resetTextureUnits=V,this.getTextureUnits=C,this.setTextureUnits=N,this.setTexture2D=G,this.setTexture2DArray=Y,this.setTexture3D=X,this.setTextureCube=ie,this.rebindTextures=J,this.setupRenderTarget=te,this.updateRenderTargetMipmap=pe,this.updateMultisampleRenderTarget=je,this.setupDepthRenderbuffer=W,this.setupFrameBufferTexture=ue,this.useMultisampledRTT=Je,this.isReversedDepthBuffer=function(){return t.buffers.depth.getReversed()}}function Tv(s,e){function t(n,i=Xn){let r,o=ot.getTransfer(i);if(n===Sn)return s.UNSIGNED_BYTE;if(n===ml)return s.UNSIGNED_SHORT_4_4_4_4;if(n===gl)return s.UNSIGNED_SHORT_5_5_5_1;if(n===_h)return s.UNSIGNED_INT_5_9_9_9_REV;if(n===vh)return s.UNSIGNED_INT_10F_11F_11F_REV;if(n===gh)return s.BYTE;if(n===xh)return s.SHORT;if(n===Rr)return s.UNSIGNED_SHORT;if(n===pl)return s.INT;if(n===ri)return s.UNSIGNED_INT;if(n===Un)return s.FLOAT;if(n===nn)return s.HALF_FLOAT;if(n===yh)return s.ALPHA;if(n===Mh)return s.RGB;if(n===Fn)return s.RGBA;if(n===di)return s.DEPTH_COMPONENT;if(n===ls)return s.DEPTH_STENCIL;if(n===xl)return s.RED;if(n===_l)return s.RED_INTEGER;if(n===cs)return s.RG;if(n===vl)return s.RG_INTEGER;if(n===yl)return s.RGBA_INTEGER;if(n===Oo||n===Bo||n===zo||n===ko)if(o===Mt)if(r=e.get("WEBGL_compressed_texture_s3tc_srgb"),r!==null){if(n===Oo)return r.COMPRESSED_SRGB_S3TC_DXT1_EXT;if(n===Bo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT1_EXT;if(n===zo)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT3_EXT;if(n===ko)return r.COMPRESSED_SRGB_ALPHA_S3TC_DXT5_EXT}else return null;else if(r=e.get("WEBGL_compressed_texture_s3tc"),r!==null){if(n===Oo)return r.COMPRESSED_RGB_S3TC_DXT1_EXT;if(n===Bo)return r.COMPRESSED_RGBA_S3TC_DXT1_EXT;if(n===zo)return r.COMPRESSED_RGBA_S3TC_DXT3_EXT;if(n===ko)return r.COMPRESSED_RGBA_S3TC_DXT5_EXT}else return null;if(n===Ml||n===bl||n===Sl||n===wl)if(r=e.get("WEBGL_compressed_texture_pvrtc"),r!==null){if(n===Ml)return r.COMPRESSED_RGB_PVRTC_4BPPV1_IMG;if(n===bl)return r.COMPRESSED_RGB_PVRTC_2BPPV1_IMG;if(n===Sl)return r.COMPRESSED_RGBA_PVRTC_4BPPV1_IMG;if(n===wl)return r.COMPRESSED_RGBA_PVRTC_2BPPV1_IMG}else return null;if(n===El||n===Tl||n===Al||n===Rl||n===Cl||n===Ho||n===Pl)if(r=e.get("WEBGL_compressed_texture_etc"),r!==null){if(n===El||n===Tl)return o===Mt?r.COMPRESSED_SRGB8_ETC2:r.COMPRESSED_RGB8_ETC2;if(n===Al)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ETC2_EAC:r.COMPRESSED_RGBA8_ETC2_EAC;if(n===Rl)return r.COMPRESSED_R11_EAC;if(n===Cl)return r.COMPRESSED_SIGNED_R11_EAC;if(n===Ho)return r.COMPRESSED_RG11_EAC;if(n===Pl)return r.COMPRESSED_SIGNED_RG11_EAC}else return null;if(n===Il||n===Ll||n===Dl||n===Nl||n===Ul||n===Fl||n===Ol||n===Bl||n===zl||n===kl||n===Hl||n===Vl||n===Gl||n===Wl)if(r=e.get("WEBGL_compressed_texture_astc"),r!==null){if(n===Il)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_4x4_KHR:r.COMPRESSED_RGBA_ASTC_4x4_KHR;if(n===Ll)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x4_KHR:r.COMPRESSED_RGBA_ASTC_5x4_KHR;if(n===Dl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_5x5_KHR:r.COMPRESSED_RGBA_ASTC_5x5_KHR;if(n===Nl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x5_KHR:r.COMPRESSED_RGBA_ASTC_6x5_KHR;if(n===Ul)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_6x6_KHR:r.COMPRESSED_RGBA_ASTC_6x6_KHR;if(n===Fl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x5_KHR:r.COMPRESSED_RGBA_ASTC_8x5_KHR;if(n===Ol)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x6_KHR:r.COMPRESSED_RGBA_ASTC_8x6_KHR;if(n===Bl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_8x8_KHR:r.COMPRESSED_RGBA_ASTC_8x8_KHR;if(n===zl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x5_KHR:r.COMPRESSED_RGBA_ASTC_10x5_KHR;if(n===kl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x6_KHR:r.COMPRESSED_RGBA_ASTC_10x6_KHR;if(n===Hl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x8_KHR:r.COMPRESSED_RGBA_ASTC_10x8_KHR;if(n===Vl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_10x10_KHR:r.COMPRESSED_RGBA_ASTC_10x10_KHR;if(n===Gl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x10_KHR:r.COMPRESSED_RGBA_ASTC_12x10_KHR;if(n===Wl)return o===Mt?r.COMPRESSED_SRGB8_ALPHA8_ASTC_12x12_KHR:r.COMPRESSED_RGBA_ASTC_12x12_KHR}else return null;if(n===Xl||n===ql||n===Yl)if(r=e.get("EXT_texture_compression_bptc"),r!==null){if(n===Xl)return o===Mt?r.COMPRESSED_SRGB_ALPHA_BPTC_UNORM_EXT:r.COMPRESSED_RGBA_BPTC_UNORM_EXT;if(n===ql)return r.COMPRESSED_RGB_BPTC_SIGNED_FLOAT_EXT;if(n===Yl)return r.COMPRESSED_RGB_BPTC_UNSIGNED_FLOAT_EXT}else return null;if(n===Zl||n===Kl||n===Vo||n===jl)if(r=e.get("EXT_texture_compression_rgtc"),r!==null){if(n===Zl)return r.COMPRESSED_RED_RGTC1_EXT;if(n===Kl)return r.COMPRESSED_SIGNED_RED_RGTC1_EXT;if(n===Vo)return r.COMPRESSED_RED_GREEN_RGTC2_EXT;if(n===jl)return r.COMPRESSED_SIGNED_RED_GREEN_RGTC2_EXT}else return null;return n===Cr?s.UNSIGNED_INT_24_8:s[n]!==void 0?s[n]:null}return{convert:t}}var Av=`
void main() {

	gl_Position = vec4( position, 1.0 );

}`,Rv=`
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

}`,Gh=class{constructor(){this.texture=null,this.mesh=null,this.depthNear=0,this.depthFar=0}init(e,t){if(this.texture===null){let n=new ho(e.texture);(e.depthNear!==t.depthNear||e.depthFar!==t.depthFar)&&(this.depthNear=e.depthNear,this.depthFar=e.depthFar),this.texture=n}}getMesh(e){if(this.texture!==null&&this.mesh===null){let t=e.cameras[0].viewport,n=new mt({vertexShader:Av,fragmentShader:Rv,uniforms:{depthColor:{value:this.texture},depthWidth:{value:t.z},depthHeight:{value:t.w}}});this.mesh=new Ke(new ln(20,20),n)}return this.mesh}reset(){this.texture=null,this.mesh=null}getDepthTexture(){return this.texture}},Wh=class extends Vn{constructor(e,t){super();let n=this,i=null,r=1,o=null,a="local-floor",l=1,c=null,h=null,u=null,d=null,f=null,g=null,y=typeof XRWebGLBinding<"u",m=new Gh,p={},b=t.getContextAttributes(),S=null,x=null,A=[],T=[],I=new _e,v=null,D=new Qt;D.viewport=new _t;let w=new Qt;w.viewport=new _t;let R=[D,w],L=new ol,V=null,C=null;this.cameraAutoUpdate=!0,this.enabled=!1,this.isPresenting=!1,this.getController=function(oe){let ge=A[oe];return ge===void 0&&(ge=new fr,A[oe]=ge),ge.getTargetRaySpace()},this.getControllerGrip=function(oe){let ge=A[oe];return ge===void 0&&(ge=new fr,A[oe]=ge),ge.getGripSpace()},this.getHand=function(oe){let ge=A[oe];return ge===void 0&&(ge=new fr,A[oe]=ge),ge.getHandSpace()};function N(oe){let ge=T.indexOf(oe.inputSource);if(ge===-1)return;let ae=A[ge];ae!==void 0&&(ae.update(oe.inputSource,oe.frame,c||o),ae.dispatchEvent({type:oe.type,data:oe.inputSource}))}function U(){i.removeEventListener("select",N),i.removeEventListener("selectstart",N),i.removeEventListener("selectend",N),i.removeEventListener("squeeze",N),i.removeEventListener("squeezestart",N),i.removeEventListener("squeezeend",N),i.removeEventListener("end",U),i.removeEventListener("inputsourceschange",E);for(let oe=0;oe<A.length;oe++){let ge=T[oe];ge!==null&&(T[oe]=null,A[oe].disconnect(ge))}V=null,C=null,m.reset();for(let oe in p)delete p[oe];e.setRenderTarget(S),f=null,d=null,u=null,i=null,x=null,Re.stop(),n.isPresenting=!1,e.setPixelRatio(v),e.setSize(I.width,I.height,!1),n.dispatchEvent({type:"sessionend"})}this.setFramebufferScaleFactor=function(oe){r=oe,n.isPresenting===!0&&We("WebXRManager: Cannot change framebuffer scale while presenting.")},this.setReferenceSpaceType=function(oe){a=oe,n.isPresenting===!0&&We("WebXRManager: Cannot change reference space type while presenting.")},this.getReferenceSpace=function(){return c||o},this.setReferenceSpace=function(oe){c=oe},this.getBaseLayer=function(){return d!==null?d:f},this.getBinding=function(){return u===null&&y&&(u=new XRWebGLBinding(i,t)),u},this.getFrame=function(){return g},this.getSession=function(){return i},this.setSession=async function(oe){if(i=oe,i!==null){if(S=e.getRenderTarget(),i.addEventListener("select",N),i.addEventListener("selectstart",N),i.addEventListener("selectend",N),i.addEventListener("squeeze",N),i.addEventListener("squeezestart",N),i.addEventListener("squeezeend",N),i.addEventListener("end",U),i.addEventListener("inputsourceschange",E),b.xrCompatible!==!0&&await t.makeXRCompatible(),v=e.getPixelRatio(),e.getSize(I),y&&"createProjectionLayer"in XRWebGLBinding.prototype){let ae=null,Se=null,he=null;b.depth&&(he=b.stencil?t.DEPTH24_STENCIL8:t.DEPTH_COMPONENT24,ae=b.stencil?ls:di,Se=b.stencil?Cr:ri);let ue={colorFormat:t.RGBA8,depthFormat:he,scaleFactor:r};u=this.getBinding(),d=u.createProjectionLayer(ue),i.updateRenderState({layers:[d]}),e.setPixelRatio(1),e.setSize(d.textureWidth,d.textureHeight,!1),x=new Wt(d.textureWidth,d.textureHeight,{format:Fn,type:Sn,depthTexture:new Ni(d.textureWidth,d.textureHeight,Se,void 0,void 0,void 0,void 0,void 0,void 0,ae),stencilBuffer:b.stencil,colorSpace:e.outputColorSpace,samples:b.antialias?4:0,resolveDepthBuffer:d.ignoreDepthValues===!1,resolveStencilBuffer:d.ignoreDepthValues===!1})}else{let ae={antialias:b.antialias,alpha:!0,depth:b.depth,stencil:b.stencil,framebufferScaleFactor:r};f=new XRWebGLLayer(i,t,ae),i.updateRenderState({baseLayer:f}),e.setPixelRatio(1),e.setSize(f.framebufferWidth,f.framebufferHeight,!1),x=new Wt(f.framebufferWidth,f.framebufferHeight,{format:Fn,type:Sn,colorSpace:e.outputColorSpace,stencilBuffer:b.stencil,resolveDepthBuffer:f.ignoreDepthValues===!1,resolveStencilBuffer:f.ignoreDepthValues===!1})}x.isXRRenderTarget=!0,this.setFoveation(l),c=null,o=await i.requestReferenceSpace(a),Re.setContext(i),Re.start(),n.isPresenting=!0,n.dispatchEvent({type:"sessionstart"})}},this.getEnvironmentBlendMode=function(){if(i!==null)return i.environmentBlendMode},this.getDepthTexture=function(){return m.getDepthTexture()};function E(oe){for(let ge=0;ge<oe.removed.length;ge++){let ae=oe.removed[ge],Se=T.indexOf(ae);Se>=0&&(T[Se]=null,A[Se].disconnect(ae))}for(let ge=0;ge<oe.added.length;ge++){let ae=oe.added[ge],Se=T.indexOf(ae);if(Se===-1){for(let ue=0;ue<A.length;ue++)if(ue>=T.length){T.push(ae),Se=ue;break}else if(T[ue]===null){T[ue]=ae,Se=ue;break}if(Se===-1)break}let he=A[Se];he&&he.connect(ae)}}let G=new B,Y=new B;function X(oe,ge,ae){G.setFromMatrixPosition(ge.matrixWorld),Y.setFromMatrixPosition(ae.matrixWorld);let Se=G.distanceTo(Y),he=ge.projectionMatrix.elements,ue=ae.projectionMatrix.elements,H=he[14]/(he[10]-1),Z=he[14]/(he[10]+1),W=(he[9]+1)/he[5],J=(he[9]-1)/he[5],te=(he[8]-1)/he[0],pe=(ue[8]+1)/ue[0],we=H*te,Pe=H*pe,je=Se/(-te+pe),Xe=je*-te;if(ge.matrixWorld.decompose(oe.position,oe.quaternion,oe.scale),oe.translateX(Xe),oe.translateZ(je),oe.matrixWorld.compose(oe.position,oe.quaternion,oe.scale),oe.matrixWorldInverse.copy(oe.matrixWorld).invert(),he[10]===-1)oe.projectionMatrix.copy(ge.projectionMatrix),oe.projectionMatrixInverse.copy(ge.projectionMatrixInverse);else{let Je=H+je,q=Z+je,it=we-Xe,Ue=Pe+(Se-Xe),M=W*Z/q*Je,_=J*Z/q*Je;oe.projectionMatrix.makePerspective(it,Ue,M,_,Je,q),oe.projectionMatrixInverse.copy(oe.projectionMatrix).invert()}}function ie(oe,ge){ge===null?oe.matrixWorld.copy(oe.matrix):oe.matrixWorld.multiplyMatrices(ge.matrixWorld,oe.matrix),oe.matrixWorldInverse.copy(oe.matrixWorld).invert()}this.updateCamera=function(oe){if(i===null)return;let ge=oe.near,ae=oe.far;m.texture!==null&&(m.depthNear>0&&(ge=m.depthNear),m.depthFar>0&&(ae=m.depthFar)),L.near=w.near=D.near=ge,L.far=w.far=D.far=ae,(V!==L.near||C!==L.far)&&(i.updateRenderState({depthNear:L.near,depthFar:L.far}),V=L.near,C=L.far),L.layers.mask=oe.layers.mask|6,D.layers.mask=L.layers.mask&-5,w.layers.mask=L.layers.mask&-3;let Se=oe.parent,he=L.cameras;ie(L,Se);for(let ue=0;ue<he.length;ue++)ie(he[ue],Se);he.length===2?X(L,D,w):L.projectionMatrix.copy(D.projectionMatrix),ce(oe,L,Se)};function ce(oe,ge,ae){ae===null?oe.matrix.copy(ge.matrixWorld):(oe.matrix.copy(ae.matrixWorld),oe.matrix.invert(),oe.matrix.multiply(ge.matrixWorld)),oe.matrix.decompose(oe.position,oe.quaternion,oe.scale),oe.updateMatrixWorld(!0),oe.projectionMatrix.copy(ge.projectionMatrix),oe.projectionMatrixInverse.copy(ge.projectionMatrixInverse),oe.isPerspectiveCamera&&(oe.fov=As*2*Math.atan(1/oe.projectionMatrix.elements[5]),oe.zoom=1)}this.getCamera=function(){return L},this.getFoveation=function(){if(!(d===null&&f===null))return l},this.setFoveation=function(oe){l=oe,d!==null&&(d.fixedFoveation=oe),f!==null&&f.fixedFoveation!==void 0&&(f.fixedFoveation=oe)},this.hasDepthSensing=function(){return m.texture!==null},this.getDepthSensingMesh=function(){return m.getMesh(L)},this.getCameraTexture=function(oe){return p[oe]};let be=null;function Le(oe,ge){if(h=ge.getViewerPose(c||o),g=ge,h!==null){let ae=h.views;f!==null&&(e.setRenderTargetFramebuffer(x,f.framebuffer),e.setRenderTarget(x));let Se=!1;ae.length!==L.cameras.length&&(L.cameras.length=0,Se=!0);for(let Z=0;Z<ae.length;Z++){let W=ae[Z],J=null;if(f!==null)J=f.getViewport(W);else{let pe=u.getViewSubImage(d,W);J=pe.viewport,Z===0&&(e.setRenderTargetTextures(x,pe.colorTexture,pe.depthStencilTexture),e.setRenderTarget(x))}let te=R[Z];te===void 0&&(te=new Qt,te.layers.enable(Z),te.viewport=new _t,R[Z]=te),te.matrix.fromArray(W.transform.matrix),te.matrix.decompose(te.position,te.quaternion,te.scale),te.projectionMatrix.fromArray(W.projectionMatrix),te.projectionMatrixInverse.copy(te.projectionMatrix).invert(),te.viewport.set(J.x,J.y,J.width,J.height),Z===0&&(L.matrix.copy(te.matrix),L.matrix.decompose(L.position,L.quaternion,L.scale)),Se===!0&&L.cameras.push(te)}let he=i.enabledFeatures;if(he&&he.includes("depth-sensing")&&i.depthUsage=="gpu-optimized"&&y){u=n.getBinding();let Z=u.getDepthInformation(ae[0]);Z&&Z.isValid&&Z.texture&&m.init(Z,i.renderState)}if(he&&he.includes("camera-access")&&y){e.state.unbindTexture(),u=n.getBinding();for(let Z=0;Z<ae.length;Z++){let W=ae[Z].camera;if(W){let J=p[W];J||(J=new ho,p[W]=J);let te=u.getCameraImage(W);J.sourceTexture=te}}}}for(let ae=0;ae<A.length;ae++){let Se=T[ae],he=A[ae];Se!==null&&he!==void 0&&he.update(Se,ge,c||o)}be&&be(oe,ge),ge.detectedPlanes&&n.dispatchEvent({type:"planesdetected",data:ge}),g=null}let Re=new Df;Re.setAnimationLoop(Le),this.setAnimationLoop=function(oe){be=oe},this.dispose=function(){}}},Cv=new Qe,zf=new et;zf.set(-1,0,0,0,1,0,0,0,1);function Pv(s,e){function t(m,p){m.matrixAutoUpdate===!0&&m.updateMatrix(),p.value.copy(m.matrix)}function n(m,p){p.color.getRGB(m.fogColor.value,Th(s)),p.isFog?(m.fogNear.value=p.near,m.fogFar.value=p.far):p.isFogExp2&&(m.fogDensity.value=p.density)}function i(m,p,b,S,x){p.isNodeMaterial?p.uniformsNeedUpdate=!1:p.isMeshBasicMaterial?r(m,p):p.isMeshLambertMaterial?(r(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshToonMaterial?(r(m,p),u(m,p)):p.isMeshPhongMaterial?(r(m,p),h(m,p),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)):p.isMeshStandardMaterial?(r(m,p),d(m,p),p.isMeshPhysicalMaterial&&f(m,p,x)):p.isMeshMatcapMaterial?(r(m,p),g(m,p)):p.isMeshDepthMaterial?r(m,p):p.isMeshDistanceMaterial?(r(m,p),y(m,p)):p.isMeshNormalMaterial?r(m,p):p.isLineBasicMaterial?(o(m,p),p.isLineDashedMaterial&&a(m,p)):p.isPointsMaterial?l(m,p,b,S):p.isSpriteMaterial?c(m,p):p.isShadowMaterial?(m.color.value.copy(p.color),m.opacity.value=p.opacity):p.isShaderMaterial&&(p.uniformsNeedUpdate=!1)}function r(m,p){m.opacity.value=p.opacity,p.color&&m.diffuse.value.copy(p.color),p.emissive&&m.emissive.value.copy(p.emissive).multiplyScalar(p.emissiveIntensity),p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.bumpMap&&(m.bumpMap.value=p.bumpMap,t(p.bumpMap,m.bumpMapTransform),m.bumpScale.value=p.bumpScale,p.side===tn&&(m.bumpScale.value*=-1)),p.normalMap&&(m.normalMap.value=p.normalMap,t(p.normalMap,m.normalMapTransform),m.normalScale.value.copy(p.normalScale),p.side===tn&&m.normalScale.value.negate()),p.displacementMap&&(m.displacementMap.value=p.displacementMap,t(p.displacementMap,m.displacementMapTransform),m.displacementScale.value=p.displacementScale,m.displacementBias.value=p.displacementBias),p.emissiveMap&&(m.emissiveMap.value=p.emissiveMap,t(p.emissiveMap,m.emissiveMapTransform)),p.specularMap&&(m.specularMap.value=p.specularMap,t(p.specularMap,m.specularMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest);let b=e.get(p),S=b.envMap,x=b.envMapRotation;S&&(m.envMap.value=S,m.envMapRotation.value.setFromMatrix4(Cv.makeRotationFromEuler(x)).transpose(),S.isCubeTexture&&S.isRenderTargetTexture===!1&&m.envMapRotation.value.premultiply(zf),m.reflectivity.value=p.reflectivity,m.ior.value=p.ior,m.refractionRatio.value=p.refractionRatio),p.lightMap&&(m.lightMap.value=p.lightMap,m.lightMapIntensity.value=p.lightMapIntensity,t(p.lightMap,m.lightMapTransform)),p.aoMap&&(m.aoMap.value=p.aoMap,m.aoMapIntensity.value=p.aoMapIntensity,t(p.aoMap,m.aoMapTransform))}function o(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform))}function a(m,p){m.dashSize.value=p.dashSize,m.totalSize.value=p.dashSize+p.gapSize,m.scale.value=p.scale}function l(m,p,b,S){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.size.value=p.size*b,m.scale.value=S*.5,p.map&&(m.map.value=p.map,t(p.map,m.uvTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function c(m,p){m.diffuse.value.copy(p.color),m.opacity.value=p.opacity,m.rotation.value=p.rotation,p.map&&(m.map.value=p.map,t(p.map,m.mapTransform)),p.alphaMap&&(m.alphaMap.value=p.alphaMap,t(p.alphaMap,m.alphaMapTransform)),p.alphaTest>0&&(m.alphaTest.value=p.alphaTest)}function h(m,p){m.specular.value.copy(p.specular),m.shininess.value=Math.max(p.shininess,1e-4)}function u(m,p){p.gradientMap&&(m.gradientMap.value=p.gradientMap)}function d(m,p){m.metalness.value=p.metalness,p.metalnessMap&&(m.metalnessMap.value=p.metalnessMap,t(p.metalnessMap,m.metalnessMapTransform)),m.roughness.value=p.roughness,p.roughnessMap&&(m.roughnessMap.value=p.roughnessMap,t(p.roughnessMap,m.roughnessMapTransform)),p.envMap&&(m.envMapIntensity.value=p.envMapIntensity)}function f(m,p,b){m.ior.value=p.ior,p.sheen>0&&(m.sheenColor.value.copy(p.sheenColor).multiplyScalar(p.sheen),m.sheenRoughness.value=p.sheenRoughness,p.sheenColorMap&&(m.sheenColorMap.value=p.sheenColorMap,t(p.sheenColorMap,m.sheenColorMapTransform)),p.sheenRoughnessMap&&(m.sheenRoughnessMap.value=p.sheenRoughnessMap,t(p.sheenRoughnessMap,m.sheenRoughnessMapTransform))),p.clearcoat>0&&(m.clearcoat.value=p.clearcoat,m.clearcoatRoughness.value=p.clearcoatRoughness,p.clearcoatMap&&(m.clearcoatMap.value=p.clearcoatMap,t(p.clearcoatMap,m.clearcoatMapTransform)),p.clearcoatRoughnessMap&&(m.clearcoatRoughnessMap.value=p.clearcoatRoughnessMap,t(p.clearcoatRoughnessMap,m.clearcoatRoughnessMapTransform)),p.clearcoatNormalMap&&(m.clearcoatNormalMap.value=p.clearcoatNormalMap,t(p.clearcoatNormalMap,m.clearcoatNormalMapTransform),m.clearcoatNormalScale.value.copy(p.clearcoatNormalScale),p.side===tn&&m.clearcoatNormalScale.value.negate())),p.dispersion>0&&(m.dispersion.value=p.dispersion),p.iridescence>0&&(m.iridescence.value=p.iridescence,m.iridescenceIOR.value=p.iridescenceIOR,m.iridescenceThicknessMinimum.value=p.iridescenceThicknessRange[0],m.iridescenceThicknessMaximum.value=p.iridescenceThicknessRange[1],p.iridescenceMap&&(m.iridescenceMap.value=p.iridescenceMap,t(p.iridescenceMap,m.iridescenceMapTransform)),p.iridescenceThicknessMap&&(m.iridescenceThicknessMap.value=p.iridescenceThicknessMap,t(p.iridescenceThicknessMap,m.iridescenceThicknessMapTransform))),p.transmission>0&&(m.transmission.value=p.transmission,m.transmissionSamplerMap.value=b.texture,m.transmissionSamplerSize.value.set(b.width,b.height),p.transmissionMap&&(m.transmissionMap.value=p.transmissionMap,t(p.transmissionMap,m.transmissionMapTransform)),m.thickness.value=p.thickness,p.thicknessMap&&(m.thicknessMap.value=p.thicknessMap,t(p.thicknessMap,m.thicknessMapTransform)),m.attenuationDistance.value=p.attenuationDistance,m.attenuationColor.value.copy(p.attenuationColor)),p.anisotropy>0&&(m.anisotropyVector.value.set(p.anisotropy*Math.cos(p.anisotropyRotation),p.anisotropy*Math.sin(p.anisotropyRotation)),p.anisotropyMap&&(m.anisotropyMap.value=p.anisotropyMap,t(p.anisotropyMap,m.anisotropyMapTransform))),m.specularIntensity.value=p.specularIntensity,m.specularColor.value.copy(p.specularColor),p.specularColorMap&&(m.specularColorMap.value=p.specularColorMap,t(p.specularColorMap,m.specularColorMapTransform)),p.specularIntensityMap&&(m.specularIntensityMap.value=p.specularIntensityMap,t(p.specularIntensityMap,m.specularIntensityMapTransform))}function g(m,p){p.matcap&&(m.matcap.value=p.matcap)}function y(m,p){let b=e.get(p).light;m.referencePosition.value.setFromMatrixPosition(b.matrixWorld),m.nearDistance.value=b.shadow.camera.near,m.farDistance.value=b.shadow.camera.far}return{refreshFogUniforms:n,refreshMaterialUniforms:i}}function Iv(s,e,t,n){let i={},r={},o=[],a=s.getParameter(s.MAX_UNIFORM_BUFFER_BINDINGS);function l(x,A){let T=A.program;n.uniformBlockBinding(x,T)}function c(x,A){let T=i[x.id];T===void 0&&(m(x),T=h(x),i[x.id]=T,x.addEventListener("dispose",b));let I=A.program;n.updateUBOMapping(x,I);let v=e.render.frame;r[x.id]!==v&&(d(x),r[x.id]=v)}function h(x){let A=u();x.__bindingPointIndex=A;let T=s.createBuffer(),I=x.__size,v=x.usage;return s.bindBuffer(s.UNIFORM_BUFFER,T),s.bufferData(s.UNIFORM_BUFFER,I,v),s.bindBuffer(s.UNIFORM_BUFFER,null),s.bindBufferBase(s.UNIFORM_BUFFER,A,T),T}function u(){for(let x=0;x<a;x++)if(o.indexOf(x)===-1)return o.push(x),x;return $e("WebGLRenderer: Maximum number of simultaneously usable uniforms groups reached."),0}function d(x){let A=i[x.id],T=x.uniforms,I=x.__cache;s.bindBuffer(s.UNIFORM_BUFFER,A);for(let v=0,D=T.length;v<D;v++){let w=T[v];if(Array.isArray(w))for(let R=0,L=w.length;R<L;R++)f(w[R],v,R,I);else f(w,v,0,I)}s.bindBuffer(s.UNIFORM_BUFFER,null)}function f(x,A,T,I){if(y(x,A,T,I)===!0){let v=x.__offset,D=x.value;if(Array.isArray(D)){let w=0;for(let R=0;R<D.length;R++){let L=D[R],V=p(L);g(L,x.__data,w),typeof L!="number"&&typeof L!="boolean"&&!L.isMatrix3&&!ArrayBuffer.isView(L)&&(w+=V.storage/Float32Array.BYTES_PER_ELEMENT)}}else g(D,x.__data,0);s.bufferSubData(s.UNIFORM_BUFFER,v,x.__data)}}function g(x,A,T){typeof x=="number"||typeof x=="boolean"?A[0]=x:x.isMatrix3?(A[0]=x.elements[0],A[1]=x.elements[1],A[2]=x.elements[2],A[3]=0,A[4]=x.elements[3],A[5]=x.elements[4],A[6]=x.elements[5],A[7]=0,A[8]=x.elements[6],A[9]=x.elements[7],A[10]=x.elements[8],A[11]=0):ArrayBuffer.isView(x)?A.set(new x.constructor(x.buffer,x.byteOffset,A.length)):x.toArray(A,T)}function y(x,A,T,I){let v=x.value,D=A+"_"+T;if(I[D]===void 0)return typeof v=="number"||typeof v=="boolean"?I[D]=v:ArrayBuffer.isView(v)?I[D]=v.slice():I[D]=v.clone(),!0;{let w=I[D];if(typeof v=="number"||typeof v=="boolean"){if(w!==v)return I[D]=v,!0}else{if(ArrayBuffer.isView(v))return!0;if(w.equals(v)===!1)return w.copy(v),!0}}return!1}function m(x){let A=x.uniforms,T=0,I=16;for(let D=0,w=A.length;D<w;D++){let R=Array.isArray(A[D])?A[D]:[A[D]];for(let L=0,V=R.length;L<V;L++){let C=R[L],N=Array.isArray(C.value)?C.value:[C.value];for(let U=0,E=N.length;U<E;U++){let G=N[U],Y=p(G),X=T%I,ie=X%Y.boundary,ce=X+ie;T+=ie,ce!==0&&I-ce<Y.storage&&(T+=I-ce),C.__data=new Float32Array(Y.storage/Float32Array.BYTES_PER_ELEMENT),C.__offset=T,T+=Y.storage}}}let v=T%I;return v>0&&(T+=I-v),x.__size=T,x.__cache={},this}function p(x){let A={boundary:0,storage:0};return typeof x=="number"||typeof x=="boolean"?(A.boundary=4,A.storage=4):x.isVector2?(A.boundary=8,A.storage=8):x.isVector3||x.isColor?(A.boundary=16,A.storage=12):x.isVector4?(A.boundary=16,A.storage=16):x.isMatrix3?(A.boundary=48,A.storage=48):x.isMatrix4?(A.boundary=64,A.storage=64):x.isTexture?We("WebGLRenderer: Texture samplers can not be part of an uniforms group."):ArrayBuffer.isView(x)?(A.boundary=16,A.storage=x.byteLength):We("WebGLRenderer: Unsupported uniform value type.",x),A}function b(x){let A=x.target;A.removeEventListener("dispose",b);let T=o.indexOf(A.__bindingPointIndex);o.splice(T,1),s.deleteBuffer(i[A.id]),delete i[A.id],delete r[A.id]}function S(){for(let x in i)s.deleteBuffer(i[x]);o=[],i={},r={}}return{bind:l,update:c,dispose:S}}var Lv=new Uint16Array([12469,15057,12620,14925,13266,14620,13807,14376,14323,13990,14545,13625,14713,13328,14840,12882,14931,12528,14996,12233,15039,11829,15066,11525,15080,11295,15085,10976,15082,10705,15073,10495,13880,14564,13898,14542,13977,14430,14158,14124,14393,13732,14556,13410,14702,12996,14814,12596,14891,12291,14937,11834,14957,11489,14958,11194,14943,10803,14921,10506,14893,10278,14858,9960,14484,14039,14487,14025,14499,13941,14524,13740,14574,13468,14654,13106,14743,12678,14818,12344,14867,11893,14889,11509,14893,11180,14881,10751,14852,10428,14812,10128,14765,9754,14712,9466,14764,13480,14764,13475,14766,13440,14766,13347,14769,13070,14786,12713,14816,12387,14844,11957,14860,11549,14868,11215,14855,10751,14825,10403,14782,10044,14729,9651,14666,9352,14599,9029,14967,12835,14966,12831,14963,12804,14954,12723,14936,12564,14917,12347,14900,11958,14886,11569,14878,11247,14859,10765,14828,10401,14784,10011,14727,9600,14660,9289,14586,8893,14508,8533,15111,12234,15110,12234,15104,12216,15092,12156,15067,12010,15028,11776,14981,11500,14942,11205,14902,10752,14861,10393,14812,9991,14752,9570,14682,9252,14603,8808,14519,8445,14431,8145,15209,11449,15208,11451,15202,11451,15190,11438,15163,11384,15117,11274,15055,10979,14994,10648,14932,10343,14871,9936,14803,9532,14729,9218,14645,8742,14556,8381,14461,8020,14365,7603,15273,10603,15272,10607,15267,10619,15256,10631,15231,10614,15182,10535,15118,10389,15042,10167,14963,9787,14883,9447,14800,9115,14710,8665,14615,8318,14514,7911,14411,7507,14279,7198,15314,9675,15313,9683,15309,9712,15298,9759,15277,9797,15229,9773,15166,9668,15084,9487,14995,9274,14898,8910,14800,8539,14697,8234,14590,7790,14479,7409,14367,7067,14178,6621,15337,8619,15337,8631,15333,8677,15325,8769,15305,8871,15264,8940,15202,8909,15119,8775,15022,8565,14916,8328,14804,8009,14688,7614,14569,7287,14448,6888,14321,6483,14088,6171,15350,7402,15350,7419,15347,7480,15340,7613,15322,7804,15287,7973,15229,8057,15148,8012,15046,7846,14933,7611,14810,7357,14682,7069,14552,6656,14421,6316,14251,5948,14007,5528,15356,5942,15356,5977,15353,6119,15348,6294,15332,6551,15302,6824,15249,7044,15171,7122,15070,7050,14949,6861,14818,6611,14679,6349,14538,6067,14398,5651,14189,5311,13935,4958,15359,4123,15359,4153,15356,4296,15353,4646,15338,5160,15311,5508,15263,5829,15188,6042,15088,6094,14966,6001,14826,5796,14678,5543,14527,5287,14377,4985,14133,4586,13869,4257,15360,1563,15360,1642,15358,2076,15354,2636,15341,3350,15317,4019,15273,4429,15203,4732,15105,4911,14981,4932,14836,4818,14679,4621,14517,4386,14359,4156,14083,3795,13808,3437,15360,122,15360,137,15358,285,15355,636,15344,1274,15322,2177,15281,2765,15215,3223,15120,3451,14995,3569,14846,3567,14681,3466,14511,3305,14344,3121,14037,2800,13753,2467,15360,0,15360,1,15359,21,15355,89,15346,253,15325,479,15287,796,15225,1148,15133,1492,15008,1749,14856,1882,14685,1886,14506,1783,14324,1608,13996,1398,13702,1183]),yi=null;function Dv(){return yi===null&&(yi=new es(Lv,16,16,cs,nn),yi.name="DFG_LUT",yi.minFilter=kt,yi.magFilter=kt,yi.wrapS=Hn,yi.wrapT=Hn,yi.generateMipmaps=!1,yi.needsUpdate=!0),yi}var sc=class{constructor(e={}){let{canvas:t=rf(),context:n=null,depth:i=!0,stencil:r=!1,alpha:o=!1,antialias:a=!1,premultipliedAlpha:l=!0,preserveDrawingBuffer:c=!1,powerPreference:h="default",failIfMajorPerformanceCaveat:u=!1,reversedDepthBuffer:d=!1,outputBufferType:f=Sn}=e;this.isWebGLRenderer=!0;let g;if(n!==null){if(typeof WebGLRenderingContext<"u"&&n instanceof WebGLRenderingContext)throw new Error("THREE.WebGLRenderer: WebGL 1 is not supported since r163.");g=n.getContextAttributes().alpha}else g=o;let y=f,m=new Set([yl,vl,_l]),p=new Set([Sn,ri,Rr,Cr,ml,gl]),b=new Uint32Array(4),S=new Int32Array(4),x=new B,A=null,T=null,I=[],v=[],D=null;this.domElement=t,this.debug={checkShaderErrors:!0,onShaderError:null},this.autoClear=!0,this.autoClearColor=!0,this.autoClearDepth=!0,this.autoClearStencil=!0,this.sortObjects=!0,this.clippingPlanes=[],this.localClippingEnabled=!1,this.toneMapping=ii,this.toneMappingExposure=1,this.transmissionResolutionScale=1;let w=this,R=!1,L=null,V=null,C=null,N=null;this._outputColorSpace=zt;let U=0,E=0,G=null,Y=-1,X=null,ie=new _t,ce=new _t,be=null,Le=new xe(0),Re=0,oe=t.width,ge=t.height,ae=1,Se=null,he=null,ue=new _t(0,0,oe,ge),H=new _t(0,0,oe,ge),Z=!1,W=new xr,J=!1,te=!1,pe=new Qe,we=new B,Pe=new _t,je={background:null,fog:null,environment:null,overrideMaterial:null,isScene:!0},Xe=!1;function Je(){return G===null?ae:1}let q=n;function it(F,Q){return t.getContext(F,Q)}try{let F={alpha:!0,depth:i,stencil:r,antialias:a,premultipliedAlpha:l,preserveDrawingBuffer:c,powerPreference:h,failIfMajorPerformanceCaveat:u};if("setAttribute"in t&&t.setAttribute("data-engine",`three.js r${"185"}`),t.addEventListener("webglcontextlost",Bt,!1),t.addEventListener("webglcontextrestored",Tt,!1),t.addEventListener("webglcontextcreationerror",An,!1),q===null){let Q="webgl2";if(q=it(Q,F),q===null)throw it(Q)?new Error("THREE.WebGLRenderer: Error creating WebGL context with your selected attributes."):new Error("THREE.WebGLRenderer: Error creating WebGL context.")}}catch(F){throw $e("WebGLRenderer: "+F.message),F}let Ue,M,_,O,k,P,z,$,j,ne,de,Ce,Me,ve,Be,Ee,Ve,K,ye,fe,Ae,Te,me;function Ge(){Ue=new kx(q),Ue.init(),Ae=new Tv(q,Ue),M=new Lx(q,Ue,e,Ae),_=new wv(q,Ue),M.reversedDepthBuffer&&d&&_.buffers.depth.setReversed(!0),V=q.createFramebuffer(),C=q.createFramebuffer(),N=q.createFramebuffer(),O=new Gx(q),k=new hv,P=new Ev(q,Ue,_,k,M,Ae,O),z=new zx(w),$=new Ym(q),Te=new Px(q,$),j=new Hx(q,$,O,Te),ne=new Xx(q,j,$,Te,O),K=new Wx(q,M,P),Be=new Dx(k),de=new cv(w,z,Ue,M,Te,Be),Ce=new Pv(w,k),Me=new dv,ve=new _v(Ue),Ve=new Cx(w,z,_,ne,g,l),Ee=new Sv(w,ne,M),me=new Iv(q,O,M,_),ye=new Ix(q,Ue,O),fe=new Vx(q,Ue,O),O.programs=de.programs,w.capabilities=M,w.extensions=Ue,w.properties=k,w.renderLists=Me,w.shadowMap=Ee,w.state=_,w.info=O}Ge(),y!==Sn&&(D=new Yx(y,t.width,t.height,a,i,r));let ze=new Wh(w,q);this.xr=ze,this.getContext=function(){return q},this.getContextAttributes=function(){return q.getContextAttributes()},this.forceContextLoss=function(){let F=Ue.get("WEBGL_lose_context");F&&F.loseContext()},this.forceContextRestore=function(){let F=Ue.get("WEBGL_lose_context");F&&F.restoreContext()},this.getPixelRatio=function(){return ae},this.setPixelRatio=function(F){F!==void 0&&(ae=F,this.setSize(oe,ge,!1))},this.getSize=function(F){return F.set(oe,ge)},this.setSize=function(F,Q,le=!0){if(ze.isPresenting){We("WebGLRenderer: Can't change size while VR device is presenting.");return}oe=F,ge=Q,t.width=Math.floor(F*ae),t.height=Math.floor(Q*ae),le===!0&&(t.style.width=F+"px",t.style.height=Q+"px"),D!==null&&D.setSize(t.width,t.height),this.setViewport(0,0,F,Q)},this.getDrawingBufferSize=function(F){return F.set(oe*ae,ge*ae).floor()},this.setDrawingBufferSize=function(F,Q,le){oe=F,ge=Q,ae=le,t.width=Math.floor(F*le),t.height=Math.floor(Q*le),this.setViewport(0,0,F,Q)},this.setEffects=function(F){if(y===Sn){$e("WebGLRenderer: setEffects() requires outputBufferType set to HalfFloatType or FloatType.");return}if(F){for(let Q=0;Q<F.length;Q++)if(F[Q].isOutputPass===!0){We("WebGLRenderer: OutputPass is not needed in setEffects(). Tone mapping and color space conversion are applied automatically.");break}}D.setEffects(F||[])},this.getCurrentViewport=function(F){return F.copy(ie)},this.getViewport=function(F){return F.copy(ue)},this.setViewport=function(F,Q,le,se){F.isVector4?ue.set(F.x,F.y,F.z,F.w):ue.set(F,Q,le,se),_.viewport(ie.copy(ue).multiplyScalar(ae).round())},this.getScissor=function(F){return F.copy(H)},this.setScissor=function(F,Q,le,se){F.isVector4?H.set(F.x,F.y,F.z,F.w):H.set(F,Q,le,se),_.scissor(ce.copy(H).multiplyScalar(ae).round())},this.getScissorTest=function(){return Z},this.setScissorTest=function(F){_.setScissorTest(Z=F)},this.setOpaqueSort=function(F){Se=F},this.setTransparentSort=function(F){he=F},this.getClearColor=function(F){return F.copy(Ve.getClearColor())},this.setClearColor=function(){Ve.setClearColor(...arguments)},this.getClearAlpha=function(){return Ve.getClearAlpha()},this.setClearAlpha=function(){Ve.setClearAlpha(...arguments)},this.clear=function(F=!0,Q=!0,le=!0){let se=0;if(F){let re=!1;if(G!==null){let Ne=G.texture.format;re=m.has(Ne)}if(re){let Ne=G.texture.type,He=p.has(Ne),De=Ve.getClearColor(),qe=Ve.getClearAlpha(),Ye=De.r,rt=De.g,ft=De.b;He?(b[0]=Ye,b[1]=rt,b[2]=ft,b[3]=qe,q.clearBufferuiv(q.COLOR,0,b)):(S[0]=Ye,S[1]=rt,S[2]=ft,S[3]=qe,q.clearBufferiv(q.COLOR,0,S))}else se|=q.COLOR_BUFFER_BIT}Q&&(se|=q.DEPTH_BUFFER_BIT,this.state.buffers.depth.setMask(!0)),le&&(se|=q.STENCIL_BUFFER_BIT,this.state.buffers.stencil.setMask(4294967295)),se!==0&&q.clear(se)},this.clearColor=function(){this.clear(!0,!1,!1)},this.clearDepth=function(){this.clear(!1,!0,!1)},this.clearStencil=function(){this.clear(!1,!1,!0)},this.setNodesHandler=function(F){F.setRenderer(this),L=F},this.dispose=function(){t.removeEventListener("webglcontextlost",Bt,!1),t.removeEventListener("webglcontextrestored",Tt,!1),t.removeEventListener("webglcontextcreationerror",An,!1),Ve.dispose(),Me.dispose(),ve.dispose(),k.dispose(),z.dispose(),ne.dispose(),Te.dispose(),me.dispose(),de.dispose(),ze.dispose(),ze.removeEventListener("sessionstart",st),ze.removeEventListener("sessionend",lt),tt.stop()};function Bt(F){F.preventDefault(),to("WebGLRenderer: Context Lost."),R=!0}function Tt(){to("WebGLRenderer: Context Restored."),R=!1;let F=O.autoReset,Q=Ee.enabled,le=Ee.autoUpdate,se=Ee.needsUpdate,re=Ee.type;Ge(),O.autoReset=F,Ee.enabled=Q,Ee.autoUpdate=le,Ee.needsUpdate=se,Ee.type=re}function An(F){$e("WebGLRenderer: A WebGL context could not be created. Reason: ",F.statusMessage)}function Rn(F){let Q=F.target;Q.removeEventListener("dispose",Rn),Sc(Q)}function Sc(F){wc(F),k.remove(F)}function wc(F){let Q=k.get(F).programs;Q!==void 0&&(Q.forEach(function(le){de.releaseProgram(le)}),F.isShaderMaterial&&de.releaseShaderCache(F))}this.renderBufferDirect=function(F,Q,le,se,re,Ne){Q===null&&(Q=je);let He=re.isMesh&&re.matrixWorld.determinantAffine()<0,De=ea(F,Q,le,se,re);_.setMaterial(se,He);let qe=le.index,Ye=1;if(se.wireframe===!0){if(qe=j.getWireframeAttribute(le),qe===void 0)return;Ye=2}let rt=le.drawRange,ft=le.attributes.position,Ze=rt.start*Ye,Rt=(rt.start+rt.count)*Ye;Ne!==null&&(Ze=Math.max(Ze,Ne.start*Ye),Rt=Math.min(Rt,(Ne.start+Ne.count)*Ye)),qe!==null?(Ze=Math.max(Ze,0),Rt=Math.min(Rt,qe.count)):ft!=null&&(Ze=Math.max(Ze,0),Rt=Math.min(Rt,ft.count));let qt=Rt-Ze;if(qt<0||qt===1/0)return;Te.setup(re,se,De,le,qe);let Gt,Lt=ye;if(qe!==null&&(Gt=$.get(qe),Lt=fe,Lt.setIndex(Gt)),re.isMesh)se.wireframe===!0?(_.setLineWidth(se.wireframeLinewidth*Je()),Lt.setMode(q.LINES)):Lt.setMode(q.TRIANGLES);else if(re.isLine){let cn=se.linewidth;cn===void 0&&(cn=1),_.setLineWidth(cn*Je()),re.isLineSegments?Lt.setMode(q.LINES):re.isLineLoop?Lt.setMode(q.LINE_LOOP):Lt.setMode(q.LINE_STRIP)}else re.isPoints?Lt.setMode(q.POINTS):re.isSprite&&Lt.setMode(q.TRIANGLES);if(re.isBatchedMesh)if(Ue.get("WEBGL_multi_draw"))Lt.renderMultiDraw(re._multiDrawStarts,re._multiDrawCounts,re._multiDrawCount);else{let cn=re._multiDrawStarts,ke=re._multiDrawCounts,Cn=re._multiDrawCount,yt=qe?$.get(qe).bytesPerElement:1,Bn=k.get(se).currentProgram.getUniforms();for(let li=0;li<Cn;li++)Bn.setValue(q,"_gl_DrawID",li),Lt.render(cn[li]/yt,ke[li])}else if(re.isInstancedMesh)Lt.renderInstances(Ze,qt,re.count);else if(le.isInstancedBufferGeometry){let cn=le._maxInstanceCount!==void 0?le._maxInstanceCount:1/0,ke=Math.min(le.instanceCount,cn);Lt.renderInstances(Ze,qt,ke)}else Lt.render(Ze,qt)};function Vi(F,Q,le){F.transparent===!0&&F.side===Nt&&F.forceSinglePass===!1?(F.side=tn,F.needsUpdate=!0,At(F,Q,le),F.side=Ln,F.needsUpdate=!0,At(F,Q,le),F.side=Nt):At(F,Q,le)}this.compile=function(F,Q,le=null){le===null&&(le=F),T=ve.get(le),T.init(Q),v.push(T),le.traverseVisible(function(re){re.isLight&&re.layers.test(Q.layers)&&(T.pushLight(re),re.castShadow&&T.pushShadow(re))}),F!==le&&F.traverseVisible(function(re){re.isLight&&re.layers.test(Q.layers)&&(T.pushLight(re),re.castShadow&&T.pushShadow(re))}),T.setupLights();let se=new Set;return F.traverse(function(re){if(!(re.isMesh||re.isPoints||re.isLine||re.isSprite))return;let Ne=re.material;if(Ne)if(Array.isArray(Ne))for(let He=0;He<Ne.length;He++){let De=Ne[He];Vi(De,le,re),se.add(De)}else Vi(Ne,le,re),se.add(Ne)}),T=v.pop(),se},this.compileAsync=function(F,Q,le=null){let se=this.compile(F,Q,le);return new Promise(re=>{function Ne(){if(se.forEach(function(He){k.get(He).currentProgram.isReady()&&se.delete(He)}),se.size===0){re(F);return}setTimeout(Ne,10)}Ue.get("KHR_parallel_shader_compile")!==null?Ne():setTimeout(Ne,10)})};let ee=null;function Oe(F){ee&&ee(F)}function st(){tt.stop()}function lt(){tt.start()}let tt=new Df;tt.setAnimationLoop(Oe),typeof self<"u"&&tt.setContext(self),this.setAnimationLoop=function(F){ee=F,ze.setAnimationLoop(F),F===null?tt.stop():tt.start()},ze.addEventListener("sessionstart",st),ze.addEventListener("sessionend",lt),this.render=function(F,Q){if(Q!==void 0&&Q.isCamera!==!0){$e("WebGLRenderer.render: camera is not an instance of THREE.Camera.");return}if(R===!0)return;L!==null&&L.renderStart(F,Q);let le=ze.enabled===!0&&ze.isPresenting===!0,se=D!==null&&(G===null||le)&&D.begin(w,G);if(F.matrixWorldAutoUpdate===!0&&F.updateMatrixWorld(),Q.parent===null&&Q.matrixWorldAutoUpdate===!0&&Q.updateMatrixWorld(),ze.enabled===!0&&ze.isPresenting===!0&&(D===null||D.isCompositing()===!1)&&(ze.cameraAutoUpdate===!0&&ze.updateCamera(Q),Q=ze.getCamera()),F.isScene===!0&&F.onBeforeRender(w,F,Q,G),T=ve.get(F,v.length),T.init(Q),T.state.textureUnits=P.getTextureUnits(),v.push(T),pe.multiplyMatrices(Q.projectionMatrix,Q.matrixWorldInverse),W.setFromProjectionMatrix(pe,Qn,Q.reversedDepth),te=this.localClippingEnabled,J=Be.init(this.clippingPlanes,te),A=Me.get(F,I.length),A.init(),I.push(A),ze.enabled===!0&&ze.isPresenting===!0){let He=w.xr.getDepthSensingMesh();He!==null&&Ht(He,Q,-1/0,w.sortObjects)}Ht(F,Q,0,w.sortObjects),A.finish(),w.sortObjects===!0&&A.sort(Se,he,Q.reversedDepth),Xe=ze.enabled===!1||ze.isPresenting===!1||ze.hasDepthSensing()===!1,Xe&&Ve.addToRenderList(A,F),this.info.render.frame++,this.info.autoReset===!0&&this.info.reset(),J===!0&&Be.beginShadows();let re=T.state.shadowsArray;if(Ee.render(re,F,Q),J===!0&&Be.endShadows(),(se&&D.hasRenderPass())===!1){let He=A.opaque,De=A.transmissive;if(T.setupLights(),Q.isArrayCamera){let qe=Q.cameras;if(De.length>0)for(let Ye=0,rt=qe.length;Ye<rt;Ye++){let ft=qe[Ye];Ft(He,De,F,ft)}Xe&&Ve.render(F);for(let Ye=0,rt=qe.length;Ye<rt;Ye++){let ft=qe[Ye];It(A,F,ft,ft.viewport)}}else De.length>0&&Ft(He,De,F,Q),Xe&&Ve.render(F),It(A,F,Q)}G!==null&&E===0&&(P.updateMultisampleRenderTarget(G),P.updateRenderTargetMipmap(G)),se&&D.end(w),F.isScene===!0&&F.onAfterRender(w,F,Q),Te.resetDefaultState(),Y=-1,X=null,v.pop(),v.length>0?(T=v[v.length-1],P.setTextureUnits(T.state.textureUnits),J===!0&&Be.setGlobalState(w.clippingPlanes,T.state.camera)):T=null,I.pop(),I.length>0?A=I[I.length-1]:A=null,L!==null&&L.renderEnd()};function Ht(F,Q,le,se){if(F.visible===!1)return;if(F.layers.test(Q.layers)){if(F.isGroup)le=F.renderOrder;else if(F.isLOD)F.autoUpdate===!0&&F.update(Q);else if(F.isLightProbeGrid)T.pushLightProbeGrid(F);else if(F.isLight)T.pushLight(F),F.castShadow&&T.pushShadow(F);else if(F.isSprite){if(!F.frustumCulled||W.intersectsSprite(F)){se&&Pe.setFromMatrixPosition(F.matrixWorld).applyMatrix4(pe);let He=ne.update(F),De=F.material;De.visible&&A.push(F,He,De,le,Pe.z,null)}}else if((F.isMesh||F.isLine||F.isPoints)&&(!F.frustumCulled||W.intersectsObject(F))){let He=ne.update(F),De=F.material;if(se&&(F.boundingSphere!==void 0?(F.boundingSphere===null&&F.computeBoundingSphere(),Pe.copy(F.boundingSphere.center)):(He.boundingSphere===null&&He.computeBoundingSphere(),Pe.copy(He.boundingSphere.center)),Pe.applyMatrix4(F.matrixWorld).applyMatrix4(pe)),Array.isArray(De)){let qe=He.groups;for(let Ye=0,rt=qe.length;Ye<rt;Ye++){let ft=qe[Ye],Ze=De[ft.materialIndex];Ze&&Ze.visible&&A.push(F,He,Ze,le,Pe.z,ft)}}else De.visible&&A.push(F,He,De,le,Pe.z,null)}}let Ne=F.children;for(let He=0,De=Ne.length;He<De;He++)Ht(Ne[He],Q,le,se)}function It(F,Q,le,se){let{opaque:re,transmissive:Ne,transparent:He}=F;T.setupLightsView(le),J===!0&&Be.setGlobalState(w.clippingPlanes,le),se&&_.viewport(ie.copy(se)),re.length>0&&vt(re,Q,le),Ne.length>0&&vt(Ne,Q,le),He.length>0&&vt(He,Q,le),_.buffers.depth.setTest(!0),_.buffers.depth.setMask(!0),_.buffers.color.setMask(!0),_.setPolygonOffset(!1)}function Ft(F,Q,le,se){if((le.isScene===!0?le.overrideMaterial:null)!==null)return;if(T.state.transmissionRenderTarget[se.id]===void 0){let Ze=Ue.has("EXT_color_buffer_half_float")||Ue.has("EXT_color_buffer_float");T.state.transmissionRenderTarget[se.id]=new Wt(1,1,{generateMipmaps:!0,type:Ze?nn:Sn,minFilter:si,samples:Math.max(4,M.samples),stencilBuffer:r,resolveDepthBuffer:!1,resolveStencilBuffer:!1,colorSpace:ot.workingColorSpace})}let Ne=T.state.transmissionRenderTarget[se.id],He=se.viewport||ie;Ne.setSize(He.z*w.transmissionResolutionScale,He.w*w.transmissionResolutionScale);let De=w.getRenderTarget(),qe=w.getActiveCubeFace(),Ye=w.getActiveMipmapLevel();w.setRenderTarget(Ne),w.getClearColor(Le),Re=w.getClearAlpha(),Re<1&&w.setClearColor(16777215,.5),w.clear(),Xe&&Ve.render(le);let rt=w.toneMapping;w.toneMapping=ii;let ft=se.viewport;if(se.viewport!==void 0&&(se.viewport=void 0),T.setupLightsView(se),J===!0&&Be.setGlobalState(w.clippingPlanes,se),vt(F,le,se),P.updateMultisampleRenderTarget(Ne),P.updateRenderTargetMipmap(Ne),Ue.has("WEBGL_multisampled_render_to_texture")===!1){let Ze=!1;for(let Rt=0,qt=Q.length;Rt<qt;Rt++){let Gt=Q[Rt],{object:Lt,geometry:cn,material:ke,group:Cn}=Gt;if(ke.side===Nt&&Lt.layers.test(se.layers)){let yt=ke.side;ke.side=tn,ke.needsUpdate=!0,Fe(Lt,le,se,cn,ke,Cn),ke.side=yt,ke.needsUpdate=!0,Ze=!0}}Ze===!0&&(P.updateMultisampleRenderTarget(Ne),P.updateRenderTargetMipmap(Ne))}w.setRenderTarget(De,qe,Ye),w.setClearColor(Le,Re),ft!==void 0&&(se.viewport=ft),w.toneMapping=rt}function vt(F,Q,le){let se=Q.isScene===!0?Q.overrideMaterial:null;for(let re=0,Ne=F.length;re<Ne;re++){let He=F[re],{object:De,geometry:qe,group:Ye}=He,rt=He.material;rt.allowOverride===!0&&se!==null&&(rt=se),De.layers.test(le.layers)&&Fe(De,Q,le,qe,rt,Ye)}}function Fe(F,Q,le,se,re,Ne){F.onBeforeRender(w,Q,le,se,re,Ne),F.modelViewMatrix.multiplyMatrices(le.matrixWorldInverse,F.matrixWorld),F.normalMatrix.getNormalMatrix(F.modelViewMatrix),re.onBeforeRender(w,Q,le,se,F,Ne),re.transparent===!0&&re.side===Nt&&re.forceSinglePass===!1?(re.side=tn,re.needsUpdate=!0,w.renderBufferDirect(le,Q,se,re,F,Ne),re.side=Ln,re.needsUpdate=!0,w.renderBufferDirect(le,Q,se,re,F,Ne),re.side=Nt):w.renderBufferDirect(le,Q,se,re,F,Ne),F.onAfterRender(w,Q,le,se,re,Ne)}function At(F,Q,le){Q.isScene!==!0&&(Q=je);let se=k.get(F),re=T.state.lights,Ne=T.state.shadowsArray,He=re.state.version,De=de.getParameters(F,re.state,Ne,Q,le,T.state.lightProbeGridArray),qe=de.getProgramCacheKey(De),Ye=se.programs;se.environment=F.isMeshStandardMaterial||F.isMeshLambertMaterial||F.isMeshPhongMaterial?Q.environment:null,se.fog=Q.fog;let rt=F.isMeshStandardMaterial||F.isMeshLambertMaterial&&!F.envMap||F.isMeshPhongMaterial&&!F.envMap;se.envMap=z.get(F.envMap||se.environment,rt),se.envMapRotation=se.environment!==null&&F.envMap===null?Q.environmentRotation:F.envMapRotation,Ye===void 0&&(F.addEventListener("dispose",Rn),Ye=new Map,se.programs=Ye);let ft=Ye.get(qe);if(ft!==void 0){if(se.currentProgram===ft&&se.lightsStateVersion===He)return dt(F,De),ft}else De.uniforms=de.getUniforms(F),L!==null&&F.isNodeMaterial&&L.build(F,le,De),F.onBeforeCompile(De,w),ft=de.acquireProgram(De,qe),Ye.set(qe,ft),se.uniforms=De.uniforms;let Ze=se.uniforms;return(!F.isShaderMaterial&&!F.isRawShaderMaterial||F.clipping===!0)&&(Ze.clippingPlanes=Be.uniform),dt(F,De),se.needsLights=Up(F),se.lightsStateVersion=He,se.needsLights&&(Ze.ambientLightColor.value=re.state.ambient,Ze.lightProbe.value=re.state.probe,Ze.directionalLights.value=re.state.directional,Ze.directionalLightShadows.value=re.state.directionalShadow,Ze.spotLights.value=re.state.spot,Ze.spotLightShadows.value=re.state.spotShadow,Ze.rectAreaLights.value=re.state.rectArea,Ze.ltc_1.value=re.state.rectAreaLTC1,Ze.ltc_2.value=re.state.rectAreaLTC2,Ze.pointLights.value=re.state.point,Ze.pointLightShadows.value=re.state.pointShadow,Ze.hemisphereLights.value=re.state.hemi,Ze.directionalShadowMatrix.value=re.state.directionalShadowMatrix,Ze.spotLightMatrix.value=re.state.spotLightMatrix,Ze.spotLightMap.value=re.state.spotLightMap,Ze.pointShadowMatrix.value=re.state.pointShadowMatrix),se.lightProbeGrid=T.state.lightProbeGridArray.length>0,se.currentProgram=ft,se.uniformsList=null,ft}function Ot(F){if(F.uniformsList===null){let Q=F.currentProgram.getUniforms();F.uniformsList=Lr.seqWithValue(Q.seq,F.uniforms)}return F.uniformsList}function dt(F,Q){let le=k.get(F);le.outputColorSpace=Q.outputColorSpace,le.batching=Q.batching,le.batchingColor=Q.batchingColor,le.instancing=Q.instancing,le.instancingColor=Q.instancingColor,le.instancingMorph=Q.instancingMorph,le.skinning=Q.skinning,le.morphTargets=Q.morphTargets,le.morphNormals=Q.morphNormals,le.morphColors=Q.morphColors,le.morphTargetsCount=Q.morphTargetsCount,le.numClippingPlanes=Q.numClippingPlanes,le.numIntersection=Q.numClipIntersection,le.vertexAlphas=Q.vertexAlphas,le.vertexTangents=Q.vertexTangents,le.toneMapping=Q.toneMapping}function Vt(F,Q){if(F.length===0)return null;if(F.length===1)return F[0].texture!==null?F[0]:null;x.setFromMatrixPosition(Q.matrixWorld);for(let le=0,se=F.length;le<se;le++){let re=F[le];if(re.texture!==null&&re.boundingBox.containsPoint(x))return re}return null}function ea(F,Q,le,se,re){Q.isScene!==!0&&(Q=je),P.resetTextureUnits();let Ne=Q.fog,He=se.isMeshStandardMaterial||se.isMeshLambertMaterial||se.isMeshPhongMaterial?Q.environment:null,De=G===null?w.outputColorSpace:G.isXRRenderTarget===!0?G.texture.colorSpace:ot.workingColorSpace,qe=se.isMeshStandardMaterial||se.isMeshLambertMaterial&&!se.envMap||se.isMeshPhongMaterial&&!se.envMap,Ye=z.get(se.envMap||He,qe),rt=se.vertexColors===!0&&!!le.attributes.color&&le.attributes.color.itemSize===4,ft=!!le.attributes.tangent&&(!!se.normalMap||se.anisotropy>0),Ze=!!le.morphAttributes.position,Rt=!!le.morphAttributes.normal,qt=!!le.morphAttributes.color,Gt=ii;se.toneMapped&&(G===null||G.isXRRenderTarget===!0)&&(Gt=w.toneMapping);let Lt=le.morphAttributes.position||le.morphAttributes.normal||le.morphAttributes.color,cn=Lt!==void 0?Lt.length:0,ke=k.get(se),Cn=T.state.lights;if(J===!0&&(te===!0||F!==X)){let Ut=F===X&&se.id===Y;Be.setState(se,F,Ut)}let yt=!1;se.version===ke.__version?(ke.needsLights&&ke.lightsStateVersion!==Cn.state.version||ke.outputColorSpace!==De||re.isBatchedMesh&&ke.batching===!1||!re.isBatchedMesh&&ke.batching===!0||re.isBatchedMesh&&ke.batchingColor===!0&&re.colorTexture===null||re.isBatchedMesh&&ke.batchingColor===!1&&re.colorTexture!==null||re.isInstancedMesh&&ke.instancing===!1||!re.isInstancedMesh&&ke.instancing===!0||re.isSkinnedMesh&&ke.skinning===!1||!re.isSkinnedMesh&&ke.skinning===!0||re.isInstancedMesh&&ke.instancingColor===!0&&re.instanceColor===null||re.isInstancedMesh&&ke.instancingColor===!1&&re.instanceColor!==null||re.isInstancedMesh&&ke.instancingMorph===!0&&re.morphTexture===null||re.isInstancedMesh&&ke.instancingMorph===!1&&re.morphTexture!==null||ke.envMap!==Ye||se.fog===!0&&ke.fog!==Ne||ke.numClippingPlanes!==void 0&&(ke.numClippingPlanes!==Be.numPlanes||ke.numIntersection!==Be.numIntersection)||ke.vertexAlphas!==rt||ke.vertexTangents!==ft||ke.morphTargets!==Ze||ke.morphNormals!==Rt||ke.morphColors!==qt||ke.toneMapping!==Gt||ke.morphTargetsCount!==cn||!!ke.lightProbeGrid!=T.state.lightProbeGridArray.length>0)&&(yt=!0):(yt=!0,ke.__version=se.version);let Bn=ke.currentProgram;yt===!0&&(Bn=At(se,Q,re),L&&se.isNodeMaterial&&L.onUpdateProgram(se,Bn,ke));let li=!1,Gi=!1,Ws=!1,Dt=Bn.getUniforms(),Yt=ke.uniforms;if(_.useProgram(Bn.program)&&(li=!0,Gi=!0,Ws=!0),se.id!==Y&&(Y=se.id,Gi=!0),ke.needsLights){let Ut=Vt(T.state.lightProbeGridArray,re);ke.lightProbeGrid!==Ut&&(ke.lightProbeGrid=Ut,Gi=!0)}if(li||X!==F){_.buffers.depth.getReversed()&&F.reversedDepth!==!0&&(F._reversedDepth=!0,F.updateProjectionMatrix()),Dt.setValue(q,"projectionMatrix",F.projectionMatrix),Dt.setValue(q,"viewMatrix",F.matrixWorldInverse);let Xi=Dt.map.cameraPosition;Xi!==void 0&&Xi.setValue(q,we.setFromMatrixPosition(F.matrixWorld)),M.logarithmicDepthBuffer&&Dt.setValue(q,"logDepthBufFC",2/(Math.log(F.far+1)/Math.LN2)),(se.isMeshPhongMaterial||se.isMeshToonMaterial||se.isMeshLambertMaterial||se.isMeshBasicMaterial||se.isMeshStandardMaterial||se.isShaderMaterial)&&Dt.setValue(q,"isOrthographic",F.isOrthographicCamera===!0),X!==F&&(X=F,Gi=!0,Ws=!0)}if(ke.needsLights&&(Cn.state.directionalShadowMap.length>0&&Dt.setValue(q,"directionalShadowMap",Cn.state.directionalShadowMap,P),Cn.state.spotShadowMap.length>0&&Dt.setValue(q,"spotShadowMap",Cn.state.spotShadowMap,P),Cn.state.pointShadowMap.length>0&&Dt.setValue(q,"pointShadowMap",Cn.state.pointShadowMap,P)),re.isSkinnedMesh){Dt.setOptional(q,re,"bindMatrix"),Dt.setOptional(q,re,"bindMatrixInverse");let Ut=re.skeleton;Ut&&(Ut.boneTexture===null&&Ut.computeBoneTexture(),Dt.setValue(q,"boneTexture",Ut.boneTexture,P))}re.isBatchedMesh&&(Dt.setOptional(q,re,"batchingTexture"),Dt.setValue(q,"batchingTexture",re._matricesTexture,P),Dt.setOptional(q,re,"batchingIdTexture"),Dt.setValue(q,"batchingIdTexture",re._indirectTexture,P),Dt.setOptional(q,re,"batchingColorTexture"),re._colorsTexture!==null&&Dt.setValue(q,"batchingColorTexture",re._colorsTexture,P));let Wi=le.morphAttributes;if((Wi.position!==void 0||Wi.normal!==void 0||Wi.color!==void 0)&&K.update(re,le,Bn),(Gi||ke.receiveShadow!==re.receiveShadow)&&(ke.receiveShadow=re.receiveShadow,Dt.setValue(q,"receiveShadow",re.receiveShadow)),(se.isMeshStandardMaterial||se.isMeshLambertMaterial||se.isMeshPhongMaterial)&&se.envMap===null&&Q.environment!==null&&(Yt.envMapIntensity.value=Q.environmentIntensity),Yt.dfgLUT!==void 0&&(Yt.dfgLUT.value=Dv()),Gi){if(Dt.setValue(q,"toneMappingExposure",w.toneMappingExposure),ke.needsLights&&Ei(Yt,Ws),Ne&&se.fog===!0&&Ce.refreshFogUniforms(Yt,Ne),Ce.refreshMaterialUniforms(Yt,se,ae,ge,T.state.transmissionRenderTarget[F.id]),ke.needsLights&&ke.lightProbeGrid){let Ut=ke.lightProbeGrid;Yt.probesSH.value=Ut.texture,Yt.probesMin.value.copy(Ut.boundingBox.min),Yt.probesMax.value.copy(Ut.boundingBox.max),Yt.probesResolution.value.copy(Ut.resolution)}Lr.upload(q,Ot(ke),Yt,P)}if(se.isShaderMaterial&&se.uniformsNeedUpdate===!0&&(Lr.upload(q,Ot(ke),Yt,P),se.uniformsNeedUpdate=!1),se.isSpriteMaterial&&Dt.setValue(q,"center",re.center),Dt.setValue(q,"modelViewMatrix",re.modelViewMatrix),Dt.setValue(q,"normalMatrix",re.normalMatrix),Dt.setValue(q,"modelMatrix",re.matrixWorld),se.uniformsGroups!==void 0){let Ut=se.uniformsGroups;for(let Xi=0,Xs=Ut.length;Xi<Xs;Xi++){let Uu=Ut[Xi];me.update(Uu,Bn),me.bind(Uu,Bn)}}return Bn}function Ei(F,Q){F.ambientLightColor.needsUpdate=Q,F.lightProbe.needsUpdate=Q,F.directionalLights.needsUpdate=Q,F.directionalLightShadows.needsUpdate=Q,F.pointLights.needsUpdate=Q,F.pointLightShadows.needsUpdate=Q,F.spotLights.needsUpdate=Q,F.spotLightShadows.needsUpdate=Q,F.rectAreaLights.needsUpdate=Q,F.hemisphereLights.needsUpdate=Q}function Up(F){return F.isMeshLambertMaterial||F.isMeshToonMaterial||F.isMeshPhongMaterial||F.isMeshStandardMaterial||F.isShadowMaterial||F.isShaderMaterial&&F.lights===!0}this.getActiveCubeFace=function(){return U},this.getActiveMipmapLevel=function(){return E},this.getRenderTarget=function(){return G},this.setRenderTargetTextures=function(F,Q,le){let se=k.get(F);se.__autoAllocateDepthBuffer=F.resolveDepthBuffer===!1,se.__autoAllocateDepthBuffer===!1&&(se.__useRenderToTexture=!1),k.get(F.texture).__webglTexture=Q,k.get(F.depthTexture).__webglTexture=se.__autoAllocateDepthBuffer?void 0:le,se.__hasExternalTextures=!0},this.setRenderTargetFramebuffer=function(F,Q){let le=k.get(F);le.__webglFramebuffer=Q,le.__useDefaultFramebuffer=Q===void 0},this.setRenderTarget=function(F,Q=0,le=0){G=F,U=Q,E=le;let se=null,re=!1,Ne=!1;if(F){let De=k.get(F);if(De.__useDefaultFramebuffer!==void 0){_.bindFramebuffer(q.FRAMEBUFFER,De.__webglFramebuffer),ie.copy(F.viewport),ce.copy(F.scissor),be=F.scissorTest,_.viewport(ie),_.scissor(ce),_.setScissorTest(be),Y=-1;return}else if(De.__webglFramebuffer===void 0)P.setupRenderTarget(F);else if(De.__hasExternalTextures)P.rebindTextures(F,k.get(F.texture).__webglTexture,k.get(F.depthTexture).__webglTexture);else if(F.depthBuffer){let rt=F.depthTexture;if(De.__boundDepthTexture!==rt){if(rt!==null&&k.has(rt)&&(F.width!==rt.image.width||F.height!==rt.image.height))throw new Error("THREE.WebGLRenderer: Attached DepthTexture is initialized to the incorrect size.");P.setupDepthRenderbuffer(F)}}let qe=F.texture;(qe.isData3DTexture||qe.isDataArrayTexture||qe.isCompressedArrayTexture)&&(Ne=!0);let Ye=k.get(F).__webglFramebuffer;F.isWebGLCubeRenderTarget?(Array.isArray(Ye[Q])?se=Ye[Q][le]:se=Ye[Q],re=!0):F.samples>0&&P.useMultisampledRTT(F)===!1?se=k.get(F).__webglMultisampledFramebuffer:Array.isArray(Ye)?se=Ye[le]:se=Ye,ie.copy(F.viewport),ce.copy(F.scissor),be=F.scissorTest}else ie.copy(ue).multiplyScalar(ae).floor(),ce.copy(H).multiplyScalar(ae).floor(),be=Z;if(le!==0&&(se=V),_.bindFramebuffer(q.FRAMEBUFFER,se)&&_.drawBuffers(F,se),_.viewport(ie),_.scissor(ce),_.setScissorTest(be),re){let De=k.get(F.texture);q.framebufferTexture2D(q.FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_CUBE_MAP_POSITIVE_X+Q,De.__webglTexture,le)}else if(Ne){let De=Q;for(let qe=0;qe<F.textures.length;qe++){let Ye=k.get(F.textures[qe]);q.framebufferTextureLayer(q.FRAMEBUFFER,q.COLOR_ATTACHMENT0+qe,Ye.__webglTexture,le,De)}}else if(F!==null&&le!==0){let De=k.get(F.texture);q.framebufferTexture2D(q.FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_2D,De.__webglTexture,le)}Y=-1},this.readRenderTargetPixels=function(F,Q,le,se,re,Ne,He,De=0){if(!(F&&F.isWebGLRenderTarget)){$e("WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");return}let qe=k.get(F).__webglFramebuffer;if(F.isWebGLCubeRenderTarget&&He!==void 0&&(qe=qe[He]),qe){_.bindFramebuffer(q.FRAMEBUFFER,qe);try{let Ye=F.textures[De],rt=Ye.format,ft=Ye.type;if(F.textures.length>1&&q.readBuffer(q.COLOR_ATTACHMENT0+De),!M.textureFormatReadable(rt)){$e("WebGLRenderer.readRenderTargetPixels: renderTarget is not in RGBA or implementation defined format.");return}if(!M.textureTypeReadable(ft)){$e("WebGLRenderer.readRenderTargetPixels: renderTarget is not in UnsignedByteType or implementation defined type.");return}Q>=0&&Q<=F.width-se&&le>=0&&le<=F.height-re&&q.readPixels(Q,le,se,re,Ae.convert(rt),Ae.convert(ft),Ne)}finally{let Ye=G!==null?k.get(G).__webglFramebuffer:null;_.bindFramebuffer(q.FRAMEBUFFER,Ye)}}},this.readRenderTargetPixelsAsync=async function(F,Q,le,se,re,Ne,He,De=0){if(!(F&&F.isWebGLRenderTarget))throw new Error("THREE.WebGLRenderer.readRenderTargetPixels: renderTarget is not THREE.WebGLRenderTarget.");let qe=k.get(F).__webglFramebuffer;if(F.isWebGLCubeRenderTarget&&He!==void 0&&(qe=qe[He]),qe)if(Q>=0&&Q<=F.width-se&&le>=0&&le<=F.height-re){_.bindFramebuffer(q.FRAMEBUFFER,qe);let Ye=F.textures[De],rt=Ye.format,ft=Ye.type;if(F.textures.length>1&&q.readBuffer(q.COLOR_ATTACHMENT0+De),!M.textureFormatReadable(rt))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in RGBA or implementation defined format.");if(!M.textureTypeReadable(ft))throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: renderTarget is not in UnsignedByteType or implementation defined type.");let Ze=q.createBuffer();q.bindBuffer(q.PIXEL_PACK_BUFFER,Ze),q.bufferData(q.PIXEL_PACK_BUFFER,Ne.byteLength,q.STREAM_READ),q.readPixels(Q,le,se,re,Ae.convert(rt),Ae.convert(ft),0);let Rt=G!==null?k.get(G).__webglFramebuffer:null;_.bindFramebuffer(q.FRAMEBUFFER,Rt);let qt=q.fenceSync(q.SYNC_GPU_COMMANDS_COMPLETE,0);return q.flush(),await af(q,qt,4),q.bindBuffer(q.PIXEL_PACK_BUFFER,Ze),q.getBufferSubData(q.PIXEL_PACK_BUFFER,0,Ne),q.deleteBuffer(Ze),q.deleteSync(qt),Ne}else throw new Error("THREE.WebGLRenderer.readRenderTargetPixelsAsync: requested read bounds are out of range.")},this.copyFramebufferToTexture=function(F,Q=null,le=0){let se=Math.pow(2,-le),re=Math.floor(F.image.width*se),Ne=Math.floor(F.image.height*se),He=Q!==null?Q.x:0,De=Q!==null?Q.y:0;P.setTexture2D(F,0),q.copyTexSubImage2D(q.TEXTURE_2D,le,0,0,He,De,re,Ne),_.unbindTexture()},this.copyTextureToTexture=function(F,Q,le=null,se=null,re=0,Ne=0){let He,De,qe,Ye,rt,ft,Ze,Rt,qt,Gt=F.isCompressedTexture?F.mipmaps[Ne]:F.image;if(le!==null)He=le.max.x-le.min.x,De=le.max.y-le.min.y,qe=le.isBox3?le.max.z-le.min.z:1,Ye=le.min.x,rt=le.min.y,ft=le.isBox3?le.min.z:0;else{let Yt=Math.pow(2,-re);He=Math.floor(Gt.width*Yt),De=Math.floor(Gt.height*Yt),F.isDataArrayTexture?qe=Gt.depth:F.isData3DTexture?qe=Math.floor(Gt.depth*Yt):qe=1,Ye=0,rt=0,ft=0}se!==null?(Ze=se.x,Rt=se.y,qt=se.z):(Ze=0,Rt=0,qt=0);let Lt=Ae.convert(Q.format),cn=Ae.convert(Q.type),ke;Q.isData3DTexture?(P.setTexture3D(Q,0),ke=q.TEXTURE_3D):Q.isDataArrayTexture||Q.isCompressedArrayTexture?(P.setTexture2DArray(Q,0),ke=q.TEXTURE_2D_ARRAY):(P.setTexture2D(Q,0),ke=q.TEXTURE_2D),_.activeTexture(q.TEXTURE0),_.pixelStorei(q.UNPACK_FLIP_Y_WEBGL,Q.flipY),_.pixelStorei(q.UNPACK_PREMULTIPLY_ALPHA_WEBGL,Q.premultiplyAlpha),_.pixelStorei(q.UNPACK_ALIGNMENT,Q.unpackAlignment);let Cn=_.getParameter(q.UNPACK_ROW_LENGTH),yt=_.getParameter(q.UNPACK_IMAGE_HEIGHT),Bn=_.getParameter(q.UNPACK_SKIP_PIXELS),li=_.getParameter(q.UNPACK_SKIP_ROWS),Gi=_.getParameter(q.UNPACK_SKIP_IMAGES);_.pixelStorei(q.UNPACK_ROW_LENGTH,Gt.width),_.pixelStorei(q.UNPACK_IMAGE_HEIGHT,Gt.height),_.pixelStorei(q.UNPACK_SKIP_PIXELS,Ye),_.pixelStorei(q.UNPACK_SKIP_ROWS,rt),_.pixelStorei(q.UNPACK_SKIP_IMAGES,ft);let Ws=F.isDataArrayTexture||F.isData3DTexture,Dt=Q.isDataArrayTexture||Q.isData3DTexture;if(F.isDepthTexture){let Yt=k.get(F),Wi=k.get(Q),Ut=k.get(Yt.__renderTarget),Xi=k.get(Wi.__renderTarget);_.bindFramebuffer(q.READ_FRAMEBUFFER,Ut.__webglFramebuffer),_.bindFramebuffer(q.DRAW_FRAMEBUFFER,Xi.__webglFramebuffer);for(let Xs=0;Xs<qe;Xs++)Ws&&(q.framebufferTextureLayer(q.READ_FRAMEBUFFER,q.COLOR_ATTACHMENT0,k.get(F).__webglTexture,re,ft+Xs),q.framebufferTextureLayer(q.DRAW_FRAMEBUFFER,q.COLOR_ATTACHMENT0,k.get(Q).__webglTexture,Ne,qt+Xs)),q.blitFramebuffer(Ye,rt,He,De,Ze,Rt,He,De,q.DEPTH_BUFFER_BIT,q.NEAREST);_.bindFramebuffer(q.READ_FRAMEBUFFER,null),_.bindFramebuffer(q.DRAW_FRAMEBUFFER,null)}else if(re!==0||F.isRenderTargetTexture||k.has(F)){let Yt=k.get(F),Wi=k.get(Q);_.bindFramebuffer(q.READ_FRAMEBUFFER,C),_.bindFramebuffer(q.DRAW_FRAMEBUFFER,N);for(let Ut=0;Ut<qe;Ut++)Ws?q.framebufferTextureLayer(q.READ_FRAMEBUFFER,q.COLOR_ATTACHMENT0,Yt.__webglTexture,re,ft+Ut):q.framebufferTexture2D(q.READ_FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_2D,Yt.__webglTexture,re),Dt?q.framebufferTextureLayer(q.DRAW_FRAMEBUFFER,q.COLOR_ATTACHMENT0,Wi.__webglTexture,Ne,qt+Ut):q.framebufferTexture2D(q.DRAW_FRAMEBUFFER,q.COLOR_ATTACHMENT0,q.TEXTURE_2D,Wi.__webglTexture,Ne),re!==0?q.blitFramebuffer(Ye,rt,He,De,Ze,Rt,He,De,q.COLOR_BUFFER_BIT,q.NEAREST):Dt?q.copyTexSubImage3D(ke,Ne,Ze,Rt,qt+Ut,Ye,rt,He,De):q.copyTexSubImage2D(ke,Ne,Ze,Rt,Ye,rt,He,De);_.bindFramebuffer(q.READ_FRAMEBUFFER,null),_.bindFramebuffer(q.DRAW_FRAMEBUFFER,null)}else Dt?F.isDataTexture||F.isData3DTexture?q.texSubImage3D(ke,Ne,Ze,Rt,qt,He,De,qe,Lt,cn,Gt.data):Q.isCompressedArrayTexture?q.compressedTexSubImage3D(ke,Ne,Ze,Rt,qt,He,De,qe,Lt,Gt.data):q.texSubImage3D(ke,Ne,Ze,Rt,qt,He,De,qe,Lt,cn,Gt):F.isDataTexture?q.texSubImage2D(q.TEXTURE_2D,Ne,Ze,Rt,He,De,Lt,cn,Gt.data):F.isCompressedTexture?q.compressedTexSubImage2D(q.TEXTURE_2D,Ne,Ze,Rt,Gt.width,Gt.height,Lt,Gt.data):q.texSubImage2D(q.TEXTURE_2D,Ne,Ze,Rt,He,De,Lt,cn,Gt);_.pixelStorei(q.UNPACK_ROW_LENGTH,Cn),_.pixelStorei(q.UNPACK_IMAGE_HEIGHT,yt),_.pixelStorei(q.UNPACK_SKIP_PIXELS,Bn),_.pixelStorei(q.UNPACK_SKIP_ROWS,li),_.pixelStorei(q.UNPACK_SKIP_IMAGES,Gi),Ne===0&&Q.generateMipmaps&&q.generateMipmap(ke),_.unbindTexture()},this.initRenderTarget=function(F){k.get(F).__webglFramebuffer===void 0&&P.setupRenderTarget(F)},this.initTexture=function(F){F.isCubeTexture?P.setTextureCube(F,0):F.isData3DTexture?P.setTexture3D(F,0):F.isDataArrayTexture||F.isCompressedArrayTexture?P.setTexture2DArray(F,0):P.setTexture2D(F,0),_.unbindTexture()},this.resetState=function(){U=0,E=0,G=null,_.reset(),Te.reset()},typeof __THREE_DEVTOOLS__<"u"&&__THREE_DEVTOOLS__.dispatchEvent(new CustomEvent("observe",{detail:this}))}get coordinateSystem(){return Qn}get outputColorSpace(){return this._outputColorSpace}set outputColorSpace(e){this._outputColorSpace=e;let t=this.getContext();t.drawingBufferColorSpace=ot._getDrawingBufferColorSpace(e),t.unpackColorSpace=ot._getUnpackColorSpace()}};function Xh(s,e){if(e===bh)return console.warn("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Geometry already defined as triangles."),s;if(e===Pr||e===Go){let t=s.getIndex();if(t===null){let o=[],a=s.getAttribute("position");if(a!==void 0){for(let l=0;l<a.count;l++)o.push(l);s.setIndex(o),t=s.getIndex()}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Undefined position attribute. Processing not possible."),s}let n=t.count-2,i=[];if(e===Pr)for(let o=1;o<=n;o++)i.push(t.getX(0)),i.push(t.getX(o)),i.push(t.getX(o+1));else for(let o=0;o<n;o++)o%2===0?(i.push(t.getX(o)),i.push(t.getX(o+1)),i.push(t.getX(o+2))):(i.push(t.getX(o+2)),i.push(t.getX(o+1)),i.push(t.getX(o)));i.length/3!==n&&console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unable to generate correct amount of triangles.");let r=s.clone();return r.setIndex(i),r.clearGroups(),r}else return console.error("THREE.BufferGeometryUtils.toTrianglesDrawMode(): Unknown draw mode:",e),s}function kf(s){let e=new Map,t=new Map,n=s.clone();return Hf(s,n,function(i,r){e.set(r,i),t.set(i,r)}),n.traverse(function(i){if(!i.isSkinnedMesh)return;let r=i,o=e.get(i),a=o.skeleton.bones;r.skeleton=o.skeleton.clone(),r.bindMatrix.copy(o.bindMatrix),r.skeleton.bones=a.map(function(l){return t.get(l)}),r.bind(r.skeleton,r.bindMatrix)}),n}function Hf(s,e,t){t(s,e);for(let n=0;n<s.children.length;n++)Hf(s.children[n],e.children[n],t)}var ds=class extends _i{constructor(e){super(e),this.dracoLoader=null,this.ktx2Loader=null,this.meshoptDecoder=null,this.pluginCallbacks=[],this.register(function(t){return new $h(t)}),this.register(function(t){return new Qh(t)}),this.register(function(t){return new lu(t)}),this.register(function(t){return new cu(t)}),this.register(function(t){return new hu(t)}),this.register(function(t){return new tu(t)}),this.register(function(t){return new nu(t)}),this.register(function(t){return new iu(t)}),this.register(function(t){return new su(t)}),this.register(function(t){return new Jh(t)}),this.register(function(t){return new ru(t)}),this.register(function(t){return new eu(t)}),this.register(function(t){return new au(t)}),this.register(function(t){return new ou(t)}),this.register(function(t){return new Kh(t)}),this.register(function(t){return new ac(t,gt.EXT_MESHOPT_COMPRESSION)}),this.register(function(t){return new ac(t,gt.KHR_MESHOPT_COMPRESSION)}),this.register(function(t){return new uu(t)})}load(e,t,n,i){let r=this,o;if(this.resourcePath!=="")o=this.resourcePath;else if(this.path!==""){let c=zi.extractUrlBase(e);o=zi.resolveURL(c,this.path)}else o=zi.extractUrlBase(e);this.manager.itemStart(e);let a=function(c){i?i(c):console.error(c),r.manager.itemError(e),r.manager.itemEnd(e)},l=new wr(this.manager);l.setPath(this.path),l.setResponseType("arraybuffer"),l.setRequestHeader(this.requestHeader),l.setWithCredentials(this.withCredentials),l.load(e,function(c){try{r.parse(c,o,function(h){t(h),r.manager.itemEnd(e)},a)}catch(h){a(h)}},n,a)}setDRACOLoader(e){return this.dracoLoader=e,this}setKTX2Loader(e){return this.ktx2Loader=e,this}setMeshoptDecoder(e){return this.meshoptDecoder=e,this}register(e){return this.pluginCallbacks.indexOf(e)===-1&&this.pluginCallbacks.push(e),this}unregister(e){return this.pluginCallbacks.indexOf(e)!==-1&&this.pluginCallbacks.splice(this.pluginCallbacks.indexOf(e),1),this}parse(e,t,n,i){let r,o={},a={},l=new TextDecoder;if(typeof e=="string")r=JSON.parse(e);else if(e instanceof ArrayBuffer)if(l.decode(new Uint8Array(e,0,4))===qf){try{o[gt.KHR_BINARY_GLTF]=new du(e)}catch(u){i&&i(u);return}r=JSON.parse(o[gt.KHR_BINARY_GLTF].content)}else r=JSON.parse(l.decode(e));else r=e;if(r.asset===void 0||r.asset.version[0]<2){i&&i(new Error("THREE.GLTFLoader: Unsupported asset. glTF versions >=2.0 are supported."));return}let c=new vu(r,{path:t||this.resourcePath||"",crossOrigin:this.crossOrigin,requestHeader:this.requestHeader,manager:this.manager,ktx2Loader:this.ktx2Loader,meshoptDecoder:this.meshoptDecoder});c.fileLoader.setRequestHeader(this.requestHeader);for(let h=0;h<this.pluginCallbacks.length;h++){let u=this.pluginCallbacks[h](c);u.name||console.error("THREE.GLTFLoader: Invalid plugin found: missing name"),a[u.name]=u,o[u.name]=!0}if(r.extensionsUsed)for(let h=0;h<r.extensionsUsed.length;++h){let u=r.extensionsUsed[h],d=r.extensionsRequired||[];switch(u){case gt.KHR_MATERIALS_UNLIT:o[u]=new jh;break;case gt.KHR_DRACO_MESH_COMPRESSION:o[u]=new fu(r,this.dracoLoader);break;case gt.KHR_TEXTURE_TRANSFORM:o[u]=new pu;break;case gt.KHR_MESH_QUANTIZATION:o[u]=new mu;break;default:d.indexOf(u)>=0&&a[u]===void 0&&console.warn('THREE.GLTFLoader: Unknown extension "'+u+'".')}}c.setExtensions(o),c.setPlugins(a),c.parse(n,i)}parseAsync(e,t){let n=this;return new Promise(function(i,r){n.parse(e,t,i,r)})}};function Nv(){let s={};return{get:function(e){return s[e]},add:function(e,t){s[e]=t},remove:function(e){delete s[e]},removeAll:function(){s={}}}}function Jt(s,e,t){let n=s.json.materials[e];return n.extensions&&n.extensions[t]?n.extensions[t]:null}var gt={KHR_BINARY_GLTF:"KHR_binary_glTF",KHR_DRACO_MESH_COMPRESSION:"KHR_draco_mesh_compression",KHR_LIGHTS_PUNCTUAL:"KHR_lights_punctual",KHR_MATERIALS_CLEARCOAT:"KHR_materials_clearcoat",KHR_MATERIALS_DISPERSION:"KHR_materials_dispersion",KHR_MATERIALS_IOR:"KHR_materials_ior",KHR_MATERIALS_SHEEN:"KHR_materials_sheen",KHR_MATERIALS_SPECULAR:"KHR_materials_specular",KHR_MATERIALS_TRANSMISSION:"KHR_materials_transmission",KHR_MATERIALS_IRIDESCENCE:"KHR_materials_iridescence",KHR_MATERIALS_ANISOTROPY:"KHR_materials_anisotropy",KHR_MATERIALS_UNLIT:"KHR_materials_unlit",KHR_MATERIALS_VOLUME:"KHR_materials_volume",KHR_TEXTURE_BASISU:"KHR_texture_basisu",KHR_TEXTURE_TRANSFORM:"KHR_texture_transform",KHR_MESH_QUANTIZATION:"KHR_mesh_quantization",KHR_MATERIALS_EMISSIVE_STRENGTH:"KHR_materials_emissive_strength",EXT_MATERIALS_BUMP:"EXT_materials_bump",EXT_TEXTURE_WEBP:"EXT_texture_webp",EXT_TEXTURE_AVIF:"EXT_texture_avif",EXT_MESHOPT_COMPRESSION:"EXT_meshopt_compression",KHR_MESHOPT_COMPRESSION:"KHR_meshopt_compression",EXT_MESH_GPU_INSTANCING:"EXT_mesh_gpu_instancing"},Kh=class{constructor(e){this.parser=e,this.name=gt.KHR_LIGHTS_PUNCTUAL,this.cache={refs:{},uses:{}}}_markDefs(){let e=this.parser,t=this.parser.json.nodes||[];for(let n=0,i=t.length;n<i;n++){let r=t[n];r.extensions&&r.extensions[this.name]&&r.extensions[this.name].light!==void 0&&e._addNodeRef(this.cache,r.extensions[this.name].light)}}_loadLight(e){let t=this.parser,n="light:"+e,i=t.cache.get(n);if(i)return i;let r=t.json,l=((r.extensions&&r.extensions[this.name]||{}).lights||[])[e],c,h=new xe(16777215);l.color!==void 0&&h.setRGB(l.color[0],l.color[1],l.color[2],gn);let u=l.range!==void 0?l.range:0;switch(l.type){case"directional":c=new ss(h),c.target.position.set(0,0,-1),c.add(c.target);break;case"point":c=new ni(h),c.distance=u;break;case"spot":c=new So(h),c.distance=u,l.spot=l.spot||{},l.spot.innerConeAngle=l.spot.innerConeAngle!==void 0?l.spot.innerConeAngle:0,l.spot.outerConeAngle=l.spot.outerConeAngle!==void 0?l.spot.outerConeAngle:Math.PI/4,c.angle=l.spot.outerConeAngle,c.penumbra=1-l.spot.innerConeAngle/l.spot.outerConeAngle,c.target.position.set(0,0,-1),c.add(c.target);break;default:throw new Error("THREE.GLTFLoader: Unexpected light type: "+l.type)}return c.position.set(0,0,0),bi(c,l),l.intensity!==void 0&&(c.intensity=l.intensity),c.name=t.createUniqueName(l.name||"light_"+e),i=Promise.resolve(c),t.cache.add(n,i),i}getDependency(e,t){if(e==="light")return this._loadLight(t)}createNodeAttachment(e){let t=this,n=this.parser,r=n.json.nodes[e],a=(r.extensions&&r.extensions[this.name]||{}).light;return a===void 0?null:this._loadLight(a).then(function(l){return n._getNodeRef(t.cache,a,l)})}},jh=class{constructor(){this.name=gt.KHR_MATERIALS_UNLIT}getMaterialType(){return bt}extendParams(e,t,n){let i=[];e.color=new xe(1,1,1),e.opacity=1;let r=t.pbrMetallicRoughness;if(r){if(Array.isArray(r.baseColorFactor)){let o=r.baseColorFactor;e.color.setRGB(o[0],o[1],o[2],gn),e.opacity=o[3]}r.baseColorTexture!==void 0&&i.push(n.assignTexture(e,"map",r.baseColorTexture,zt))}return Promise.all(i)}},Jh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_EMISSIVE_STRENGTH}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);return n===null||n.emissiveStrength!==void 0&&(t.emissiveIntensity=n.emissiveStrength),Promise.resolve()}},$h=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_CLEARCOAT}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(n.clearcoatFactor!==void 0&&(t.clearcoat=n.clearcoatFactor),n.clearcoatTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatMap",n.clearcoatTexture)),n.clearcoatRoughnessFactor!==void 0&&(t.clearcoatRoughness=n.clearcoatRoughnessFactor),n.clearcoatRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"clearcoatRoughnessMap",n.clearcoatRoughnessTexture)),n.clearcoatNormalTexture!==void 0&&(i.push(this.parser.assignTexture(t,"clearcoatNormalMap",n.clearcoatNormalTexture)),n.clearcoatNormalTexture.scale!==void 0)){let r=n.clearcoatNormalTexture.scale;t.clearcoatNormalScale=new _e(r,r)}return Promise.all(i)}},Qh=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_DISPERSION}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);return n===null||(t.dispersion=n.dispersion!==void 0?n.dispersion:0),Promise.resolve()}},eu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_IRIDESCENCE}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.iridescenceFactor!==void 0&&(t.iridescence=n.iridescenceFactor),n.iridescenceTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceMap",n.iridescenceTexture)),n.iridescenceIor!==void 0&&(t.iridescenceIOR=n.iridescenceIor),t.iridescenceThicknessRange===void 0&&(t.iridescenceThicknessRange=[100,400]),n.iridescenceThicknessMinimum!==void 0&&(t.iridescenceThicknessRange[0]=n.iridescenceThicknessMinimum),n.iridescenceThicknessMaximum!==void 0&&(t.iridescenceThicknessRange[1]=n.iridescenceThicknessMaximum),n.iridescenceThicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"iridescenceThicknessMap",n.iridescenceThicknessTexture)),Promise.all(i)}},tu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_SHEEN}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];if(t.sheenColor=new xe(0,0,0),t.sheenRoughness=0,t.sheen=1,n.sheenColorFactor!==void 0){let r=n.sheenColorFactor;t.sheenColor.setRGB(r[0],r[1],r[2],gn)}return n.sheenRoughnessFactor!==void 0&&(t.sheenRoughness=n.sheenRoughnessFactor),n.sheenColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenColorMap",n.sheenColorTexture,zt)),n.sheenRoughnessTexture!==void 0&&i.push(this.parser.assignTexture(t,"sheenRoughnessMap",n.sheenRoughnessTexture)),Promise.all(i)}},nu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_TRANSMISSION}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.transmissionFactor!==void 0&&(t.transmission=n.transmissionFactor),n.transmissionTexture!==void 0&&i.push(this.parser.assignTexture(t,"transmissionMap",n.transmissionTexture)),Promise.all(i)}},iu=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_VOLUME}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.thickness=n.thicknessFactor!==void 0?n.thicknessFactor:0,n.thicknessTexture!==void 0&&i.push(this.parser.assignTexture(t,"thicknessMap",n.thicknessTexture)),t.attenuationDistance=n.attenuationDistance||1/0;let r=n.attenuationColor||[1,1,1];return t.attenuationColor=new xe().setRGB(r[0],r[1],r[2],gn),Promise.all(i)}},su=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_IOR}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);return n===null||(t.ior=n.ior!==void 0?n.ior:1.5,t.ior===0&&(t.ior=1e3)),Promise.resolve()}},ru=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_SPECULAR}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];t.specularIntensity=n.specularFactor!==void 0?n.specularFactor:1,n.specularTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularIntensityMap",n.specularTexture));let r=n.specularColorFactor||[1,1,1];return t.specularColor=new xe().setRGB(r[0],r[1],r[2],gn),n.specularColorTexture!==void 0&&i.push(this.parser.assignTexture(t,"specularColorMap",n.specularColorTexture,zt)),Promise.all(i)}},ou=class{constructor(e){this.parser=e,this.name=gt.EXT_MATERIALS_BUMP}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return t.bumpScale=n.bumpFactor!==void 0?n.bumpFactor:1,n.bumpTexture!==void 0&&i.push(this.parser.assignTexture(t,"bumpMap",n.bumpTexture)),Promise.all(i)}},au=class{constructor(e){this.parser=e,this.name=gt.KHR_MATERIALS_ANISOTROPY}getMaterialType(e){return Jt(this.parser,e,this.name)!==null?Mn:null}extendMaterialParams(e,t){let n=Jt(this.parser,e,this.name);if(n===null)return Promise.resolve();let i=[];return n.anisotropyStrength!==void 0&&(t.anisotropy=n.anisotropyStrength),n.anisotropyRotation!==void 0&&(t.anisotropyRotation=n.anisotropyRotation),n.anisotropyTexture!==void 0&&i.push(this.parser.assignTexture(t,"anisotropyMap",n.anisotropyTexture)),Promise.all(i)}},lu=class{constructor(e){this.parser=e,this.name=gt.KHR_TEXTURE_BASISU}loadTexture(e){let t=this.parser,n=t.json,i=n.textures[e];if(!i.extensions||!i.extensions[this.name])return null;let r=i.extensions[this.name],o=t.options.ktx2Loader;if(!o){if(n.extensionsRequired&&n.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setKTX2Loader must be called before loading KTX2 textures");return null}return t.loadTextureImage(e,r.source,o)}},cu=class{constructor(e){this.parser=e,this.name=gt.EXT_TEXTURE_WEBP}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},hu=class{constructor(e){this.parser=e,this.name=gt.EXT_TEXTURE_AVIF}loadTexture(e){let t=this.name,n=this.parser,i=n.json,r=i.textures[e];if(!r.extensions||!r.extensions[t])return null;let o=r.extensions[t],a=i.images[o.source],l=n.textureLoader;if(a.uri){let c=n.options.manager.getHandler(a.uri);c!==null&&(l=c)}return n.loadTextureImage(e,o.source,l)}},ac=class{constructor(e,t){this.name=t,this.parser=e}loadBufferView(e){let t=this.parser.json,n=t.bufferViews[e];if(n.extensions&&n.extensions[this.name]){let i=n.extensions[this.name],r=this.parser.getDependency("buffer",i.buffer),o=this.parser.options.meshoptDecoder;if(!o||!o.supported){if(t.extensionsRequired&&t.extensionsRequired.indexOf(this.name)>=0)throw new Error("THREE.GLTFLoader: setMeshoptDecoder must be called before loading compressed files");return null}return r.then(function(a){let l=i.byteOffset||0,c=i.byteLength||0,h=i.count,u=i.byteStride,d=new Uint8Array(a,l,c);return o.decodeGltfBufferAsync?o.decodeGltfBufferAsync(h,u,d,i.mode,i.filter).then(function(f){return f.buffer}):o.ready.then(function(){let f=new ArrayBuffer(h*u);return o.decodeGltfBuffer(new Uint8Array(f),h,u,d,i.mode,i.filter),f})})}else return null}},uu=class{constructor(e){this.name=gt.EXT_MESH_GPU_INSTANCING,this.parser=e}createNodeMesh(e){let t=this.parser.json,n=t.nodes[e];if(!n.extensions||!n.extensions[this.name]||n.mesh===void 0)return null;let i=t.meshes[n.mesh];for(let c of i.primitives)if(c.mode!==Yn.TRIANGLES&&c.mode!==Yn.TRIANGLE_STRIP&&c.mode!==Yn.TRIANGLE_FAN&&c.mode!==void 0)return null;let o=n.extensions[this.name].attributes,a=[],l={};for(let c in o)a.push(this.parser.getDependency("accessor",o[c]).then(h=>(l[c]=h,l[c])));return a.length<1?null:(a.push(this.parser.createNodeMesh(e)),Promise.all(a).then(c=>{let h=c.pop(),u=h.isGroup?h.children:[h],d=c[0].count,f=[];for(let g of u){let y=new Qe,m=new B,p=new Zt,b=new B(1,1,1),S=new vn(g.geometry,g.material,d);for(let x=0;x<d;x++)l.TRANSLATION&&m.fromBufferAttribute(l.TRANSLATION,x),l.ROTATION&&p.fromBufferAttribute(l.ROTATION,x),l.SCALE&&b.fromBufferAttribute(l.SCALE,x),S.setMatrixAt(x,y.compose(m,p,b));for(let x in l)if(x==="_COLOR_0"){let A=l[x];S.instanceColor=new ts(A.array,A.itemSize,A.normalized)}else x!=="TRANSLATION"&&x!=="ROTATION"&&x!=="SCALE"&&g.geometry.setAttribute(x,l[x]);St.prototype.copy.call(S,g),this.parser.assignFinalMaterial(S),f.push(S)}return h.isGroup?(h.clear(),h.add(...f),h):f[0]}))}},qf="glTF",Zo=12,Vf={JSON:1313821514,BIN:5130562},du=class{constructor(e){this.name=gt.KHR_BINARY_GLTF,this.content=null,this.body=null;let t=new DataView(e,0,Zo),n=new TextDecoder;if(this.header={magic:n.decode(new Uint8Array(e.slice(0,4))),version:t.getUint32(4,!0),length:t.getUint32(8,!0)},this.header.magic!==qf)throw new Error("THREE.GLTFLoader: Unsupported glTF-Binary header.");if(this.header.version<2)throw new Error("THREE.GLTFLoader: Legacy binary file detected.");let i=this.header.length-Zo,r=new DataView(e,Zo),o=0;for(;o<i;){let a=r.getUint32(o,!0);o+=4;let l=r.getUint32(o,!0);if(o+=4,l===Vf.JSON){let c=new Uint8Array(e,Zo+o,a);this.content=n.decode(c)}else if(l===Vf.BIN){let c=Zo+o;this.body=e.slice(c,c+a)}o+=a}if(this.content===null)throw new Error("THREE.GLTFLoader: JSON content not found.")}},fu=class{constructor(e,t){if(!t)throw new Error("THREE.GLTFLoader: No DRACOLoader instance provided.");this.name=gt.KHR_DRACO_MESH_COMPRESSION,this.json=e,this.dracoLoader=t,this.dracoLoader.preload()}decodePrimitive(e,t){let n=this.json,i=this.dracoLoader,r=e.extensions[this.name].bufferView,o=e.extensions[this.name].attributes,a={},l={},c={};for(let h in o){let u=xu[h]||h.toLowerCase();a[u]=o[h]}for(let h in e.attributes){let u=xu[h]||h.toLowerCase();if(o[h]!==void 0){let d=n.accessors[e.attributes[h]],f=Ur[d.componentType];c[u]=f.name,l[u]=d.normalized===!0}}return t.getDependency("bufferView",r).then(function(h){return new Promise(function(u,d){i.decodeDracoFile(h,function(f){for(let g in f.attributes){let y=f.attributes[g],m=l[g];m!==void 0&&(y.normalized=m)}u(f)},a,c,gn,d)})})}},pu=class{constructor(){this.name=gt.KHR_TEXTURE_TRANSFORM}extendTexture(e,t){return(t.texCoord===void 0||t.texCoord===e.channel)&&t.offset===void 0&&t.rotation===void 0&&t.scale===void 0||(e=e.clone(),t.texCoord!==void 0&&(e.channel=t.texCoord),t.offset!==void 0&&e.offset.fromArray(t.offset),t.rotation!==void 0&&(e.rotation=t.rotation),t.scale!==void 0&&e.repeat.fromArray(t.scale),e.needsUpdate=!0),e}},mu=class{constructor(){this.name=gt.KHR_MESH_QUANTIZATION}},lc=class extends xi{constructor(e,t,n,i){super(e,t,n,i)}copySampleValue_(e){let t=this.resultBuffer,n=this.sampleValues,i=this.valueSize,r=e*i*3+i;for(let o=0;o!==i;o++)t[o]=n[r+o];return t}interpolate_(e,t,n,i){let r=this.resultBuffer,o=this.sampleValues,a=this.valueSize,l=a*2,c=a*3,h=i-t,u=(n-t)/h,d=u*u,f=d*u,g=e*c,y=g-c,m=-2*f+3*d,p=f-d,b=1-m,S=p-d+u;for(let x=0;x!==a;x++){let A=o[y+x+a],T=o[y+x+l]*h,I=o[g+x+a],v=o[g+x]*h;r[x]=b*A+S*T+m*I+p*v}return r}},Uv=new Zt,gu=class extends lc{interpolate_(e,t,n,i){let r=super.interpolate_(e,t,n,i);return Uv.fromArray(r).normalize().toArray(r),r}},Yn={FLOAT:5126,FLOAT_MAT3:35675,FLOAT_MAT4:35676,FLOAT_VEC2:35664,FLOAT_VEC3:35665,FLOAT_VEC4:35666,LINEAR:9729,REPEAT:10497,SAMPLER_2D:35678,POINTS:0,LINES:1,LINE_LOOP:2,LINE_STRIP:3,TRIANGLES:4,TRIANGLE_STRIP:5,TRIANGLE_FAN:6,UNSIGNED_BYTE:5121,UNSIGNED_SHORT:5123},Ur={5120:Int8Array,5121:Uint8Array,5122:Int16Array,5123:Uint16Array,5125:Uint32Array,5126:Float32Array},Gf={9728:Kt,9729:kt,9984:fl,9985:Ar,9986:Os,9987:si},Wf={33071:Hn,33648:ar,10497:ui},qh={SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT2:4,MAT3:9,MAT4:16},xu={POSITION:"position",NORMAL:"normal",TANGENT:"tangent",TEXCOORD_0:"uv",TEXCOORD_1:"uv1",TEXCOORD_2:"uv2",TEXCOORD_3:"uv3",COLOR_0:"color",WEIGHTS_0:"skinWeight",JOINTS_0:"skinIndex"},us={scale:"scale",translation:"position",rotation:"quaternion",weights:"morphTargetInfluences"},Fv={CUBICSPLINE:void 0,LINEAR:Ts,STEP:Es},Yh={OPAQUE:"OPAQUE",MASK:"MASK",BLEND:"BLEND"};function Ov(s){return s.DefaultMaterial===void 0&&(s.DefaultMaterial=new yn({color:16777215,emissive:0,metalness:1,roughness:1,transparent:!1,depthTest:!0,side:Ln})),s.DefaultMaterial}function ks(s,e,t){for(let n in t.extensions)s[n]===void 0&&(e.userData.gltfExtensions=e.userData.gltfExtensions||{},e.userData.gltfExtensions[n]=t.extensions[n])}function bi(s,e){e.extras!==void 0&&(typeof e.extras=="object"?Object.assign(s.userData,e.extras):console.warn("THREE.GLTFLoader: Ignoring primitive type .extras, "+e.extras))}function Bv(s,e,t){let n=!1,i=!1,r=!1;for(let c=0,h=e.length;c<h;c++){let u=e[c];if(u.POSITION!==void 0&&(n=!0),u.NORMAL!==void 0&&(i=!0),u.COLOR_0!==void 0&&(r=!0),n&&i&&r)break}if(!n&&!i&&!r)return Promise.resolve(s);let o=[],a=[],l=[];for(let c=0,h=e.length;c<h;c++){let u=e[c];if(n){let d=u.POSITION!==void 0?t.getDependency("accessor",u.POSITION):s.attributes.position;o.push(d)}if(i){let d=u.NORMAL!==void 0?t.getDependency("accessor",u.NORMAL):s.attributes.normal;a.push(d)}if(r){let d=u.COLOR_0!==void 0?t.getDependency("accessor",u.COLOR_0):s.attributes.color;l.push(d)}}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l)]).then(function(c){let h=c[0],u=c[1],d=c[2];return n&&(s.morphAttributes.position=h),i&&(s.morphAttributes.normal=u),r&&(s.morphAttributes.color=d),s.morphTargetsRelative=!0,s})}function zv(s,e){if(s.updateMorphTargets(),e.weights!==void 0)for(let t=0,n=e.weights.length;t<n;t++)s.morphTargetInfluences[t]=e.weights[t];if(e.extras&&Array.isArray(e.extras.targetNames)){let t=e.extras.targetNames;if(s.morphTargetInfluences.length===t.length){s.morphTargetDictionary={};for(let n=0,i=t.length;n<i;n++)s.morphTargetDictionary[t[n]]=n}else console.warn("THREE.GLTFLoader: Invalid extras.targetNames length. Ignoring names.")}}function kv(s){let e,t=s.extensions&&s.extensions[gt.KHR_DRACO_MESH_COMPRESSION];if(t?e="draco:"+t.bufferView+":"+t.indices+":"+Zh(t.attributes):e=s.indices+":"+Zh(s.attributes)+":"+s.mode,s.targets!==void 0)for(let n=0,i=s.targets.length;n<i;n++)e+=":"+Zh(s.targets[n]);return e}function Zh(s){let e="",t=Object.keys(s).sort();for(let n=0,i=t.length;n<i;n++)e+=t[n]+":"+s[t[n]]+";";return e}function _u(s){switch(s){case Int8Array:return 1/127;case Uint8Array:return 1/255;case Int16Array:return 1/32767;case Uint16Array:return 1/65535;default:throw new Error("THREE.GLTFLoader: Unsupported normalized accessor component type.")}}function Hv(s){return s.search(/\.jpe?g($|\?)/i)>0||s.search(/^data\:image\/jpeg/)===0?"image/jpeg":s.search(/\.webp($|\?)/i)>0||s.search(/^data\:image\/webp/)===0?"image/webp":s.search(/\.ktx2($|\?)/i)>0||s.search(/^data\:image\/ktx2/)===0?"image/ktx2":"image/png"}var Vv=new Qe,vu=class{constructor(e={},t={}){this.json=e,this.extensions={},this.plugins={},this.options=t,this.cache=new Nv,this.associations=new Map,this.primitiveCache={},this.nodeCache={},this.meshCache={refs:{},uses:{}},this.cameraCache={refs:{},uses:{}},this.lightCache={refs:{},uses:{}},this.sourceCache={},this.textureCache={},this.nodeNamesUsed={};let n=!1,i=-1,r=!1,o=-1;if(typeof navigator<"u"&&typeof navigator.userAgent<"u"){let a=navigator.userAgent;n=/^((?!chrome|android).)*safari/i.test(a)===!0;let l=a.match(/Version\/(\d+)/);i=n&&l?parseInt(l[1],10):-1,r=a.indexOf("Firefox")>-1,o=r?a.match(/Firefox\/([0-9]+)\./)[1]:-1}typeof createImageBitmap>"u"||n&&i<17||r&&o<98?this.textureLoader=new vo(this.options.manager):this.textureLoader=new wo(this.options.manager),this.textureLoader.setCrossOrigin(this.options.crossOrigin),this.textureLoader.setRequestHeader(this.options.requestHeader),this.fileLoader=new wr(this.options.manager),this.fileLoader.setResponseType("arraybuffer"),this.options.crossOrigin==="use-credentials"&&this.fileLoader.setWithCredentials(!0)}setExtensions(e){this.extensions=e}setPlugins(e){this.plugins=e}parse(e,t){let n=this,i=this.json,r=this.extensions;this.cache.removeAll(),this.nodeCache={},this._invokeAll(function(o){return o._markDefs&&o._markDefs()}),Promise.all(this._invokeAll(function(o){return o.beforeRoot&&o.beforeRoot()})).then(function(){return Promise.all([n.getDependencies("scene"),n.getDependencies("animation"),n.getDependencies("camera")])}).then(function(o){let a={scene:o[0][i.scene||0],scenes:o[0],animations:o[1],cameras:o[2],asset:i.asset,parser:n,userData:{}};return ks(r,a,i),bi(a,i),Promise.all(n._invokeAll(function(l){return l.afterRoot&&l.afterRoot(a)})).then(function(){for(let l of a.scenes)l.updateMatrixWorld();e(a)})}).catch(t)}_markDefs(){let e=this.json.nodes||[],t=this.json.skins||[],n=this.json.meshes||[];for(let i=0,r=t.length;i<r;i++){let o=t[i].joints;for(let a=0,l=o.length;a<l;a++)e[o[a]].isBone=!0}for(let i=0,r=e.length;i<r;i++){let o=e[i];o.mesh!==void 0&&(this._addNodeRef(this.meshCache,o.mesh),o.skin!==void 0&&(n[o.mesh].isSkinnedMesh=!0)),o.camera!==void 0&&this._addNodeRef(this.cameraCache,o.camera)}}_addNodeRef(e,t){t!==void 0&&(e.refs[t]===void 0&&(e.refs[t]=e.uses[t]=0),e.refs[t]++)}_getNodeRef(e,t,n){if(e.refs[t]<=1)return n;let i=n.clone(),r=(o,a)=>{let l=this.associations.get(o);l!=null&&this.associations.set(a,l);for(let[c,h]of o.children.entries())r(h,a.children[c])};return r(n,i),i.name+="_instance_"+e.uses[t]++,i}_invokeOne(e){let t=Object.values(this.plugins);t.push(this);for(let n=0;n<t.length;n++){let i=e(t[n]);if(i)return i}return null}_invokeAll(e){let t=Object.values(this.plugins);t.unshift(this);let n=[];for(let i=0;i<t.length;i++){let r=e(t[i]);r&&n.push(r)}return n}getDependency(e,t){let n=e+":"+t,i=this.cache.get(n);if(!i){switch(e){case"scene":i=this.loadScene(t);break;case"node":i=this._invokeOne(function(r){return r.loadNode&&r.loadNode(t)});break;case"mesh":i=this._invokeOne(function(r){return r.loadMesh&&r.loadMesh(t)});break;case"accessor":i=this.loadAccessor(t);break;case"bufferView":i=this._invokeOne(function(r){return r.loadBufferView&&r.loadBufferView(t)});break;case"buffer":i=this.loadBuffer(t);break;case"material":i=this._invokeOne(function(r){return r.loadMaterial&&r.loadMaterial(t)});break;case"texture":i=this._invokeOne(function(r){return r.loadTexture&&r.loadTexture(t)});break;case"skin":i=this.loadSkin(t);break;case"animation":i=this._invokeOne(function(r){return r.loadAnimation&&r.loadAnimation(t)});break;case"camera":i=this.loadCamera(t);break;default:if(i=this._invokeOne(function(r){return r!=this&&r.getDependency&&r.getDependency(e,t)}),!i)throw new Error("Unknown type: "+e);break}this.cache.add(n,i)}return i}getDependencies(e){let t=this.cache.get(e);if(!t){let n=this,i=this.json[e+(e==="mesh"?"es":"s")]||[];t=Promise.all(i.map(function(r,o){return n.getDependency(e,o)})),this.cache.add(e,t)}return t}loadBuffer(e){let t=this.json.buffers[e],n=this.fileLoader;if(t.type&&t.type!=="arraybuffer")throw new Error("THREE.GLTFLoader: "+t.type+" buffer type is not supported.");if(t.uri===void 0&&e===0)return Promise.resolve(this.extensions[gt.KHR_BINARY_GLTF].body);let i=this.options;return new Promise(function(r,o){n.load(zi.resolveURL(t.uri,i.path),r,void 0,function(){o(new Error('THREE.GLTFLoader: Failed to load buffer "'+t.uri+'".'))})})}loadBufferView(e){let t=this.json.bufferViews[e];return this.getDependency("buffer",t.buffer).then(function(n){let i=t.byteLength||0,r=t.byteOffset||0;return n.slice(r,r+i)})}loadAccessor(e){let t=this,n=this.json,i=this.json.accessors[e];if(i.bufferView===void 0&&i.sparse===void 0){let o=qh[i.type],a=Ur[i.componentType],l=i.normalized===!0,c=new a(i.count*o);return Promise.resolve(new xt(c,o,l))}let r=[];return i.bufferView!==void 0?r.push(this.getDependency("bufferView",i.bufferView)):r.push(null),i.sparse!==void 0&&(r.push(this.getDependency("bufferView",i.sparse.indices.bufferView)),r.push(this.getDependency("bufferView",i.sparse.values.bufferView))),Promise.all(r).then(function(o){let a=o[0],l=qh[i.type],c=Ur[i.componentType],h=c.BYTES_PER_ELEMENT,u=h*l,d=i.byteOffset||0,f=i.bufferView!==void 0?n.bufferViews[i.bufferView].byteStride:void 0,g=i.normalized===!0,y,m;if(f&&f!==u){let p=Math.floor(d/f),b="InterleavedBuffer:"+i.bufferView+":"+i.componentType+":"+p+":"+i.count,S=t.cache.get(b);S||(y=new c(a,p*f,i.count*f/h),S=new pr(y,f/h),t.cache.add(b,S)),m=new mr(S,l,d%f/h,g)}else a===null?y=new c(i.count*l):y=new c(a,d,i.count*l),m=new xt(y,l,g);if(i.sparse!==void 0){let p=qh.SCALAR,b=Ur[i.sparse.indices.componentType],S=i.sparse.indices.byteOffset||0,x=i.sparse.values.byteOffset||0,A=new b(o[1],S,i.sparse.count*p),T=new c(o[2],x,i.sparse.count*l);a!==null&&(m=new xt(m.array.slice(),m.itemSize,m.normalized)),m.normalized=!1;for(let I=0,v=A.length;I<v;I++){let D=A[I];if(m.setX(D,T[I*l]),l>=2&&m.setY(D,T[I*l+1]),l>=3&&m.setZ(D,T[I*l+2]),l>=4&&m.setW(D,T[I*l+3]),l>=5)throw new Error("THREE.GLTFLoader: Unsupported itemSize in sparse BufferAttribute.")}m.normalized=g}return m})}loadTexture(e){let t=this.json,n=this.options,r=t.textures[e].source,o=t.images[r],a=this.textureLoader;if(o.uri){let l=n.manager.getHandler(o.uri);l!==null&&(a=l)}return this.loadTextureImage(e,r,a)}loadTextureImage(e,t,n){let i=this,r=this.json,o=r.textures[e],a=r.images[t],l=(a.uri||a.bufferView)+":"+o.sampler;if(this.textureCache[l])return this.textureCache[l];let c=this.loadImageSource(t,n).then(function(h){h.flipY=!1,h.name=o.name||a.name||"",h.name===""&&typeof a.uri=="string"&&a.uri.startsWith("data:image/")===!1&&(h.name=a.uri);let d=(r.samplers||{})[o.sampler]||{};return h.magFilter=Gf[d.magFilter]||kt,h.minFilter=Gf[d.minFilter]||si,h.wrapS=Wf[d.wrapS]||ui,h.wrapT=Wf[d.wrapT]||ui,h.generateMipmaps=!h.isCompressedTexture&&h.minFilter!==Kt&&h.minFilter!==kt,i.associations.set(h,{textures:e}),h}).catch(function(){return null});return this.textureCache[l]=c,c}loadImageSource(e,t){let n=this,i=this.json,r=this.options;if(this.sourceCache[e]!==void 0)return this.sourceCache[e].then(u=>u.clone());let o=i.images[e],a=self.URL||self.webkitURL,l=o.uri||"",c=!1;if(o.bufferView!==void 0)l=n.getDependency("bufferView",o.bufferView).then(function(u){c=!0;let d=new Blob([u],{type:o.mimeType});return l=a.createObjectURL(d),l});else if(o.uri===void 0)throw new Error("THREE.GLTFLoader: Image "+e+" is missing URI and bufferView");let h=Promise.resolve(l).then(function(u){return new Promise(function(d,f){let g=d;t.isImageBitmapLoader===!0&&(g=function(y){let m=new jt(y);m.needsUpdate=!0,d(m)}),t.load(zi.resolveURL(u,r.path),g,void 0,f)})}).then(function(u){return c===!0&&a.revokeObjectURL(l),bi(u,o),u.userData.mimeType=o.mimeType||Hv(o.uri),u}).catch(function(u){throw console.error("THREE.GLTFLoader: Couldn't load texture",l),u});return this.sourceCache[e]=h,h}assignTexture(e,t,n,i){let r=this;return this.getDependency("texture",n.index).then(function(o){if(!o)return null;if(n.texCoord!==void 0&&n.texCoord>0&&(o=o.clone(),o.channel=n.texCoord),r.extensions[gt.KHR_TEXTURE_TRANSFORM]){let a=n.extensions!==void 0?n.extensions[gt.KHR_TEXTURE_TRANSFORM]:void 0;if(a){let l=r.associations.get(o);o=r.extensions[gt.KHR_TEXTURE_TRANSFORM].extendTexture(o,a),r.associations.set(o,l)}}return i!==void 0&&(o.colorSpace=i),e[t]=o,o})}assignFinalMaterial(e){let t=e.geometry,n=e.material,i=t.attributes.tangent===void 0,r=t.attributes.color!==void 0,o=t.attributes.normal===void 0;if(e.isPoints){let a="PointsMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new _r,xn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,l.sizeAttenuation=!1,this.cache.add(a,l)),n=l}else if(e.isLine){let a="LineBasicMaterial:"+n.uuid,l=this.cache.get(a);l||(l=new pi,xn.prototype.copy.call(l,n),l.color.copy(n.color),l.map=n.map,this.cache.add(a,l)),n=l}if(i||r||o){let a="ClonedMaterial:"+n.uuid+":";i&&(a+="derivative-tangents:"),r&&(a+="vertex-colors:"),o&&(a+="flat-shading:");let l=this.cache.get(a);l||(l=n.clone(),r&&(l.vertexColors=!0),o&&(l.flatShading=!0),i&&(l.normalScale&&(l.normalScale.y*=-1),l.clearcoatNormalScale&&(l.clearcoatNormalScale.y*=-1)),this.cache.add(a,l),this.associations.set(l,this.associations.get(n))),n=l}e.material=n}getMaterialType(){return yn}loadMaterial(e){let t=this,n=this.json,i=this.extensions,r=n.materials[e],o,a={},l=r.extensions||{},c=[];if(l[gt.KHR_MATERIALS_UNLIT]){let u=i[gt.KHR_MATERIALS_UNLIT];o=u.getMaterialType(),c.push(u.extendParams(a,r,t))}else{let u=r.pbrMetallicRoughness||{};if(a.color=new xe(1,1,1),a.opacity=1,Array.isArray(u.baseColorFactor)){let d=u.baseColorFactor;a.color.setRGB(d[0],d[1],d[2],gn),a.opacity=d[3]}u.baseColorTexture!==void 0&&c.push(t.assignTexture(a,"map",u.baseColorTexture,zt)),a.metalness=u.metallicFactor!==void 0?u.metallicFactor:1,a.roughness=u.roughnessFactor!==void 0?u.roughnessFactor:1,u.metallicRoughnessTexture!==void 0&&(c.push(t.assignTexture(a,"metalnessMap",u.metallicRoughnessTexture)),c.push(t.assignTexture(a,"roughnessMap",u.metallicRoughnessTexture))),o=this._invokeOne(function(d){return d.getMaterialType&&d.getMaterialType(e)}),c.push(Promise.all(this._invokeAll(function(d){return d.extendMaterialParams&&d.extendMaterialParams(e,a)})))}r.doubleSided===!0&&(a.side=Nt);let h=r.alphaMode||Yh.OPAQUE;if(h===Yh.BLEND?(a.transparent=!0,a.depthWrite=!1):(a.transparent=!1,h===Yh.MASK&&(a.alphaTest=r.alphaCutoff!==void 0?r.alphaCutoff:.5)),r.normalTexture!==void 0&&o!==bt&&(c.push(t.assignTexture(a,"normalMap",r.normalTexture)),a.normalScale=new _e(1,1),r.normalTexture.scale!==void 0)){let u=r.normalTexture.scale;a.normalScale.set(u,u)}if(r.occlusionTexture!==void 0&&o!==bt&&(c.push(t.assignTexture(a,"aoMap",r.occlusionTexture)),r.occlusionTexture.strength!==void 0&&(a.aoMapIntensity=r.occlusionTexture.strength)),r.emissiveFactor!==void 0&&o!==bt){let u=r.emissiveFactor;a.emissive=new xe().setRGB(u[0],u[1],u[2],gn)}return r.emissiveTexture!==void 0&&o!==bt&&c.push(t.assignTexture(a,"emissiveMap",r.emissiveTexture,zt)),Promise.all(c).then(function(){let u=new o(a);return r.name&&(u.name=r.name),bi(u,r),t.associations.set(u,{materials:e}),r.extensions&&ks(i,u,r),u})}createUniqueName(e){let t=Ct.sanitizeNodeName(e||"");return t in this.nodeNamesUsed?t+"_"+ ++this.nodeNamesUsed[t]:(this.nodeNamesUsed[t]=0,t)}loadGeometries(e){let t=this,n=this.extensions,i=this.primitiveCache;function r(a){return n[gt.KHR_DRACO_MESH_COMPRESSION].decodePrimitive(a,t).then(function(l){return Xf(l,a,t)})}let o=[];for(let a=0,l=e.length;a<l;a++){let c=e[a],h=kv(c),u=i[h];if(u)o.push(u.promise);else{let d;c.extensions&&c.extensions[gt.KHR_DRACO_MESH_COMPRESSION]?d=r(c):d=Xf(new pt,c,t),i[h]={primitive:c,promise:d},o.push(d)}}return Promise.all(o)}loadMesh(e){let t=this,n=this.json,i=this.extensions,r=n.meshes[e],o=r.primitives,a=[];for(let l=0,c=o.length;l<c;l++){let h=o[l].material===void 0?Ov(this.cache):this.getDependency("material",o[l].material);a.push(h)}return a.push(t.loadGeometries(o)),Promise.all(a).then(function(l){let c=l.slice(0,l.length-1),h=l[l.length-1],u=[];for(let f=0,g=h.length;f<g;f++){let y=h[f],m=o[f],p,b=c[f];if(m.mode===Yn.TRIANGLES||m.mode===Yn.TRIANGLE_STRIP||m.mode===Yn.TRIANGLE_FAN||m.mode===void 0)p=r.isSkinnedMesh===!0?new oo(y,b):new Ke(y,b),p.isSkinnedMesh===!0&&p.normalizeSkinWeights(),m.mode===Yn.TRIANGLE_STRIP?p.geometry=Xh(p.geometry,Go):m.mode===Yn.TRIANGLE_FAN&&(p.geometry=Xh(p.geometry,Pr));else if(m.mode===Yn.LINES)p=new Cs(y,b);else if(m.mode===Yn.LINE_STRIP)p=new Di(y,b);else if(m.mode===Yn.LINE_LOOP)p=new lo(y,b);else if(m.mode===Yn.POINTS)p=new _n(y,b);else throw new Error("THREE.GLTFLoader: Primitive mode unsupported: "+m.mode);Object.keys(p.geometry.morphAttributes).length>0&&zv(p,r),p.name=t.createUniqueName(r.name||"mesh_"+e),bi(p,r),m.extensions&&ks(i,p,m),t.assignFinalMaterial(p),u.push(p)}for(let f=0,g=u.length;f<g;f++)t.associations.set(u[f],{meshes:e,primitives:f});if(u.length===1)return r.extensions&&ks(i,u[0],r),u[0];let d=new ct;r.extensions&&ks(i,d,r),t.associations.set(d,{meshes:e});for(let f=0,g=u.length;f<g;f++)d.add(u[f]);return d})}loadCamera(e){let t,n=this.json.cameras[e],i=n[n.type];if(!i){console.warn("THREE.GLTFLoader: Missing camera parameters.");return}return n.type==="perspective"?t=new Qt(wt.radToDeg(i.yfov),i.aspectRatio||1,i.znear||1,i.zfar||2e6):n.type==="orthographic"&&(t=new vi(-i.xmag,i.xmag,i.ymag,-i.ymag,i.znear,i.zfar)),n.name&&(t.name=this.createUniqueName(n.name)),bi(t,n),Promise.resolve(t)}loadSkin(e){let t=this.json.skins[e],n=[];for(let i=0,r=t.joints.length;i<r;i++)n.push(this._loadNodeShallow(t.joints[i]));return t.inverseBindMatrices!==void 0?n.push(this.getDependency("accessor",t.inverseBindMatrices)):n.push(null),Promise.all(n).then(function(i){let r=i.pop(),o=i,a=[],l=[];for(let c=0,h=o.length;c<h;c++){let u=o[c];if(u){a.push(u);let d=new Qe;r!==null&&d.fromArray(r.array,c*16),l.push(d)}else console.warn('THREE.GLTFLoader: Joint "%s" could not be found.',t.joints[c])}return new ao(a,l)})}loadAnimation(e){let t=this.json,n=this,i=t.animations[e],r=i.name?i.name:"animation_"+e,o=[],a=[],l=[],c=[],h=[];for(let u=0,d=i.channels.length;u<d;u++){let f=i.channels[u],g=i.samplers[f.sampler],y=f.target,m=y.node,p=i.parameters!==void 0?i.parameters[g.input]:g.input,b=i.parameters!==void 0?i.parameters[g.output]:g.output;y.node!==void 0&&(o.push(this.getDependency("node",m)),a.push(this.getDependency("accessor",p)),l.push(this.getDependency("accessor",b)),c.push(g),h.push(y))}return Promise.all([Promise.all(o),Promise.all(a),Promise.all(l),Promise.all(c),Promise.all(h)]).then(function(u){let d=u[0],f=u[1],g=u[2],y=u[3],m=u[4],p=[];for(let S=0,x=d.length;S<x;S++){let A=d[S],T=f[S],I=g[S],v=y[S],D=m[S];if(A===void 0)continue;A.updateMatrix&&A.updateMatrix();let w=n._createAnimationTracks(A,T,I,v,D);if(w)for(let R=0;R<w.length;R++)p.push(w[R])}let b=new Ds(r,void 0,p);return bi(b,i),b})}createNodeMesh(e){let t=this.json,n=this,i=t.nodes[e];return i.mesh===void 0?null:n.getDependency("mesh",i.mesh).then(function(r){let o=n._getNodeRef(n.meshCache,i.mesh,r);return i.weights!==void 0&&o.traverse(function(a){if(a.isMesh)for(let l=0,c=i.weights.length;l<c;l++)a.morphTargetInfluences[l]=i.weights[l]}),o})}loadNode(e){let t=this.json,n=this,i=t.nodes[e],r=n._loadNodeShallow(e),o=[],a=i.children||[];for(let c=0,h=a.length;c<h;c++)o.push(n.getDependency("node",a[c]));let l=i.skin===void 0?Promise.resolve(null):n.getDependency("skin",i.skin);return Promise.all([r,Promise.all(o),l]).then(function(c){let h=c[0],u=c[1],d=c[2];d!==null&&h.traverse(function(f){f.isSkinnedMesh&&f.bind(d,Vv)});for(let f=0,g=u.length;f<g;f++)h.add(u[f]);if(h.userData.pivot!==void 0&&u.length>0){let f=h.userData.pivot,g=u[0];h.pivot=new B().fromArray(f),h.position.x-=f[0],h.position.y-=f[1],h.position.z-=f[2],g.position.set(0,0,0),delete h.userData.pivot}return h})}_loadNodeShallow(e){let t=this.json,n=this.extensions,i=this;if(this.nodeCache[e]!==void 0)return this.nodeCache[e];let r=t.nodes[e],o=r.name?i.createUniqueName(r.name):"",a=[],l=i._invokeOne(function(c){return c.createNodeMesh&&c.createNodeMesh(e)});return l&&a.push(l),r.camera!==void 0&&a.push(i.getDependency("camera",r.camera).then(function(c){return i._getNodeRef(i.cameraCache,r.camera,c)})),i._invokeAll(function(c){return c.createNodeAttachment&&c.createNodeAttachment(e)}).forEach(function(c){a.push(c)}),this.nodeCache[e]=Promise.all(a).then(function(c){let h;if(r.isBone===!0?h=new gr:c.length>1?h=new ct:c.length===1?h=c[0]:h=new St,h!==c[0])for(let u=0,d=c.length;u<d;u++)h.add(c[u]);if(r.name&&(h.userData.name=r.name,h.name=o),bi(h,r),r.extensions&&ks(n,h,r),r.matrix!==void 0){let u=new Qe;u.fromArray(r.matrix),h.applyMatrix4(u)}else r.translation!==void 0&&h.position.fromArray(r.translation),r.rotation!==void 0&&h.quaternion.fromArray(r.rotation),r.scale!==void 0&&h.scale.fromArray(r.scale);if(!i.associations.has(h))i.associations.set(h,{});else if(r.mesh!==void 0&&i.meshCache.refs[r.mesh]>1){let u=i.associations.get(h);i.associations.set(h,{...u})}return i.associations.get(h).nodes=e,h}),this.nodeCache[e]}loadScene(e){let t=this.extensions,n=this.json.scenes[e],i=this,r=new ct;n.name&&(r.name=i.createUniqueName(n.name)),bi(r,n),n.extensions&&ks(t,r,n);let o=n.nodes||[],a=[];for(let l=0,c=o.length;l<c;l++)a.push(i.getDependency("node",o[l]));return Promise.all(a).then(function(l){for(let h=0,u=l.length;h<u;h++){let d=l[h];d.parent!==null?r.add(kf(d)):r.add(d)}let c=h=>{let u=new Map;for(let[d,f]of i.associations)(d instanceof xn||d instanceof jt)&&u.set(d,f);return h.traverse(d=>{let f=i.associations.get(d);f!=null&&u.set(d,f)}),u};return i.associations=c(r),r})}_createAnimationTracks(e,t,n,i,r){let o=[],a=e.name?e.name:e.uuid,l=[];function c(f){f.morphTargetInfluences&&l.push(f.name?f.name:f.uuid)}us[r.path]===us.weights?(c(e),e.isGroup&&e.children.forEach(c)):l.push(a);let h;switch(us[r.path]){case us.weights:h=Fi;break;case us.rotation:h=Oi;break;case us.translation:case us.scale:h=is;break;default:switch(n.itemSize){case 1:h=Fi;break;case 2:case 3:default:h=is;break}break}let u=i.interpolation!==void 0?Fv[i.interpolation]:Ts,d=this._getArrayFromAccessor(n);for(let f=0,g=l.length;f<g;f++){let y=new h(l[f]+"."+us[r.path],t.array,d,u);i.interpolation==="CUBICSPLINE"&&this._createCubicSplineTrackInterpolant(y),o.push(y)}return o}_getArrayFromAccessor(e){let t=e.array;if(e.normalized){let n=_u(t.constructor),i=new Float32Array(t.length);for(let r=0,o=t.length;r<o;r++)i[r]=t[r]*n;t=i}return t}_createCubicSplineTrackInterpolant(e){e.createInterpolant=function(n){let i=this instanceof Oi?gu:lc;return new i(this.times,this.values,this.getValueSize()/3,n)},e.createInterpolant.isInterpolantFactoryMethodGLTFCubicSpline=!0}};function Gv(s,e,t){let n=e.attributes,i=new en;if(n.POSITION!==void 0){let a=t.json.accessors[n.POSITION],l=a.min,c=a.max;if(l!==void 0&&c!==void 0){if(i.set(new B(l[0],l[1],l[2]),new B(c[0],c[1],c[2])),a.normalized){let h=_u(Ur[a.componentType]);i.min.multiplyScalar(h),i.max.multiplyScalar(h)}}else{console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.");return}}else return;let r=e.targets;if(r!==void 0){let a=new B,l=new B;for(let c=0,h=r.length;c<h;c++){let u=r[c];if(u.POSITION!==void 0){let d=t.json.accessors[u.POSITION],f=d.min,g=d.max;if(f!==void 0&&g!==void 0){if(l.setX(Math.max(Math.abs(f[0]),Math.abs(g[0]))),l.setY(Math.max(Math.abs(f[1]),Math.abs(g[1]))),l.setZ(Math.max(Math.abs(f[2]),Math.abs(g[2]))),d.normalized){let y=_u(Ur[d.componentType]);l.multiplyScalar(y)}a.max(l)}else console.warn("THREE.GLTFLoader: Missing min/max properties for accessor POSITION.")}}i.expandByVector(a)}s.boundingBox=i;let o=new dn;i.getCenter(o.center),o.radius=i.min.distanceTo(i.max)/2,s.boundingSphere=o}function Xf(s,e,t){let n=e.attributes,i=[];function r(o,a){return t.getDependency("accessor",o).then(function(l){s.setAttribute(a,l)})}for(let o in n){let a=xu[o]||o.toLowerCase();a in s.attributes||i.push(r(n[o],a))}if(e.indices!==void 0&&!s.index){let o=t.getDependency("accessor",e.indices).then(function(a){s.setIndex(a)});i.push(o)}return ot.workingColorSpace!==gn&&"COLOR_0"in n&&console.warn(`THREE.GLTFLoader: Converting vertex colors from "srgb-linear" to "${ot.workingColorSpace}" not supported.`),bi(s,e),Gv(s,e,t),Promise.all(i).then(function(){return e.targets!==void 0?Bv(s,e.targets,t):s})}var Yf={type:"change"},Mu={type:"start"},Kf={type:"end"},cc=new fi,Zf=new kn,Wv=Math.cos(70*wt.DEG2RAD),on=new B,wn=2*Math.PI,Pt={NONE:-1,ROTATE:0,DOLLY:1,PAN:2,TOUCH_ROTATE:3,TOUCH_PAN:4,TOUCH_DOLLY_PAN:5,TOUCH_DOLLY_ROTATE:6},yu=1e-6,hc=class extends Ro{constructor(e,t=null){super(e,t),this.state=Pt.NONE,this.target=new B,this.cursor=new B,this.minDistance=0,this.maxDistance=1/0,this.minZoom=0,this.maxZoom=1/0,this.minTargetRadius=0,this.maxTargetRadius=1/0,this.minPolarAngle=0,this.maxPolarAngle=Math.PI,this.minAzimuthAngle=-1/0,this.maxAzimuthAngle=1/0,this.enableDamping=!1,this.dampingFactor=.05,this.enableZoom=!0,this.zoomSpeed=1,this.enableRotate=!0,this.rotateSpeed=1,this.keyRotateSpeed=1,this.enablePan=!0,this.panSpeed=1,this.screenSpacePanning=!0,this.keyPanSpeed=7,this.zoomToCursor=!1,this.autoRotate=!1,this.autoRotateSpeed=2,this.keys={LEFT:"ArrowLeft",UP:"ArrowUp",RIGHT:"ArrowRight",BOTTOM:"ArrowDown"},this.mouseButtons={LEFT:rs.ROTATE,MIDDLE:rs.DOLLY,RIGHT:rs.PAN},this.touches={ONE:os.ROTATE,TWO:os.DOLLY_PAN},this.target0=this.target.clone(),this.position0=this.object.position.clone(),this.zoom0=this.object.zoom,this._cursorStyle="auto",this._domElementKeyEvents=null,this._lastPosition=new B,this._lastQuaternion=new Zt,this._lastTargetPosition=new B,this._quat=new Zt().setFromUnitVectors(e.up,new B(0,1,0)),this._quatInverse=this._quat.clone().invert(),this._spherical=new Er,this._sphericalDelta=new Er,this._scale=1,this._panOffset=new B,this._rotateStart=new _e,this._rotateEnd=new _e,this._rotateDelta=new _e,this._panStart=new _e,this._panEnd=new _e,this._panDelta=new _e,this._dollyStart=new _e,this._dollyEnd=new _e,this._dollyDelta=new _e,this._dollyDirection=new B,this._mouse=new _e,this._performCursorZoom=!1,this._pointers=[],this._pointerPositions={},this._controlActive=!1,this._onPointerMove=qv.bind(this),this._onPointerDown=Xv.bind(this),this._onPointerUp=Yv.bind(this),this._onContextMenu=ey.bind(this),this._onMouseWheel=jv.bind(this),this._onKeyDown=Jv.bind(this),this._onTouchStart=$v.bind(this),this._onTouchMove=Qv.bind(this),this._onMouseDown=Zv.bind(this),this._onMouseMove=Kv.bind(this),this._interceptControlDown=ty.bind(this),this._interceptControlUp=ny.bind(this),this.domElement!==null&&this.connect(this.domElement),this.update()}set cursorStyle(e){this._cursorStyle=e,e==="grab"?this.domElement.style.cursor="grab":this.domElement.style.cursor="auto"}get cursorStyle(){return this._cursorStyle}connect(e){super.connect(e),this.domElement.addEventListener("pointerdown",this._onPointerDown),this.domElement.addEventListener("pointercancel",this._onPointerUp),this.domElement.addEventListener("contextmenu",this._onContextMenu),this.domElement.addEventListener("wheel",this._onMouseWheel,{passive:!1}),this.domElement.getRootNode().addEventListener("keydown",this._interceptControlDown,{passive:!0,capture:!0}),this.domElement.style.touchAction="none"}disconnect(){this.domElement.removeEventListener("pointerdown",this._onPointerDown),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.domElement.removeEventListener("pointercancel",this._onPointerUp),this.domElement.removeEventListener("wheel",this._onMouseWheel),this.domElement.removeEventListener("contextmenu",this._onContextMenu),this.stopListenToKeyEvents(),this.domElement.getRootNode().removeEventListener("keydown",this._interceptControlDown,{capture:!0}),this.domElement.style.touchAction=""}dispose(){this.disconnect()}getPolarAngle(){return this._spherical.phi}getAzimuthalAngle(){return this._spherical.theta}getDistance(){return this.object.position.distanceTo(this.target)}listenToKeyEvents(e){e.addEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=e}stopListenToKeyEvents(){this._domElementKeyEvents!==null&&(this._domElementKeyEvents.removeEventListener("keydown",this._onKeyDown),this._domElementKeyEvents=null)}saveState(){this.target0.copy(this.target),this.position0.copy(this.object.position),this.zoom0=this.object.zoom}reset(){this.target.copy(this.target0),this.object.position.copy(this.position0),this.object.zoom=this.zoom0,this.object.updateProjectionMatrix(),this.dispatchEvent(Yf),this.update(),this.state=Pt.NONE}pan(e,t){this._pan(e,t),this.update()}dollyIn(e){this._dollyIn(e),this.update()}dollyOut(e){this._dollyOut(e),this.update()}rotateLeft(e){this._rotateLeft(e),this.update()}rotateUp(e){this._rotateUp(e),this.update()}update(e=null){let t=this.object.position;on.copy(t).sub(this.target),on.applyQuaternion(this._quat),this._spherical.setFromVector3(on),this.autoRotate&&this.state===Pt.NONE&&this._rotateLeft(this._getAutoRotationAngle(e)),this.enableDamping?(this._spherical.theta+=this._sphericalDelta.theta*this.dampingFactor,this._spherical.phi+=this._sphericalDelta.phi*this.dampingFactor):(this._spherical.theta+=this._sphericalDelta.theta,this._spherical.phi+=this._sphericalDelta.phi);let n=this.minAzimuthAngle,i=this.maxAzimuthAngle;isFinite(n)&&isFinite(i)&&(n<-Math.PI?n+=wn:n>Math.PI&&(n-=wn),i<-Math.PI?i+=wn:i>Math.PI&&(i-=wn),n<=i?this._spherical.theta=Math.max(n,Math.min(i,this._spherical.theta)):this._spherical.theta=this._spherical.theta>(n+i)/2?Math.max(n,this._spherical.theta):Math.min(i,this._spherical.theta)),this._spherical.phi=Math.max(this.minPolarAngle,Math.min(this.maxPolarAngle,this._spherical.phi)),this._spherical.makeSafe(),this.enableDamping===!0?this.target.addScaledVector(this._panOffset,this.dampingFactor):this.target.add(this._panOffset),this.target.sub(this.cursor),this.target.clampLength(this.minTargetRadius,this.maxTargetRadius),this.target.add(this.cursor);let r=!1;if(this.zoomToCursor&&this._performCursorZoom||this.object.isOrthographicCamera)this._spherical.radius=this._clampDistance(this._spherical.radius);else{let o=this._spherical.radius;this._spherical.radius=this._clampDistance(this._spherical.radius*this._scale),r=o!=this._spherical.radius}if(on.setFromSpherical(this._spherical),on.applyQuaternion(this._quatInverse),t.copy(this.target).add(on),this.object.lookAt(this.target),this.enableDamping===!0?(this._sphericalDelta.theta*=1-this.dampingFactor,this._sphericalDelta.phi*=1-this.dampingFactor,this._panOffset.multiplyScalar(1-this.dampingFactor)):(this._sphericalDelta.set(0,0,0),this._panOffset.set(0,0,0)),this.zoomToCursor&&this._performCursorZoom){let o=null;if(this.object.isPerspectiveCamera){let a=on.length();o=this._clampDistance(a*this._scale);let l=a-o;this.object.position.addScaledVector(this._dollyDirection,l),this.object.updateMatrixWorld(),r=!!l}else if(this.object.isOrthographicCamera){let a=new B(this._mouse.x,this._mouse.y,0);a.unproject(this.object);let l=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),this.object.updateProjectionMatrix(),r=l!==this.object.zoom;let c=new B(this._mouse.x,this._mouse.y,0);c.unproject(this.object),this.object.position.sub(c).add(a),this.object.updateMatrixWorld(),o=on.length()}else console.warn("WARNING: OrbitControls.js encountered an unknown camera type - zoom to cursor disabled."),this.zoomToCursor=!1;o!==null&&(this.screenSpacePanning?this.target.set(0,0,-1).transformDirection(this.object.matrix).multiplyScalar(o).add(this.object.position):(cc.origin.copy(this.object.position),cc.direction.set(0,0,-1).transformDirection(this.object.matrix),Math.abs(this.object.up.dot(cc.direction))<Wv?this.object.lookAt(this.target):(Zf.setFromNormalAndCoplanarPoint(this.object.up,this.target),cc.intersectPlane(Zf,this.target))))}else if(this.object.isOrthographicCamera){let o=this.object.zoom;this.object.zoom=Math.max(this.minZoom,Math.min(this.maxZoom,this.object.zoom/this._scale)),o!==this.object.zoom&&(this.object.updateProjectionMatrix(),r=!0)}return this._scale=1,this._performCursorZoom=!1,r||this._lastPosition.distanceToSquared(this.object.position)>yu||8*(1-this._lastQuaternion.dot(this.object.quaternion))>yu||this._lastTargetPosition.distanceToSquared(this.target)>yu?(this.dispatchEvent(Yf),this._lastPosition.copy(this.object.position),this._lastQuaternion.copy(this.object.quaternion),this._lastTargetPosition.copy(this.target),!0):!1}_getAutoRotationAngle(e){return e!==null?wn/60*this.autoRotateSpeed*e:wn/60/60*this.autoRotateSpeed}_getZoomScale(e){let t=Math.abs(e*.01);return Math.pow(.95,this.zoomSpeed*t)}_rotateLeft(e){this._sphericalDelta.theta-=e}_rotateUp(e){this._sphericalDelta.phi-=e}_panLeft(e,t){on.setFromMatrixColumn(t,0),on.multiplyScalar(-e),this._panOffset.add(on)}_panUp(e,t){this.screenSpacePanning===!0?on.setFromMatrixColumn(t,1):(on.setFromMatrixColumn(t,0),on.crossVectors(this.object.up,on)),on.multiplyScalar(e),this._panOffset.add(on)}_pan(e,t){let n=this.domElement;if(this.object.isPerspectiveCamera){let i=this.object.position;on.copy(i).sub(this.target);let r=on.length();r*=Math.tan(this.object.fov/2*Math.PI/180),this._panLeft(2*e*r/n.clientHeight,this.object.matrix),this._panUp(2*t*r/n.clientHeight,this.object.matrix)}else this.object.isOrthographicCamera?(this._panLeft(e*(this.object.right-this.object.left)/this.object.zoom/n.clientWidth,this.object.matrix),this._panUp(t*(this.object.top-this.object.bottom)/this.object.zoom/n.clientHeight,this.object.matrix)):(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - pan disabled."),this.enablePan=!1)}_dollyOut(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale/=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_dollyIn(e){this.object.isPerspectiveCamera||this.object.isOrthographicCamera?this._scale*=e:(console.warn("WARNING: OrbitControls.js encountered an unknown camera type - dolly/zoom disabled."),this.enableZoom=!1)}_updateZoomParameters(e,t){if(!this.zoomToCursor)return;this._performCursorZoom=!0;let n=this.domElement.getBoundingClientRect(),i=e-n.left,r=t-n.top,o=n.width,a=n.height;this._mouse.x=i/o*2-1,this._mouse.y=-(r/a)*2+1,this._dollyDirection.set(this._mouse.x,this._mouse.y,1).unproject(this.object).sub(this.object.position).normalize()}_clampDistance(e){return Math.max(this.minDistance,Math.min(this.maxDistance,e))}_handleMouseDownRotate(e){this._rotateStart.set(e.clientX,e.clientY)}_handleMouseDownDolly(e){this._updateZoomParameters(e.clientX,e.clientX),this._dollyStart.set(e.clientX,e.clientY)}_handleMouseDownPan(e){this._panStart.set(e.clientX,e.clientY)}_handleMouseMoveRotate(e){this._rotateEnd.set(e.clientX,e.clientY),this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(wn*this._rotateDelta.x/t.clientHeight),this._rotateUp(wn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd),this.update()}_handleMouseMoveDolly(e){this._dollyEnd.set(e.clientX,e.clientY),this._dollyDelta.subVectors(this._dollyEnd,this._dollyStart),this._dollyDelta.y>0?this._dollyOut(this._getZoomScale(this._dollyDelta.y)):this._dollyDelta.y<0&&this._dollyIn(this._getZoomScale(this._dollyDelta.y)),this._dollyStart.copy(this._dollyEnd),this.update()}_handleMouseMovePan(e){this._panEnd.set(e.clientX,e.clientY),this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd),this.update()}_handleMouseWheel(e){this._updateZoomParameters(e.clientX,e.clientY),e.deltaY<0?this._dollyIn(this._getZoomScale(e.deltaY)):e.deltaY>0&&this._dollyOut(this._getZoomScale(e.deltaY)),this.update()}_handleKeyDown(e){let t=!1;switch(e.code){case this.keys.UP:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(wn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,this.keyPanSpeed),t=!0;break;case this.keys.BOTTOM:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateUp(-wn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(0,-this.keyPanSpeed),t=!0;break;case this.keys.LEFT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(wn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(this.keyPanSpeed,0),t=!0;break;case this.keys.RIGHT:e.ctrlKey||e.metaKey||e.shiftKey?this.enableRotate&&this._rotateLeft(-wn*this.keyRotateSpeed/this.domElement.clientHeight):this.enablePan&&this._pan(-this.keyPanSpeed,0),t=!0;break}t&&(e.preventDefault(),this.update())}_handleTouchStartRotate(e){if(this._pointers.length===1)this._rotateStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._rotateStart.set(n,i)}}_handleTouchStartPan(e){if(this._pointers.length===1)this._panStart.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panStart.set(n,i)}}_handleTouchStartDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyStart.set(0,r)}_handleTouchStartDollyPan(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enablePan&&this._handleTouchStartPan(e)}_handleTouchStartDollyRotate(e){this.enableZoom&&this._handleTouchStartDolly(e),this.enableRotate&&this._handleTouchStartRotate(e)}_handleTouchMoveRotate(e){if(this._pointers.length==1)this._rotateEnd.set(e.pageX,e.pageY);else{let n=this._getSecondPointerPosition(e),i=.5*(e.pageX+n.x),r=.5*(e.pageY+n.y);this._rotateEnd.set(i,r)}this._rotateDelta.subVectors(this._rotateEnd,this._rotateStart).multiplyScalar(this.rotateSpeed);let t=this.domElement;this._rotateLeft(wn*this._rotateDelta.x/t.clientHeight),this._rotateUp(wn*this._rotateDelta.y/t.clientHeight),this._rotateStart.copy(this._rotateEnd)}_handleTouchMovePan(e){if(this._pointers.length===1)this._panEnd.set(e.pageX,e.pageY);else{let t=this._getSecondPointerPosition(e),n=.5*(e.pageX+t.x),i=.5*(e.pageY+t.y);this._panEnd.set(n,i)}this._panDelta.subVectors(this._panEnd,this._panStart).multiplyScalar(this.panSpeed),this._pan(this._panDelta.x,this._panDelta.y),this._panStart.copy(this._panEnd)}_handleTouchMoveDolly(e){let t=this._getSecondPointerPosition(e),n=e.pageX-t.x,i=e.pageY-t.y,r=Math.sqrt(n*n+i*i);this._dollyEnd.set(0,r),this._dollyDelta.set(0,Math.pow(this._dollyEnd.y/this._dollyStart.y,this.zoomSpeed)),this._dollyOut(this._dollyDelta.y),this._dollyStart.copy(this._dollyEnd);let o=(e.pageX+t.x)*.5,a=(e.pageY+t.y)*.5;this._updateZoomParameters(o,a)}_handleTouchMoveDollyPan(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enablePan&&this._handleTouchMovePan(e)}_handleTouchMoveDollyRotate(e){this.enableZoom&&this._handleTouchMoveDolly(e),this.enableRotate&&this._handleTouchMoveRotate(e)}_addPointer(e){this._pointers.push(e.pointerId)}_removePointer(e){delete this._pointerPositions[e.pointerId];for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId){this._pointers.splice(t,1);return}}_isTrackingPointer(e){for(let t=0;t<this._pointers.length;t++)if(this._pointers[t]==e.pointerId)return!0;return!1}_trackPointer(e){let t=this._pointerPositions[e.pointerId];t===void 0&&(t=new _e,this._pointerPositions[e.pointerId]=t),t.set(e.pageX,e.pageY)}_getSecondPointerPosition(e){let t=e.pointerId===this._pointers[0]?this._pointers[1]:this._pointers[0];return this._pointerPositions[t]}_customWheelEvent(e){let t=e.deltaMode,n={clientX:e.clientX,clientY:e.clientY,deltaY:e.deltaY};switch(t){case 1:n.deltaY*=16;break;case 2:n.deltaY*=100;break}return e.ctrlKey&&!this._controlActive&&(n.deltaY*=10),n}};function Xv(s){this.enabled!==!1&&(this._pointers.length===0&&(this.domElement.setPointerCapture(s.pointerId),this.domElement.ownerDocument.addEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.addEventListener("pointerup",this._onPointerUp)),!this._isTrackingPointer(s)&&(this._addPointer(s),s.pointerType==="touch"?this._onTouchStart(s):this._onMouseDown(s),this._cursorStyle==="grab"&&(this.domElement.style.cursor="grabbing")))}function qv(s){this.enabled!==!1&&(s.pointerType==="touch"?this._onTouchMove(s):this._onMouseMove(s))}function Yv(s){switch(this._removePointer(s),this._pointers.length){case 0:this.domElement.releasePointerCapture(s.pointerId),this.domElement.ownerDocument.removeEventListener("pointermove",this._onPointerMove),this.domElement.ownerDocument.removeEventListener("pointerup",this._onPointerUp),this.dispatchEvent(Kf),this.state=Pt.NONE,this._cursorStyle==="grab"&&(this.domElement.style.cursor="grab");break;case 1:let e=this._pointers[0],t=this._pointerPositions[e];this._onTouchStart({pointerId:e,pageX:t.x,pageY:t.y});break}}function Zv(s){let e;switch(s.button){case 0:e=this.mouseButtons.LEFT;break;case 1:e=this.mouseButtons.MIDDLE;break;case 2:e=this.mouseButtons.RIGHT;break;default:e=-1}switch(e){case rs.DOLLY:if(this.enableZoom===!1)return;this._handleMouseDownDolly(s),this.state=Pt.DOLLY;break;case rs.ROTATE:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Pt.PAN}else{if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Pt.ROTATE}break;case rs.PAN:if(s.ctrlKey||s.metaKey||s.shiftKey){if(this.enableRotate===!1)return;this._handleMouseDownRotate(s),this.state=Pt.ROTATE}else{if(this.enablePan===!1)return;this._handleMouseDownPan(s),this.state=Pt.PAN}break;default:this.state=Pt.NONE}this.state!==Pt.NONE&&this.dispatchEvent(Mu)}function Kv(s){switch(this.state){case Pt.ROTATE:if(this.enableRotate===!1)return;this._handleMouseMoveRotate(s);break;case Pt.DOLLY:if(this.enableZoom===!1)return;this._handleMouseMoveDolly(s);break;case Pt.PAN:if(this.enablePan===!1)return;this._handleMouseMovePan(s);break}}function jv(s){this.enabled===!1||this.enableZoom===!1||this.state!==Pt.NONE||(s.preventDefault(),this.dispatchEvent(Mu),this._handleMouseWheel(this._customWheelEvent(s)),this.dispatchEvent(Kf))}function Jv(s){this.enabled!==!1&&this._handleKeyDown(s)}function $v(s){switch(this._trackPointer(s),this._pointers.length){case 1:switch(this.touches.ONE){case os.ROTATE:if(this.enableRotate===!1)return;this._handleTouchStartRotate(s),this.state=Pt.TOUCH_ROTATE;break;case os.PAN:if(this.enablePan===!1)return;this._handleTouchStartPan(s),this.state=Pt.TOUCH_PAN;break;default:this.state=Pt.NONE}break;case 2:switch(this.touches.TWO){case os.DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchStartDollyPan(s),this.state=Pt.TOUCH_DOLLY_PAN;break;case os.DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchStartDollyRotate(s),this.state=Pt.TOUCH_DOLLY_ROTATE;break;default:this.state=Pt.NONE}break;default:this.state=Pt.NONE}this.state!==Pt.NONE&&this.dispatchEvent(Mu)}function Qv(s){switch(this._trackPointer(s),this.state){case Pt.TOUCH_ROTATE:if(this.enableRotate===!1)return;this._handleTouchMoveRotate(s),this.update();break;case Pt.TOUCH_PAN:if(this.enablePan===!1)return;this._handleTouchMovePan(s),this.update();break;case Pt.TOUCH_DOLLY_PAN:if(this.enableZoom===!1&&this.enablePan===!1)return;this._handleTouchMoveDollyPan(s),this.update();break;case Pt.TOUCH_DOLLY_ROTATE:if(this.enableZoom===!1&&this.enableRotate===!1)return;this._handleTouchMoveDollyRotate(s),this.update();break;default:this.state=Pt.NONE}}function ey(s){this.enabled!==!1&&s.preventDefault()}function ty(s){s.key==="Control"&&(this._controlActive=!0,this.domElement.getRootNode().addEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}function ny(s){s.key==="Control"&&(this._controlActive=!1,this.domElement.getRootNode().removeEventListener("keyup",this._interceptControlUp,{passive:!0,capture:!0}))}var uc=class extends Rs{constructor(){super(),this.name="RoomEnvironment",this.position.y=-3.5;let e=new mi;e.deleteAttribute("uv");let t=new yn({side:tn}),n=new yn,i=new ni(16777215,900,28,2);i.position.set(.418,16.199,.3),this.add(i);let r=new Ke(e,t);r.position.set(-.757,13.219,.717),r.scale.set(31.713,28.305,28.591),this.add(r);let o=new vn(e,n,6),a=new St;a.position.set(-10.906,2.009,1.846),a.rotation.set(0,-.195,0),a.scale.set(2.328,7.905,4.651),a.updateMatrix(),o.setMatrixAt(0,a.matrix),a.position.set(-5.607,-.754,-.758),a.rotation.set(0,.994,0),a.scale.set(1.97,1.534,3.955),a.updateMatrix(),o.setMatrixAt(1,a.matrix),a.position.set(6.167,.857,7.803),a.rotation.set(0,.561,0),a.scale.set(3.927,6.285,3.687),a.updateMatrix(),o.setMatrixAt(2,a.matrix),a.position.set(-2.017,.018,6.124),a.rotation.set(0,.333,0),a.scale.set(2.002,4.566,2.064),a.updateMatrix(),o.setMatrixAt(3,a.matrix),a.position.set(2.291,-.756,-2.621),a.rotation.set(0,-.286,0),a.scale.set(1.546,1.552,1.496),a.updateMatrix(),o.setMatrixAt(4,a.matrix),a.position.set(-2.193,-.369,-5.547),a.rotation.set(0,.516,0),a.scale.set(3.875,3.487,2.986),a.updateMatrix(),o.setMatrixAt(5,a.matrix),this.add(o);let l=new Ke(e,Fr(50));l.position.set(-16.116,14.37,8.208),l.scale.set(.1,2.428,2.739),this.add(l);let c=new Ke(e,Fr(50));c.position.set(-16.109,18.021,-8.207),c.scale.set(.1,2.425,2.751),this.add(c);let h=new Ke(e,Fr(17));h.position.set(14.904,12.198,-1.832),h.scale.set(.15,4.265,6.331),this.add(h);let u=new Ke(e,Fr(43));u.position.set(-.462,8.89,14.52),u.scale.set(4.38,5.441,.088),this.add(u);let d=new Ke(e,Fr(20));d.position.set(3.235,11.486,-12.541),d.scale.set(2.5,2,.1),this.add(d);let f=new Ke(e,Fr(100));f.position.set(0,20,0),f.scale.set(1,.1,1),this.add(f)}dispose(){let e=new Set;this.traverse(t=>{t.isMesh&&(e.add(t.geometry),e.add(t.material))});for(let t of e)t.dispose()}};function Fr(s){return new go({color:0,emissive:16777215,emissiveIntensity:s})}var ki={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};var On=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},iy=new vi(-1,1,1,-1,0,1),bu=class extends pt{constructor(){super(),this.setAttribute("position",new nt([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new nt([0,2,0,0,2,0],2))}},sy=new bu,Si=class{constructor(e){this._mesh=new Ke(sy,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,iy)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Or=class extends On{constructor(e,t="tDiffuse"){super(),this.textureID=t,this.uniforms=null,this.material=null,e instanceof mt?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=oi.clone(e.uniforms),this.material=new mt({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new Si(this.material)}render(e,t,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ko=class extends On{constructor(e,t){super(),this.scene=e,this.camera=t,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,t,n){let i=e.getContext(),r=e.state;r.buffers.color.setMask(!1),r.buffers.depth.setMask(!1),r.buffers.color.setLocked(!0),r.buffers.depth.setLocked(!0);let o,a;this.inverse?(o=0,a=1):(o=1,a=0),r.buffers.stencil.setTest(!0),r.buffers.stencil.setOp(i.REPLACE,i.REPLACE,i.REPLACE),r.buffers.stencil.setFunc(i.ALWAYS,o,4294967295),r.buffers.stencil.setClear(a),r.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(t),this.clear&&e.clear(),e.render(this.scene,this.camera),r.buffers.color.setLocked(!1),r.buffers.depth.setLocked(!1),r.buffers.color.setMask(!0),r.buffers.depth.setMask(!0),r.buffers.stencil.setLocked(!1),r.buffers.stencil.setFunc(i.EQUAL,1,4294967295),r.buffers.stencil.setOp(i.KEEP,i.KEEP,i.KEEP),r.buffers.stencil.setLocked(!0)}},dc=class extends On{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var fc=class{constructor(e,t){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),t===void 0){let n=e.getSize(new _e);this._width=n.width,this._height=n.height,t=new Wt(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:nn}),t.texture.name="EffectComposer.rt1"}else this._width=t.width,this._height=t.height;this.renderTarget1=t,this.renderTarget2=t.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Or(ki),this.copyPass.material.blending=Wn,this.timer=new Eo}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,t){this.passes.splice(t,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let t=this.passes.indexOf(e);t!==-1&&this.passes.splice(t,1)}isLastEnabledPass(e){for(let t=e+1;t<this.passes.length;t++)if(this.passes[t].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let t=this.renderer.getRenderTarget(),n=!1;for(let i=0,r=this.passes.length;i<r;i++){let o=this.passes[i];if(o.enabled!==!1){if(o.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(i),o.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),o.needsSwap){if(n){let a=this.renderer.getContext(),l=this.renderer.state.buffers.stencil;l.setFunc(a.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),l.setFunc(a.EQUAL,1,4294967295)}this.swapBuffers()}Ko!==void 0&&(o instanceof Ko?n=!0:o instanceof dc&&(n=!1))}}this.renderer.setRenderTarget(t)}reset(e){if(e===void 0){let t=this.renderer.getSize(new _e);this._pixelRatio=this.renderer.getPixelRatio(),this._width=t.width,this._height=t.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,t){this._width=e,this._height=t;let n=this._width*this._pixelRatio,i=this._height*this._pixelRatio;this.renderTarget1.setSize(n,i),this.renderTarget2.setSize(n,i);for(let r=0;r<this.passes.length;r++)this.passes[r].setSize(n,i)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};var jf={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new xe(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Br=class s extends On{constructor(e,t=1,n,i){super(),this.strength=t,this.radius=n,this.threshold=i,this.resolution=e!==void 0?new _e(e.x,e.y):new _e(256,256),this.clearColor=new xe(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);this.renderTargetBright=new Wt(r,o,{type:nn}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let h=0;h<this.nMips;h++){let u=new Wt(r,o,{type:nn});u.texture.name="UnrealBloomPass.h"+h,u.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(u);let d=new Wt(r,o,{type:nn});d.texture.name="UnrealBloomPass.v"+h,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),r=Math.round(r/2),o=Math.round(o/2)}let a=jf;this.highPassUniforms=oi.clone(a.uniforms),this.highPassUniforms.luminosityThreshold.value=i,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new mt({uniforms:this.highPassUniforms,vertexShader:a.vertexShader,fragmentShader:a.fragmentShader}),this.separableBlurMaterials=[];let l=[6,10,14,18,22];r=Math.round(this.resolution.x/2),o=Math.round(this.resolution.y/2);for(let h=0;h<this.nMips;h++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(l[h])),this.separableBlurMaterials[h].uniforms.invSize.value=new _e(1/r,1/o),r=Math.round(r/2),o=Math.round(o/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=t,this.compositeMaterial.uniforms.bloomRadius.value=.1;let c=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=c,this.bloomTintColors=[new B(1,1,1),new B(1,1,1),new B(1,1,1),new B(1,1,1),new B(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=oi.clone(ki.uniforms),this.blendMaterial=new mt({uniforms:this.copyUniforms,vertexShader:ki.vertexShader,fragmentShader:ki.fragmentShader,premultipliedAlpha:!0,blending:Xt,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new xe,this._oldClearAlpha=1,this._basic=new bt,this._fsQuad=new Si(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,t){let n=Math.round(e/2),i=Math.round(t/2);this.renderTargetBright.setSize(n,i);for(let r=0;r<this.nMips;r++)this.renderTargetsHorizontal[r].setSize(n,i),this.renderTargetsVertical[r].setSize(n,i),this.separableBlurMaterials[r].uniforms.invSize.value=new _e(1/n,1/i),n=Math.round(n/2),i=Math.round(i/2)}render(e,t,n,i,r){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let o=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),r&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let a=this.renderTargetBright;for(let l=0;l<this.nMips;l++)this._fsQuad.material=this.separableBlurMaterials[l],this.separableBlurMaterials[l].uniforms.colorTexture.value=a.texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[l]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[l].uniforms.colorTexture.value=this.renderTargetsHorizontal[l].texture,this.separableBlurMaterials[l].uniforms.direction.value=s.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[l]),e.clear(),this._fsQuad.render(e),a=this.renderTargetsVertical[l];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,r&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=o}_getSeparableBlurMaterial(e){let t=[],n=e/3;for(let i=0;i<e;i++)t.push(.39894*Math.exp(-.5*i*i/(n*n))/n);return new mt({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new _e(.5,.5)},direction:{value:new _e(.5,.5)},gaussianCoefficients:{value:t}},vertexShader:`

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

				}`})}};Br.BlurDirectionX=new _e(1,0);Br.BlurDirectionY=new _e(0,1);var jo={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var pc=class extends On{constructor(){super(),this.isOutputPass=!0,this.uniforms=oi.clone(jo.uniforms),this.material=new Sr({name:jo.name,uniforms:this.uniforms,vertexShader:jo.vertexShader,fragmentShader:jo.fragmentShader}),this._fsQuad=new Si(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,t,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},ot.getTransfer(this._outputColorSpace)===Mt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Po?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Io?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Lo?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Us?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===No?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Uo?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Do&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(t),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var ai=512,ry=7.5;function Su(s,e=ry){let t=new fo,n=s.map(([a,l])=>new B(a,0,l)),i=[],r=[],o=n.length;for(let a=0;a<o;a++){let l=n[a],c=n[(a+o-1)%o],h=n[(a+1)%o],u=Math.min(e,l.distanceTo(c)/2,l.distanceTo(h)/2);i.push(l.clone().addScaledVector(c.clone().sub(l).normalize(),u)),r.push(l.clone().addScaledVector(h.clone().sub(l).normalize(),u))}for(let a=0;a<o;a++)t.add(new ti(i[a],n[a],r[a])),t.add(new Mr(r[a],i[(a+1)%o]));return t}function oy(s){let e=Su(s),t=e.getSpacedPoints(ai),n=[];for(let i=0;i<ai;i++){let r=t[(i+ai-1)%ai],o=t[(i+1)%ai];n.push(new B(o.x-r.x,0,o.z-r.z).normalize())}return{points:t,tangents:n,length:e.getLength()}}function Jf(s){let e=[],t=(n,i,r,o)=>{let a=Math.cos(n.angle),l=Math.sin(n.angle);e.push({x:n.x+i*a+r*l,z:n.z-i*l+r*a,r:o,asset:n.asset})};for(let n of s)if(!(n.z<-100))if(n.asset==="data-tram")for(let i of[-2.4,0,2.4])t(n,i,0,1.5);else n.asset==="server-rack"?t(n,0,0,.8):n.asset==="street-lamp"?t(n,0,0,.5):n.asset==="planter"&&(t(n,-.9,0,.85),t(n,.9,0,.85));return e}function $f({routes:s,obstacles:e=[],robotRadius:t=1.3,lanes:n=[2.2,0,-2.2,3.9,-3.9],defaultLane:i=2.2,speeds:r=[],blocked:o=()=>!1}){let a=s.map(oy),l=s.map((w,R)=>({route:R,s:a[R].length*(R*.173%1),dir:1,lane:i,laneTarget:i,speed:0,target:0,laneVel:0,cruise:r[R]??5.1+R*.43,state:"cruise",heading:0,wait:0,cooldown:0,x:0,z:0,fx:0,fz:1,rx:1,rz:0,vx:0,vz:0,turns:0})),c=new B,h=new B,u=new B;function d(w,R,L,V,C){let N=(R/w.length%1+1)%1*ai,U=Math.floor(N)%ai,E=N-Math.floor(N);V.lerpVectors(w.points[U],w.points[(U+1)%ai],E),C.lerpVectors(w.tangents[U],w.tangents[(U+1)%ai],E).normalize().multiplyScalar(L)}let f=w=>Math.atan2(Math.sin(w),Math.cos(w));function g(w){d(a[w.route],w.s,w.dir,c,h),w.fx=h.x,w.fz=h.z,w.rx=-h.z,w.rz=h.x,w.x=c.x+w.rx*w.lane,w.z=c.z+w.rz*w.lane}function y(w,R){for(let L of e){let V=L.r+t;if((w-L.x)**2+(R-L.z)**2<V*V)return!0}return!1}function m(w,R){let L=a[w.route];for(let V of[-2.5,0,1,3,6,9,12,15,18])if(d(L,w.s+w.dir*V,w.dir,c,h),u.set(c.x-h.z*R,0,c.z+h.x*R),y(u.x,u.z)||V>=0&&o(w,u.x,u.z))return!1;return!0}function p(w){w.dir=-w.dir,w.lane=-w.lane,w.laneTarget=i,w.laneVel=0,w.state="turn",w.cooldown=4,w.wait=0,w.target=0,w.turns++}function b(w,R){if(w.held){w.target=0,w.speed=0,w.laneVel=0;return}if(w.cooldown>0&&(w.cooldown-=R),w.state==="turn"){if(w.target=0,w.speed<.05){let N=Math.atan2(w.fx,w.fz),U=f(N-w.heading);w.heading+=U*Math.min(1,R*3.2),Math.abs(U)<.1&&(w.heading=N,w.state="cruise")}return}let L=null,V=[...n].sort((N,U)=>Math.abs(N-i)-Math.abs(U-i)||U-N);for(let N of V)if(m(w,N)){L=N;break}if(L===null){w.cooldown<=0?p(w):w.target=0;return}w.laneTarget=L,w.target=w.cruise,Math.abs(w.laneTarget-w.lane)>.5&&!m(w,w.lane)&&(w.target=Math.min(w.target,2.4));let C=!1;for(let N of l){if(N===w)continue;let U=I(w,w.laneTarget,N);if(!U)continue;C=!0;let E=N.fx*w.fx+N.fz*w.fz,G=N.speed<.3;if(G||E<-.2){let Y=x.get(N)[S.length-1],X=G?0:Math.sign((Y.x-w.x)*w.rx+(Y.z-w.z)*w.rz)||-1,ie=null;for(let ce of V)if(!(X&&Math.sign(ce-w.lane)===X)&&m(w,ce)&&!I(w,ce,N)){ie=ce;break}ie!==null?w.laneTarget=ie:w.target=0}else E>.5?U.da>=U.db&&(w.target=Math.min(w.target,U.da<3.8?0:N.speed*.9)):(U.da>U.db||U.da===U.db&&w.route>N.route)&&(w.target=Math.min(w.target,U.da<=4?0:2))}C&&w.target===0?w.wait+=R:w.wait=0,w.wait>2.5&&w.cooldown<=0&&p(w)}let S=[0,2,4,6,8,10,12],x=new Map,A=new B;function T(w,R,L,V){d(a[w.route],w.s+w.dir*R,w.dir,c,h),V.set(c.x-h.z*L,0,c.z+h.x*L)}function I(w,R,L){let V=x.get(L),C=(t*2+.6)**2,N=L.speed<.3;for(let U=0;U<S.length;U++){T(w,S[U],R,A);let E=S[U]/Math.max(w.speed,1.5);for(let G=0;G<S.length&&!(N&&G>0);G++){let Y=S[G]/Math.max(L.speed,1.5);if(!(S[U]>4&&Math.abs(E-Y)>1.6)&&A.distanceToSquared(V[G])<C)return{da:S[U],db:S[G]}}}return null}function v(w){if(w>0){for(let R of l){g(R),x.has(R)||x.set(R,S.map(()=>new B));let L=x.get(R);S.forEach((V,C)=>T(R,R.state==="cruise"?V:0,R.lane,L[C]))}for(let R of l)b(R,w);for(let R=0;R<l.length;R++)for(let L=R+1;L<l.length;L++){let V=l[R],C=l[L],N=C.x-V.x,U=C.z-V.z,E=Math.hypot(N,U),G=t*2+.2;if(E>=G)continue;let Y=V.state==="turn"||V.speed<.3,X=C.state==="turn"||C.speed<.3,ie=Y===X?.5:Y?0:1,ce=Math.min(G-Math.max(E,.01),2.5*w),be=N*V.rx+U*V.rz,Le=-(N*C.rx+U*C.rz);V.lane=wt.clamp(V.lane-Math.sign(be||1)*ce*ie,-4.2,4.2),C.lane=wt.clamp(C.lane-Math.sign(Le||1)*ce*(1-ie),-4.2,4.2),V.target=Math.min(V.target,.5),C.target=Math.min(C.target,.5)}for(let R of l){let L=R.x,V=R.z,C=R.target>R.speed?6:9;if(R.held){R.vx=R.vz=0;continue}R.speed+=wt.clamp(R.target-R.speed,-C*w,C*w),R.speed<0&&(R.speed=0),R.s+=R.dir*R.speed*w;let N=R.laneTarget-R.lane,U=R.state==="turn"?0:Math.min(3,R.speed*.6),E=Math.sign(N)*Math.min(U,Math.sqrt(8*Math.abs(N)),Math.abs(N)/w);if(R.laneVel+=wt.clamp(E-R.laneVel,-4*w,4*w),R.lane+=R.laneVel*w,g(R),R.vx=(R.x-L)/w,R.vz=(R.z-V)/w,R.state==="cruise"){let G=R.speed>.25?Math.atan2(R.vx,R.vz):Math.atan2(R.fx,R.fz);R.heading+=f(G-R.heading)*Math.min(1,w*(R.speed>.25?8:3.2))}}}}let D=[];for(let w of l){for(let R=0;R<600&&(g(w),!(D.every(V=>Math.hypot(w.x-V.x,w.z-V.z)>t*2+6)&&m(w,w.lane)));R++)w.s+=1;w.heading=Math.atan2(w.fx,w.fz),D.push(w)}return{agents:l,step:v,hitsObstacle:y,seek(w,R){w.s=R,g(w),w.heading=Math.atan2(w.fx,w.fz)},adopt(w,R){let L=a[w.route],V=0,C=1/0;for(let N=0;N<ai;N++){let U=L.points[N],E=Math.hypot(U.x-R.x,U.z-R.z);E<C&&(C=E,V=N)}w.s=V/ai*L.length,d(L,w.s,w.dir,c,h),w.lane=(R.x-c.x)*-h.z+(R.z-c.z)*h.x,w.laneTarget=i,w.speed=0,w.laneVel=0,w.state="cruise",w.heading=R.heading,g(w)},stats:()=>({states:l.map(w=>w.state),turns:l.reduce((w,R)=>w+R.turns,0),lanes:l.map(w=>Math.round(w.lane*10)/10)})}}var ay=(s,e,t)=>Math.max(e,Math.min(t,s)),xc=(s,e)=>Math.atan2(Math.sin(e-s),Math.cos(e-s)),Jo=s=>({x:s.x,y:s.y||0,z:s.z,heading:s.heading||0});function Hi(s,e=1){let t=s.min.map(d=>d*e),n=s.max.map(d=>d*e),i=(n[0]-t[0])/2,r=(n[2]-t[2])/2,o=i>r,a=Math.max(i,r),l=Math.min(i,r),c=Math.min(7,Math.max(1,Math.ceil(a/Math.max(.3,l)))),h=a/c,u=[];for(let d=0;d<c;d++){let f=-a+h+d*h*2;u.push({x:(t[0]+n[0])/2+(o?f:0),z:(t[2]+n[2])/2+(o?0:f),r:Math.hypot(l,h)})}return{circles:u,minY:t[1],maxY:n[1],reach:Math.max(...u.map(d=>Math.hypot(d.x,d.z)+d.r))}}function zr(s,e){let t=Math.cos(e.heading),n=Math.sin(e.heading);return{x:e.x+s.x*t+s.z*n,z:e.z-s.x*n+s.z*t,r:s.r}}function gc(s,e,t,n,i){let r=e-s;if(Math.abs(r)<1e-10)return s>=t&&s<=n;let o=(t-s)/r,a=(n-s)/r;return i[0]=Math.max(i[0],Math.min(o,a)),i[1]=Math.min(i[1],Math.max(o,a)),i[0]<=i[1]}function ly(s,e,t,n,i,r){let o=[0,1];return gc(e.y-i.y,t.y-r.y,n.minY-s.maxY+.06,n.maxY-s.minY-.06,o)?o:null}function mc(s,e,t,n,i,r){let o=s.reach+n.reach+.06;if(Math.min(e.x,t.x)-Math.max(i.x,r.x)>o||Math.min(i.x,r.x)-Math.max(e.x,t.x)>o||Math.min(e.z,t.z)-Math.max(i.z,r.z)>o||Math.min(i.z,r.z)-Math.max(e.z,t.z)>o)return!1;let a=ly(s,e,t,n,i,r);if(!a)return!1;for(let l of s.circles)for(let c of n.circles){let h=.06+Math.abs(xc(e.heading,t.heading))*Math.hypot(l.x,l.z)+Math.abs(xc(i.heading,r.heading))*Math.hypot(c.x,c.z),u=zr(l,e),d=zr(l,t),f=zr(c,i),g=zr(c,r),y=u.x-f.x,m=u.z-f.z,p=d.x-g.x-y,b=d.z-g.z-m,S=ay(-(y*p+m*b)/Math.max(1e-12,p*p+b*b),...a),x=l.r+c.r+h;if((y+p*S)**2+(m+b*S)**2<x*x)return!0}return!1}function wu(s,e,t,n){let i=s.reach+.06;if(n.world&&(Math.min(e.x,t.x)-i>n.world[2]||Math.max(e.x,t.x)+i<n.world[0]||Math.min(e.z,t.z)-i>n.world[3]||Math.max(e.z,t.z)+i<n.world[1])||Math.min(e.y,t.y)+s.minY>=n.max[1]||Math.max(e.y,t.y)+s.maxY<=n.min[1])return!1;let r=Math.cos(n.heading||0),o=Math.sin(n.heading||0),a=l=>({x:(l.x-n.x)*r-(l.z-n.z)*o,z:(l.x-n.x)*o+(l.z-n.z)*r});for(let l of s.circles){let c=a(zr(l,e)),h=a(zr(l,t)),u=l.r+.06+Math.abs(xc(e.heading,t.heading))*Math.hypot(l.x,l.z),d=[0,1];if(gc(e.y,t.y,n.min[1]-s.maxY+.06,n.max[1]-s.minY-.06,d)&&gc(c.x,h.x,n.min[0]-u,n.max[0]+u,d)&&gc(c.z,h.z,n.min[2]-u,n.max[2]+u,d))return!0}return!1}function _c(){let s=new Map,e=new Map,t=new Map,n=0,i=0,r=0,o=0,a=(g,y)=>g.id===y.id||g.ignore===y.id||y.ignore===g.id||g.enabled===!1||y.enabled===!1;function l(g,y,m,p=!0){if(![m.x,m.y,m.z,m.heading].every(Number.isFinite))return"invalid-pose";for(let b of e.values())if(b.enabled!==!1&&!(b.owner&&(b.owner===g.ignore||b.owner===g.id))&&wu(g,y,m,b))return b.id;if(p){for(let b of s.values())if(!a(g,b)&&mc(g,y,m,b,b,b))return b.id}return null}let c=(g,y,m,p=!0)=>!l(g,y,m,p);function h(g,y,m,p=0){if(s.has(g))return s.get(g);let b={id:g,...y,...Jo(m),priority:p,enabled:!0,blocked:0};return c(b,b,b)?(s.set(g,b),b):null}function u(g,y,m=12){let p={...g,enabled:!0};for(let b=0;b<=m;b+=.5)for(let S=0;S<(b?24:1);S++){let x={...Jo(y),x:y.x+Math.cos(S*Math.PI/12)*b,z:y.z+Math.sin(S*Math.PI/12)*b};if(c(p,x,x))return x}return null}function d(g,y,m=12){let p=u(g,y,m);return p?(Object.assign(g,p),!0):!1}function f(g){let y=[...s.values()].filter(m=>m.enabled!==!1).map(m=>{let p=t.get(m.id),b=p?.next||Jo(m);return{body:m,from:Jo(m),next:b,request:p,accepted:c(m,m,b,!1)}});for(let m=0;m<=y.length;m++){let p=!1;for(let b of y)b.accepted&&b.body.follow&&y.some(S=>S.body.id===b.body.follow&&!S.accepted)&&(b.accepted=!1,p=!0);for(let b=0;b<y.length;b++)for(let S=b+1;S<y.length;S++){let x=y[b],A=y[S];if(a(x.body,A.body)||!x.accepted&&!A.accepted)continue;let T=x.accepted?x.next:x.from,I=A.accepted?A.next:A.from;if(!mc(x.body,x.from,T,A.body,A.from,I))continue;let v=Y=>Y.request&&Y.accepted&&(Math.hypot(Y.next.x-Y.from.x,Y.next.y-Y.from.y,Y.next.z-Y.from.z)>1e-8||Math.abs(xc(Y.from.heading,Y.next.heading))>1e-8),D=v(x),w=v(A);if(!D&&!w)continue;let R=x.next.x-x.from.x,L=x.next.z-x.from.z,V=A.next.x-A.from.x,C=A.next.z-A.from.z,N=R*V+L*C>.8*Math.hypot(R,L)*Math.hypot(V,C),U=D&&!mc(x.body,x.from,x.next,A.body,A.from,A.from),E=w&&!mc(x.body,x.from,x.from,A.body,A.from,A.next),G=D?w?U&&!E?A:E&&!U?x:x.body.priority!==A.body.priority?x.body.priority<A.body.priority?x:A:N?(A.from.x-x.from.x)*R+(A.from.z-x.from.z)*L>0?x:A:x.body.id>A.body.id?x:A:x:A;G.accepted=!1,p=!0}if(!p)break}for(let m of y)m.accepted?(Object.assign(m.body,m.next),m.body.blocked=0,m.body.obstruction=null):m.request&&(m.body.blocked+=g,m.body.obstruction=l(m.body,m.from,m.next),n++);for(let m of y)m.request?.commit?.(m.accepted,m.body);t.clear(),i++,r+=g}return{register:h,relocate:d,findFree:u,clear:c,obstruction:l,solve:f,begin(){t.clear()},propose(g,y,m){g?.enabled!==!1&&g&&t.set(g.id,{next:Jo(y),commit:m})},remove(g){s.delete(g),t.delete(g)},solid(g,y){let m=e.get(g),p={id:g,x:0,z:0,heading:0,...y};if(m&&m.x===p.x&&m.z===p.z&&m.heading===p.heading&&m.enabled===p.enabled&&m.min.every((T,I)=>T===p.min[I])&&m.max.every((T,I)=>T===p.max[I]))return;let b=Math.cos(p.heading),S=Math.sin(p.heading),x=[],A=[];for(let T of[p.min[0],p.max[0]])for(let I of[p.min[2],p.max[2]])x.push(p.x+T*b+I*S),A.push(p.z-T*S+I*b);p.world=[Math.min(...x),Math.min(...A),Math.max(...x),Math.max(...A)],e.set(g,p),o++},removeSolid(g){e.delete(g)&&o++},removeOwner(g){for(let[y,m]of e)m.owner===g&&(e.delete(y),o++)},revision:()=>o,solidClear(g){return[...s.values()].every(y=>y.enabled===!1||g.owner&&y.ignore===g.owner||!wu(y,y,y,{x:0,z:0,heading:0,...g}))},ceiling(g,y,m){let p=0;for(let b of e.values())b.enabled!==!1&&wu(g,{x:y,y:b.min[1],z:m,heading:0},{x:y,y:b.min[1],z:m,heading:0},b)&&(p=Math.max(p,b.max[1]-g.minY+1));return p},body:g=>s.get(g),neighbors(g,y=12){return[...s.values()].filter(m=>!a(g,m)&&Math.abs(m.y-g.y)<4&&Math.hypot(m.x-g.x,m.z-g.z)<y)},stats:()=>({bodies:s.size,solids:e.size,stops:n,steps:i,time:r,poses:[...s.values()].filter(g=>g.enabled!==!1).map(g=>({id:g.id,x:g.x,y:g.y,z:g.z,heading:g.heading,blocked:g.blocked,obstruction:g.obstruction}))}),dispose(){s.clear(),e.clear(),t.clear()}}}function Qf(s,e=1/60){let t=0;return n=>{if(!(n>0))return t=0,0;t=Math.min(t+n,e*6);let i=0;for(;t>=e-1e-9;)s(e),t-=e,i++;return i}}var ep=[[[-67,-77],[-18,-77],[-18,-32],[-67,-32]],[[-18,-77],[18,-77],[18,-32],[-18,-32]],[[18,-77],[67,-77],[67,-32],[18,-32]],[[18,13],[18,-32],[67,-32],[67,13]],[[-67,-77],[-18,-77],[-18,-32],[-67,-32]]];function tp(s,e,t){let n=!1,i=0,r=!1,o=!1,a=0,l=!1,c=new ct;c.name="city-life",s.add(c);let h=t.traffic,u=[],d=$f({routes:ep,obstacles:t.obstacles||[],blocked:(he,ue,H)=>{let Z=u[he.route],W=Z?{...Z,x:ue,z:H}:null;return Z?!h.clear(Z,W,W):!1}}),f=new Set,g=new Set,y=new Set,m=[],p=new AbortController,b=[],S=[],x=he=>(f.add(he),he),A=he=>(g.add(he),he),T=x(new Nn(1,.013,5,64));for(let he of e){let ue=A(new bt({color:7452878,transparent:!0,opacity:.08,depthWrite:!1,toneMapped:!1})),H=new Ke(T,ue);H.rotation.x=Math.PI/2,H.position.set(he.x,.6,he.z),H.scale.setScalar(he.radius*1.12),c.add(H),S.push({id:he.id,ring:H,material:ue,state:"unknown",eventUntil:0,color:new xe(7438733)})}let I=new Map,v=[],D=new Map,w=Date.now(),R=3e3,L=e.find(he=>he.id==="agent"),V=new B(0,0,1),C=new B,N={memory:12690431,graph:16765844,infra:7978495,integrations:7728086,missions:10268415,operations:16758915};if(L){for(let he of e)if(he!==L){let ue=new B(L.x,L.height+3,L.z),H=new B(he.x,he.height+4,he.z),Z=ue.clone().lerp(H,.5);Z.y=Math.max(ue.y,H.y)+12;let W=new ti(ue,Z,H),J=A(new pi({color:N[he.id],transparent:!0,opacity:0,depthWrite:!1,blending:Xt,toneMapped:!1})),te=new Di(x(new pt().setFromPoints(W.getPoints(48))),J);te.visible=!1,c.add(te),I.set(he.id,{curve:W,line:te})}}let U=x(new Nn(1,.06,5,40,Math.PI*.95));for(let he=0;he<12;he++){let ue=[];for(let H=0;H<4;H++){let Z=A(new bt({color:10345983,transparent:!0,opacity:0,depthWrite:!1,blending:Xt,toneMapped:!1})),W=new Ke(H===3?T:U,Z);W.visible=!1,c.add(W),ue.push(W)}v.push({at:-1/0,waves:ue,link:null})}function E(he,ue){if(!l||t.active?.()===!1||he.at<w||ue-he.at>R||he.at>ue||!["started","succeeded","failed","sanitized","progress"].includes(he.state))return;let H=he.to==="agent",Z=H?he.from:he.to;if(!H&&he.from!=="agent"||!I.has(Z))return;let W=Z+":"+H;if(he.at-(D.get(W)??-1/0)<450)return;D.set(W,he.at);let J=v.reduce((te,pe)=>te.at<pe.at?te:pe);Object.assign(J,{at:he.at,link:I.get(Z),incoming:H,from:he.from,to:he.to,state:he.state}),J.waves.forEach(te=>te.material.color.setHex(he.state==="failed"?16737881:N[Z]))}function G(he){let ue=Date.now();I.forEach(H=>{H.line.visible=!1});for(let H of v){(!he||t.active?.()===!1)&&(H.at=-1/0);let Z=(ue-H.at)/R,W=Z>=0&&Z<1;if(H.waves.forEach(te=>{te.visible=!1}),!W||!H.link)continue;H.link.line.visible=!0,H.link.line.material.opacity=.12*Math.sin(Z*Math.PI);for(let te=0;te<3;te++){let pe=(Z-te*.075)/.75;if(pe<0||pe>1)continue;let we=H.incoming?1-pe:pe,Pe=H.waves[te];H.link.curve.getPoint(we,Pe.position),H.link.curve.getTangent(we,C),H.incoming&&C.negate(),Pe.quaternion.setFromUnitVectors(V,C),Pe.rotateZ(Math.PI*.525),Pe.scale.setScalar(1.2+Math.sin(pe*Math.PI)*3.5),Pe.material.opacity=Math.sin(pe*Math.PI)*.8,Pe.visible=!0}let J=(Z-.74)/.26;if(J>0){let te=H.waves[3];H.link.curve.getPoint(H.incoming?0:1,te.position),te.rotation.set(-Math.PI/2,0,0),te.scale.setScalar(1+J*8),te.material.opacity=(1-J)*.6,te.visible=!0}}}let Y=x(new ln(7,7)),X=A(new mt({transparent:!0,depthWrite:!1,blending:Xt,uniforms:{color:{value:new xe(5495284)}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:"varying vec2 vUv;uniform vec3 color;void main(){float r=length(vUv-.5)*2.;float a=exp(-r*r*5.)*.23*(1.-smoothstep(.65,1.,r));gl_FragColor=vec4(color,a);}"})),ie=x(new Is(.42,1.3,12,1,!0)),ce=A(new bt({color:9629439,transparent:!0,opacity:.22,depthWrite:!1,toneMapped:!1,blending:Xt}));ep.forEach((he,ue)=>{let H=new ct;H.name="city-white-robot-"+(ue+1);let Z=new ct;H.add(Z),c.add(H);let W=new Ke(Y,X);W.rotation.x=-Math.PI/2,W.position.y=.58,c.add(W);let J=new Ke(ie,ce);J.rotation.z=Math.PI,J.position.y=-.45,H.add(J),H.visible=!1,W.visible=!1,b.push({root:H,body:Z,glow:W,jet:J,agent:d.agents[ue]})});function be(he){he.traverse(ue=>{if(ue.isMesh){f.add(ue.geometry);for(let H of Array.isArray(ue.material)?ue.material:[ue.material]){g.add(H);for(let Z of Object.values(H))Z?.isTexture&&y.add(Z)}}})}function Le(){f.forEach(ue=>ue.dispose()),g.forEach(ue=>ue.dispose());let he=new Set;y.forEach(ue=>{ue.image&&he.add(ue.image),ue.dispose()}),he.forEach(ue=>ue.close?.()),f.clear(),g.clear(),y.clear()}async function Re(){let he=setTimeout(()=>p.abort(),15e3);try{let ue=await fetch(t.robotURL,{signal:p.signal});if(!ue.ok)throw Error("Robot unavailable");let H=await ue.arrayBuffer();if(n)return;let{scene:Z}=await new ds().parseAsync(H,"");if(be(Z),n){Le();return}let W=new en().setFromObject(Z),J=W.getSize(new B),te=Math.max(J.x,J.y,J.z);if(!Number.isFinite(te)||te<=0)throw Error("Invalid robot bounds");Z.scale.setScalar(6/te),Z.updateMatrixWorld(!0),W.setFromObject(Z);let pe=W.getCenter(new B);Z.position.set(-pe.x,-W.min.y,-pe.z),Z.rotation.y=-Math.PI/2,Z.updateMatrixWorld(!0),W.setFromObject(Z),W.expandByScalar(.4);let we=Hi({min:W.min.toArray(),max:W.max.toArray()});Z.traverse(Pe=>{Pe.isMesh&&(Pe.castShadow=!1,Pe.receiveShadow=!0)}),b.forEach((Pe,je)=>{if(Pe.body.add(Z.clone(!0)),Pe.root.visible=!0,Pe.glow.visible=!0,h){let Xe=Pe.agent,Je=null;for(let q=0;q<600&&!Je;q++)Je=h.register("patrol-"+je,we,{x:Xe.x,y:1.6,z:Xe.z,heading:Xe.heading},1),Je||d.seek(Xe,Xe.s+1);Je?(u[je]=Je,t.society?.patrol(Je.id,Je,Pe.root,(q,it)=>{Xe.held=q,it&&d.adopt(Xe,it)})):(Pe.root.visible=!1,Pe.glow.visible=!1,Xe.held=!0)}}),r=!0,ae(0,!1)}catch(ue){n||(o=!0,t.onError?.(ue))}finally{clearTimeout(he)}}function oe(he){m.forEach(H=>{H.dispose(),g.delete(H)}),m.length=0,he.children.find(H=>H.userData.district==="operations")?.getObjectByName("signal")?.traverse(H=>{if(!H.isMesh)return;let Z=W=>{let J=A(W.clone());return m.push(J),J};H.material=Array.isArray(H.material)?H.material.map(Z):Z(H.material)})}function ge(he,ue=[]){for(let Z of S){let W=he.find(J=>J.id===Z.id);Z.state=!W||W.stale?"unknown":W.state,Z.color.setHex(Z.state==="error"?16730430:Z.state==="running"?7400403:Z.state==="unknown"?7438733:7452878),Z.material.color.copy(Z.color)}let H=Date.now();for(let Z of[...ue].reverse())if(Z.id>a&&(E(Z,H),l&&Z.at>=w&&t.active?.()!==!1&&t.society?.event(Z),H-Z.at<6e3)){let W=S.find(J=>J.id===Z.district);W&&(W.eventUntil=Z.at+5e3)}a=Math.max(a,...ue.map(Z=>Z.id))}function ae(he,ue){if(n)return;l=ue,G(ue);let H=ue?Math.min(.1,Math.max(0,he)):0,Z=h?d.agents.map(W=>({...W})):null;ue&&H>0&&(i+=H,d.step(H));for(let[W,J]of b.entries()){let te=J.agent,pe=Math.hypot(te.vx,te.vz),we=(!ue||te.held)&&u[W]?u[W]:te;J.root.position.set(we.x,1.6+Math.sin(i*1.6+W*1.9)*.18,we.z),J.root.rotation.y=we.heading;let Pe=(te.vx*te.rx+te.vz*te.rz)/Math.max(1,pe);if(J.body.rotation.z=Math.sin(i*.85+W)*.035-Pe*.08,J.body.rotation.x=-.035+Math.sin(i*1.1+W)*.018-Math.min(.09,pe*.015),J.glow.position.x=we.x,J.glow.position.z=we.z,J.jet.scale.y=1+Math.sin(i*4+W)*.12+pe*.04,ue&&u[W]&&!te.held){let je=u[W],Xe={x:te.x,y:1.6,z:te.z,heading:te.heading};J.escape&&i<J.escape.until?(Xe.x=je.x+J.escape.x*H*1.8,Xe.z=je.z+J.escape.z*H*1.8,Xe.heading=je.heading):J.escape=null,!h.clear(je,je,Xe,!1)&&h.clear(je,je,{...Xe,heading:je.heading},!1)&&(Xe.heading=je.heading);let Je=Math.hypot(Xe.x-je.x,Xe.z-je.z);h.propose(je,Xe,(q,it)=>{if(J.wait=q&&Je>.002?0:(J.wait||0)+H,J.escape&&q&&d.adopt(te,it),J.wait>2.5){let Ue=Math.sin(it.heading),M=Math.cos(it.heading);for(let[_,O]of[[M,-Ue],[-M,Ue],[Ue,M],[-Ue,-M]])if(h.clear(it,it,{...it,x:it.x+_*2.5,z:it.z+O*2.5})){J.escape={x:_,z:O,until:i+2};break}J.wait=0}if(q)te.trafficWait=0,te.heading=it.heading;else{let Ue=(te.trafficWait||0)+H;Object.assign(te,Z[W]),te.speed=0,te.vx=te.vz=0,te.trafficWait=Ue,Ue>2.5&&(te.state!=="turn"?(te.dir=-te.dir,te.lane=-te.lane,te.laneTarget=2.2,te.state="turn",te.turns++):te.state="cruise",te.trafficWait=0)}J.root.position.x=it.x,J.root.position.z=it.z,J.root.rotation.y=it.heading,J.glow.position.x=it.x,J.glow.position.z=it.z})}}for(let W of S){let J=ue?.5+.5*Math.sin(i*(W.state==="error"?3.2:1.8)):.5,te=W.state==="error"||W.state==="running"||W.eventUntil>Date.now();if(W.material.opacity=W.state==="unknown"?.03:te?.2+J*.46:.065,W.ring.scale.setScalar(e.find(pe=>pe.id===W.id).radius*1.12*(1+(te&&ue?J*.035:0))),W.id==="operations")for(let pe of m)pe.color.copy(W.color),pe.emissive?.copy(W.color),pe.emissiveIntensity=W.state==="error"?1.1+J*2.2:W.state==="unknown"?.08:.55}}function Se(){n||(n=!0,p.abort(),t.signal?.removeEventListener("abort",Se),c.removeFromParent(),Le(),b.length=0,m.length=0,u.forEach(he=>h.remove(he.id)))}return t.signal?.addEventListener("abort",Se,{once:!0}),t.signal?.aborted?Se():Re(),{update:ae,setData:ge,attachLandmarks:oe,dispose:Se,stats:()=>({robots:r?b.filter(he=>he.root.visible).length:0,robotError:o,time:i,navigation:d.stats(),positions:b.map(he=>he.root.position.toArray()),transmissions:v.filter(he=>he.waves.some(ue=>ue.visible)).map(he=>({from:he.from,to:he.to,state:he.state,age:Date.now()-he.at})),signals:S.map(he=>({id:he.id,state:he.state,intensity:he.material.opacity}))})}}var $o='Geist, "Segoe UI", system-ui, sans-serif',vc=13,cy=s=>{let e=2166136261;for(let t=0;t<s.length;t++)e=Math.imul(e^s.charCodeAt(t),16777619);return(e>>>0).toString(16).padStart(8,"0")},Eu={vertex:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragment:`uniform sampler2D map;uniform float time,glitch,fade,gain;uniform vec3 tint;varying vec2 vUv;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){vec2 uv=vUv;float row=floor(uv.y*44.0),frame=floor(time*24.0);
float g=glitch*step(0.7,hash(row+frame*0.37));uv.x+=(hash(row*3.1+frame)-0.5)*0.14*g;
float split=0.0025+glitch*0.02;vec4 c=texture2D(map,uv);
float r=texture2D(map,uv+vec2(split,0.0)).r,b=texture2D(map,uv-vec2(split,0.0)).b;
float a=max(c.a,max(texture2D(map,uv+vec2(split,0.0)).a,texture2D(map,uv-vec2(split,0.0)).a));
float lines=0.84+0.16*sin(uv.y*420.0-time*9.0);float flicker=0.95+0.05*sin(time*31.0)*sin(time*7.3);
float bx=(fract(uv.y*0.55-time*0.11)-0.5)*22.0;float band=0.35*exp(-bx*bx);
float edge=smoothstep(0.0,0.05,uv.x)*smoothstep(0.0,0.05,1.0-uv.x)*smoothstep(0.0,0.09,uv.y)*smoothstep(0.0,0.09,1.0-uv.y);
vec3 col=vec3(r,c.g,b)*tint*(lines*flicker+band)*gain;gl_FragColor=vec4(col*a*fade*edge,1.0);}`},hy={vertex:"varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",fragment:`uniform float time,strength;uniform vec3 base,top;varying vec2 vUv;varying vec3 vNormal,vView;
float hash(float n){return fract(sin(n)*43758.5453);}
void main(){float h=vUv.y;float rim=pow(clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0),1.5);
float fall=pow(clamp(1.0-h,0.0,1.0),1.35)*0.75+0.25*clamp(1.0-h,0.0,1.0);float scan=0.7+0.3*sin(h*64.0-time*5.5);
float noise=0.85+0.15*hash(floor(h*96.0)+floor(time*18.0));float shimmer=0.8+0.2*sin(vUv.x*40.0+time*2.0);
float a=(0.28+0.72*rim)*fall*scan*noise*shimmer*strength;gl_FragColor=vec4(mix(base,top,h)*a,1.0);}`},uy={vertex:Eu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float r=length(vUv-0.5)*2.0;float rings=smoothstep(0.35,1.0,sin(r*16.0-time*2.6));
float core=exp(-r*r*7.0);float a=(core*1.2+pow(max(0.0,1.0-r),1.8)*(0.25+0.75*rings)*0.6)*strength*(1.0-smoothstep(0.85,1.0,r));
gl_FragColor=vec4(color*a,1.0);}`},np={vertex:Eu.vertex,fragment:`uniform float time,strength;uniform vec3 color;varying vec2 vUv;
void main(){float dash=step(0.45,fract(vUv.x*28.0+time*0.35));float pulse=0.75+0.25*sin(vUv.x*6.2831*3.0-time*4.0);
gl_FragColor=vec4(color*dash*pulse*strength,1.0);}`},dy={vertex:`attribute float seed;uniform float time,pointScale;varying float vLife;
void main(){float speed=0.55+seed*0.9;float life=fract(seed*3.17+time*speed/13.0);float y=life*13.0;
float ang=seed*6.2831+time*(0.35+seed*0.4)+life*2.2;float rad=mix(1.8,6.8,life)*(0.3+0.7*fract(seed*7.13));
vec4 mv=modelViewMatrix*vec4(cos(ang)*rad,y,sin(ang)*rad,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.09+0.16*fract(seed*3.7))*pointScale/max(1.0,-mv.z);vLife=life;}`,fragment:`uniform vec3 color;uniform float strength;varying float vLife;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,2.0)*sin(vLife*3.1416)*strength;gl_FragColor=vec4(color*a,1.0);}`};function kr(s,e,t={}){return new mt({uniforms:e,vertexShader:s.vertex,fragmentShader:s.fragment,transparent:!0,depthWrite:!1,blending:Xt,side:Nt,...t})}function fy(s,e=24){let t=[],n=new Set;for(let i of Array.isArray(s)?s:[]){if(typeof i!="string")continue;let r=i.replace(/[\u0000-\u001f\u007f-\u009f\u200b-\u200f\u2028-\u202e\u2066-\u2069]/g," ").replace(/\s+/g," ").trim().slice(0,140);if(!(r.length<4||n.has(r))&&(n.add(r),t.push(r),t.length>=e))break}return t}function ip(s,e,t,n){let i=[],r=c=>s.measureText(c).width<=t,o="";for(let c of e.split(" ")){if(i.length>=n)break;let h=o?o+" "+c:c;if(r(h)){o=h;continue}if(o&&(i.push(o),o="",i.length>=n))break;let u=c;for(;!r(u)&&i.length<n;){let d=u.length-1;for(;d>1&&!r(u.slice(0,d));)d--;i.push(u.slice(0,d)),u=u.slice(d)}o=u}o&&i.length<n&&(i.push(o),o="");let a=i.join("").replace(/\s/g,"").length,l=e.replace(/\s/g,"").length;if(a<l&&i.length){let c=i[i.length-1].replace(/[\s,.;:]+$/,"");for(;c.length&&!r(c+"\u2026");)c=c.slice(0,-1);i[i.length-1]=c+"\u2026"}return i.length?i:[""]}function sp(s,e,t={}){let n=t.roof??21,i=String(t.label||"MEMORY").toUpperCase(),r=new ct;r.name="memory-hologram",r.position.set(e.x,n,e.z),s.add(r);let o=[],a=[],l=[],c=(H,Z)=>(o.push(H),a.push(Z),new Ke(H,Z)),h=new xe(6544639),u=new xe(10980351),d=new xe(14218751),f={time:{value:0}},g=!1,y=0,m=[],p=[],b=0,S="none",x=!1,A=c(new ln(11,11),kr(uy,{time:f.time,strength:{value:.9},color:{value:h}}));A.rotation.x=-Math.PI/2,A.position.y=.18,r.add(A);let T=c(new ns(7.4,2.3,vc,56,1,!0),kr(hy,{time:f.time,strength:{value:.32},base:{value:h},top:{value:u}}));T.position.y=vc/2+.3,r.add(T);let I=c(new Nn(7.5,.055,6,128),kr(np,{time:f.time,strength:{value:1.6},color:{value:h}}));I.rotation.x=Math.PI/2,I.position.y=vc+.3,r.add(I);let v=c(new Nn(2.6,.05,6,96),kr(np,{time:f.time,strength:{value:2},color:{value:d}}));v.rotation.x=Math.PI/2,v.position.y=.55,r.add(v);let D=new br(1.35,1),w=new mo(D),R=new pi({color:10217983,transparent:!0,opacity:.75,blending:Xt,depthWrite:!1,toneMapped:!1}),L=new Cs(w,R);L.position.y=4.6,r.add(L),o.push(D,w),a.push(R);let V=new bt({color:4175871,transparent:!0,opacity:.12,blending:Xt,depthWrite:!1,toneMapped:!1}),C=c(new br(1.1,1),V);L.add(C);let N=160,U=new Float32Array(N);for(let H=0;H<N;H++)U[H]=H*.618033988749895%1;let E=new pt;E.setAttribute("position",new xt(new Float32Array(N*3),3)),E.setAttribute("seed",new xt(U,1)),E.boundingSphere=new dn(new B(0,vc/2,0),12);let G=kr(dy,{time:f.time,pointScale:{value:800},color:{value:d},strength:{value:.9}}),Y=new _n(E,G);Y.frustumCulled=!1,r.add(Y),o.push(E),a.push(G);let X=new ni(6478079,26,46,2);X.position.y=6,r.add(X);function ie(H,Z,W,J){let te=document.createElement("canvas");te.width=W,te.height=J;let pe=te.getContext("2d"),we=new Ps(te);we.colorSpace=zt,we.generateMipmaps=!1,we.minFilter=kt,l.push(we);let Pe=kr(Eu,{map:{value:we},time:f.time,glitch:{value:0},fade:{value:0},gain:{value:1.35},tint:{value:new xe(16777215)}},{side:Ln}),je=c(new ln(H,Z),Pe);return{canvas:te,context:pe,texture:we,material:Pe,mesh:je,text:"",targetFade:1,next:0}}let ce=ie(17.5,8.55,1024,500),be=[ie(8.6,2.1,640,156),ie(8.6,2.1,640,156)];ce.mesh.position.y=10.4,r.add(ce.mesh),be.forEach((H,Z)=>{H.mesh.position.y=Z?14.4:5.9,H.orbit=Z?-.22:.27,H.angle=Z*2.3,r.add(H.mesh)});function Le(H,Z,W){let{context:J,canvas:te}=ce,pe=te.width,we=te.height;J.clearRect(0,0,pe,we),J.fillStyle="rgba(48,150,214,0.14)",J.beginPath(),J.roundRect(14,14,pe-28,we-28,22),J.fill(),J.strokeStyle="rgba(150,230,255,0.85)",J.lineWidth=3;for(let[Pe,je,Xe,Je]of[[18,18,1,1],[pe-18,18,-1,1],[18,we-18,1,-1],[pe-18,we-18,-1,-1]])J.beginPath(),J.moveTo(Pe,je+Je*42),J.lineTo(Pe,je),J.lineTo(Pe+Xe*42,je),J.stroke();J.fillStyle="rgba(150,230,255,0.55)",J.fillRect(48,108,pe-96,2),J.font="600 27px "+$o,J.textBaseline="middle",J.fillStyle="rgba(160,232,255,0.92)",J.textAlign="left",J.fillText("\u258C "+i+(W?"  \xB7  "+String(Z+1).padStart(2,"0")+" / "+String(W).padStart(2,"0"):""),50,72),J.textAlign="right",J.font="500 25px "+$o,J.fillStyle="rgba(190,150,255,0.85)",J.fillText("0x"+cy(H||i).toUpperCase(),pe-52,72),J.textAlign="left",J.shadowColor="rgba(120,225,255,0.9)",J.shadowBlur=16,H?(J.font="600 54px "+$o,J.fillStyle="rgba(232,250,255,0.97)",ip(J,H,pe-110,4).forEach((je,Xe)=>J.fillText(je,54,172+Xe*72))):(J.font="600 40px "+$o,J.fillStyle="rgba(180,235,255,0.7)",J.fillText("\u25AE \u25AE \u25AF \u25AE \u25AF \u25AF \u25AE \u25AF \u25AE \u25AE \u25AF \u25AE",54,250)),J.shadowBlur=0,ce.texture.needsUpdate=!0}function Re(H,Z){let{context:W,canvas:J}=H,te=J.width,pe=J.height;W.clearRect(0,0,te,pe),W.fillStyle="rgba(48,150,214,0.12)",W.beginPath(),W.roundRect(6,6,te-12,pe-12,14),W.fill(),W.fillStyle="rgba(190,150,255,0.8)",W.fillRect(20,26,6,pe-52),W.font="600 42px "+$o,W.textBaseline="middle",W.textAlign="left",W.shadowColor="rgba(160,140,255,0.9)",W.shadowBlur=12,W.fillStyle="rgba(236,240,255,0.95)",W.fillText(Z?ip(W,Z,te-70,1)[0]:"\u25AF \u25AE \u25AF \u25AE \u25AF",42,pe/2),W.shadowBlur=0,H.texture.needsUpdate=!0}function oe(){return m.length?(b>=p.length&&(p=m.map((H,Z)=>Z).sort(()=>Math.random()-.5),b=0),m[p[b++]]):""}let ge=.8,ae=[3.5,6.5];function Se(H){let Z=oe();ce.text=Z,y++,Le(Z,m.indexOf(Z),m.length),ce.material.uniforms.glitch.value=H?1:0,ce.material.uniforms.fade.value=H?.35:1,ge=g?12:4.5+Math.random()*3}function he(H,Z){let W=be[H],J=oe();W.text=J,Re(W,J.length>46?J.slice(0,44).replace(/\s+\S*$/,"")+"\u2026":J),W.material.uniforms.glitch.value=Z?.7:0,W.material.uniforms.fade.value=Z?.3:1,ae[H]=g?15:6+Math.random()*4}Le("",0,0),be.forEach(H=>Re(H,"")),document.fonts?.ready?.then(()=>{x||(Le(ce.text,m.indexOf(ce.text),m.length),be.forEach(H=>Re(H,H.text)))});let ue=new B;return{group:r,setTexts(H,Z="live"){let W=fy(H),J=W.length!==m.length||W.some((te,pe)=>te!==m[pe]);m=W,S=W.length?Z:"none",J&&(p=[],b=0,(!ce.text||!m.includes(ce.text))&&(ge=Math.min(ge,.6)))},setPointScale(H){G.uniforms.pointScale.value=H},setReducedMotion(H){g=!!H},update(H,Z,W){if(x)return;let J=W&&!g,te=J?Math.min(.1,Math.max(0,H)):0;f.time.value+=te,ge-=H,ge<=0&&Se(J),ae.forEach((pe,we)=>{ae[we]=pe-H,ae[we]<=0&&he(we,J)});for(let pe of[ce,...be]){let we=pe.material.uniforms;we.glitch.value=Math.max(0,we.glitch.value-H*2.4),we.fade.value=Math.min(1,we.fade.value+H*1.8),pe.mesh.quaternion.copy(Z.quaternion)}be.forEach(pe=>{pe.angle+=pe.orbit*te,pe.mesh.position.x=Math.cos(pe.angle)*6.4,pe.mesh.position.z=Math.sin(pe.angle)*6.4,ue.copy(pe.mesh.position).add(r.position).sub(Z.position).normalize();let we=-(ue.x*Math.cos(pe.angle)+ue.z*Math.sin(pe.angle));pe.material.uniforms.gain.value=1.35*wt.clamp(.35+we*.9,.15,1.2)}),ce.mesh.position.y=9.6+Math.sin(f.time.value*.9)*.22,L.rotation.y+=te*.7,L.rotation.x+=te*.31,C.rotation.y-=te*1.1,I.rotation.z+=te*.18,v.rotation.z-=te*.42,X.intensity=26+Math.sin(f.time.value*2.1)*6+ce.material.uniforms.glitch.value*22},stats:()=>({artifacts:m.length,source:S,switches:y,shown:ce.text?m.indexOf(ce.text):-1}),dispose(){x||(x=!0,r.removeFromParent(),o.forEach(H=>H.dispose()),a.forEach(H=>H.dispose()),l.forEach(H=>H.dispose()),X.dispose())}}}var py=`uniform sampler2D tDiffuse;uniform float time,vignette,grain,aberration,streak,shafts,letterbox,aspect,contrast,saturation;
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
gl_FragColor=vec4(c,1.0);}`;function rp(s){let e={tDiffuse:{value:null},time:s,vignette:{value:.5},grain:{value:.03},aberration:{value:.0024},streak:{value:.16},shafts:{value:0},letterbox:{value:0},aspect:{value:1.7777777777777777},contrast:{value:.22},saturation:{value:1.08},lightPos:{value:new _e(.5,.5)},shaftColor:{value:new xe(12376319)},shadowTint:{value:new xe(6686)},highlightTint:{value:new xe(1444864)}},t=new Or(new mt({uniforms:e,vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.0);}",fragmentShader:py}));t.enabled=!1;let n=new B,i=new xe(16761992),r=new xe(12376319),o=!1;return{pass:t,setCinematic(a){o=!!a},update(a,l,c,{day:h,dusk:u,cloud:d,animated:f}){e.aspect.value=l.aspect;let g=o?1:0;e.letterbox.value=f?e.letterbox.value+(g-e.letterbox.value)*Math.min(1,a*2.2):g,n.copy(c).multiplyScalar(1e3).add(l.position).project(l);let y=(n.x+1)/2,m=(n.y+1)/2,p=n.z<1?Math.max(0,1-Math.max(0,Math.abs(y-.5)-.5,Math.abs(m-.5)-.5)*4):0;e.lightPos.value.set(y,m),e.shafts.value=p*(.06+u*.14+(1-h)*.03)*(1-d*.75),e.shaftColor.value.copy(r).lerp(i,u),e.shadowTint.value.setRGB(0,.018+.01*(1-h),.024+.01*(1-h)),e.highlightTint.value.setRGB(.02+u*.03,.01+u*.006,0)},stats:()=>({enabled:t.enabled,letterbox:+e.letterbox.value.toFixed(2),shafts:+e.shafts.value.toFixed(2)}),dispose(){t.material.dispose(),t.dispose?.()}}}var Au=`float hash2(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
float vnoise(vec2 p){vec2 i=floor(p),f=fract(p);vec2 u=f*f*(3.0-2.0*f);
return mix(mix(hash2(i),hash2(i+vec2(1.0,0.0)),u.x),mix(hash2(i+vec2(0.0,1.0)),hash2(i+vec2(1.0,1.0)),u.x),u.y);}`,Tu="varying vec3 vWorld;varying vec2 vUv;void main(){vUv=uv;vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;gl_Position=projectionMatrix*viewMatrix*w;}",my="float fbm(vec2 p){float v=0.0,a=0.5;for(int i=0;i<5;i++){if(float(i)>=octaves)break;v+=a*vnoise(p);p=p*2.03+vec2(1.7,9.2);a*=0.5;}return v;}",gy=`uniform float time,day,dusk,cloud,flash,octaves;uniform vec3 zenith,horizon,haze,warm,sunDir,glow;varying vec3 vWorld;${Au}${my}
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
gl_FragColor=vec4(col,1.0);}`,op={vertex:`attribute float phase,speed,size;uniform float time,pointScale,cloud;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
float tw=0.65+0.35*sin(time*speed+phase);gl_PointSize=size*tw*pointScale;vAlpha=tw*smoothstep(0.02,0.16,position.y/1450.0)*(1.0-cloud*0.85);
float k=fract(phase*3.1);vTint=k>0.86?vec3(1.0,0.84,0.66):k>0.7?vec3(0.7,0.82,1.0):vec3(0.82,0.9,1.0);}`,fragment:"varying float vAlpha;varying vec3 vTint;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;float a=pow(1.0-d,1.6)*vAlpha;gl_FragColor=vec4(vTint*a,1.0);}"},ap={vertex:`varying vec3 vWorld;
#include <fog_pars_vertex>
void main(){vec4 w=modelMatrix*vec4(position,1.0);vWorld=w.xyz;vec4 mvPosition=viewMatrix*w;gl_Position=projectionMatrix*mvPosition;
#include <fog_vertex>
}`,fragment:`uniform float time,day;uniform vec3 deep,shallow,sky,sunDir,sunColor,glow;uniform vec4 island;varying vec3 vWorld;
#include <fog_pars_fragment>
${Au}
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
}`},xy=`uniform float time,strength;uniform vec3 color;uniform vec4 island;varying vec3 vWorld;${Au}
void main(){vec2 p=vWorld.xz;float n=vnoise(p*0.012+vec2(time*0.017,-time*0.011))*0.6+vnoise(p*0.031-vec2(time*0.02,time*0.013))*0.4;
float mist=smoothstep(0.32,0.82,n);vec2 d2=max(abs(p-island.xy)-island.zw,0.0);float outside=smoothstep(0.0,70.0,length(d2));
float far=1.0-smoothstep(500.0,850.0,length(p));gl_FragColor=vec4(color,mist*outside*far*strength);}`,lp={vertex:`attribute vec3 seed;uniform float time,pointScale;varying float vAlpha;
void main(){vec3 p=position;p.x+=sin(time*0.11*seed.x+seed.y*6.28)*9.0+time*0.35*(seed.z-0.5);p.y+=sin(time*0.13+seed.z*6.28)*3.0;
p.z+=cos(time*0.09*seed.y+seed.x*6.28)*9.0;p.x=mod(p.x+90.0,180.0)-90.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=(0.06+0.11*seed.x)*pointScale/max(1.0,-mv.z);vAlpha=0.55+0.45*sin(time*(1.0+seed.y)+seed.z*6.28);}`,fragment:`uniform vec3 color;uniform float strength;varying float vAlpha;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(color*pow(1.0-d,2.2)*vAlpha*strength,1.0);}`},_y=`uniform float strength;uniform vec3 color;varying vec2 vUv;
void main(){float along=pow(clamp(1.0-vUv.x,0.0,1.0),2.4);float across=pow(clamp(1.0-abs(vUv.y-0.5)*2.0,0.0,1.0),1.7);gl_FragColor=vec4(color*along*across*strength,1.0);}`,vy=`uniform vec3 color;uniform float strength;varying vec2 vUv;varying vec3 vNormal,vView;
void main(){float rim=clamp(1.0-abs(dot(normalize(vNormal),normalize(vView))),0.0,1.0);float a=pow(clamp(vUv.y,0.0,1.0),1.6)*(0.35+0.65*rim)*strength;gl_FragColor=vec4(color*a,1.0);}`,cp={vertex:"varying vec2 vUv;varying float vRim,vNear;void main(){vUv=uv;vec3 n=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vRim=abs(dot(n,normalize(-mv.xyz)));vNear=smoothstep(40.0,160.0,-mv.z);gl_Position=projectionMatrix*mv;}",fragment:"uniform vec3 color;uniform float strength;varying vec2 vUv;varying float vRim,vNear;void main(){float along=pow(clamp(1.0-vUv.y,0.0,1.0),1.6)*smoothstep(0.0,0.03,vUv.y);gl_FragColor=vec4(color*along*pow(clamp(vRim,0.0,1.0),2.5)*vNear*strength,1.0);}"},hp={vertex:`attribute float phase;uniform float time,pointScale;varying float vA;void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
vA=0.22+0.78*pow(max(0.0,sin(time*1.9+phase)),10.0);gl_PointSize=clamp(1.5*pointScale/max(1.0,-mv.z),2.0,14.0);}`,fragment:"varying float vA;void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;gl_FragColor=vec4(vec3(3.2,0.35,0.25)*pow(1.0-d,2.0)*vA,1.0);}"};function up(s,e={}){let t=[],n=[],i=new ct;i.name="city-atmosphere",s.add(i);let r=(z,$)=>(t.push(z),n.push($),new Ke(z,$)),o={value:0},a={value:900},l={value:0},c={value:0},h={value:.25},u={value:0},d={value:5},f=new xe(4860440),g=(e.sunDirection||new B(-70,145,85)).clone().normalize(),y=new xe(330010),m=new xe(1323596),p=new xe(8011050),b=new xe(s.fog?.color||528926),S=(z,$,j,ne={})=>new mt({uniforms:j,vertexShader:z,fragmentShader:$,...ne}),x={transparent:!0,depthWrite:!1,blending:Xt},A=r(new gi(1500,48,24),S(Tu,gy,{time:o,day:l,dusk:c,cloud:h,flash:u,octaves:d,glow:{value:f},zenith:{value:y},horizon:{value:m},haze:{value:b},warm:{value:p},sunDir:{value:g}},{side:tn,depthWrite:!1,fog:!1}));A.frustumCulled=!1,A.renderOrder=-10,i.add(A);let T=1400,I=new Float32Array(T*3),v=new Float32Array(T),D=new Float32Array(T),w=new Float32Array(T),R=12345,L=()=>(R=Math.imul(R,1664525)+1013904223>>>0)/4294967296;for(let z=0;z<T;z++){let $=L()*Math.PI*2,j=.03+L()*.97,ne=Math.sqrt(1-j*j);I.set([Math.cos($)*ne*1450,j*1450,Math.sin($)*ne*1450],z*3),v[z]=L()*Math.PI*2,D[z]=.4+L()*1.6,w[z]=8e-4+L()*L()*.0022}let V=new pt;V.setAttribute("position",new xt(I,3)),V.setAttribute("phase",new xt(v,1)),V.setAttribute("speed",new xt(D,1)),V.setAttribute("size",new xt(w,1));let C=S(op.vertex,op.fragment,{time:o,pointScale:a,cloud:h},{...x,fog:!1}),N=new _n(V,C);N.frustumCulled=!1,N.renderOrder=-9,i.add(N),t.push(V),n.push(C);let U=r(new ln(1800,1800),S(ap.vertex,ap.fragment,{time:o,day:l,glow:{value:f},island:{value:new _t(0,-7,85,87)},deep:{value:new xe(398368)},shallow:{value:new xe(930640)},sky:{value:m.clone().multiplyScalar(.7)},sunDir:{value:g},sunColor:{value:new xe(14676223)},fogColor:{value:new xe},fogDensity:{value:0},fogNear:{value:1},fogFar:{value:1e3}},{fog:!0}));U.rotation.x=-Math.PI/2,U.position.y=-3.2,i.add(U);let E=r(new ln(1900,1900),S(Tu,xy,{time:o,strength:{value:.55},color:{value:new xe(1717320)},island:{value:new _t(0,-7,85,87)}},{transparent:!0,depthWrite:!1,fog:!1}));E.rotation.x=-Math.PI/2,E.position.y=-2.4,i.add(E);let G=500,Y=new Float32Array(G*3),X=new Float32Array(G*3);for(let z=0;z<G;z++)Y.set([(L()-.5)*180,2+L()*L()*75,-95+L()*170],z*3),X.set([L(),L(),L()],z*3);let ie=new pt;ie.setAttribute("position",new xt(Y,3)),ie.setAttribute("seed",new xt(X,3));let ce=S(lp.vertex,lp.fragment,{time:o,pointScale:a,color:{value:new xe(10475775)},strength:{value:.32}},x),be=new _n(ie,ce);be.frustumCulled=!1,i.add(be),t.push(ie),n.push(ce);let Le=new ct;Le.position.set(0,86.5,-12),i.add(Le);let Re=S(Tu,_y,{strength:{value:.45},color:{value:new xe(9430783)}},{...x,side:Nt}),oe=new ln(100,5.2);oe.translate(50,0,0),t.push(oe),n.push(Re);let ge=new Ke(oe,Re),ae=new Ke(oe,Re);ae.rotation.x=Math.PI/2;let Se=new ct;Se.add(ge,ae),Se.rotation.z=-.07,Le.add(Se);let he=new bt({color:12579583,toneMapped:!1}),ue=r(new gi(.9,16,12),he);Le.add(ue);let H=(e.lamps||[]).slice(0,64),Z=S("varying vec2 vUv;varying vec3 vNormal,vView;void main(){vUv=uv;vNormal=normalize(normalMatrix*normal);vec4 mv=modelViewMatrix*vec4(position,1.0);vView=-mv.xyz;gl_Position=projectionMatrix*mv;}",vy,{color:{value:new xe(16767392)},strength:{value:.3}},{...x,side:Nt}),W=new Is(2.7,6.1,18,1,!0),J=new vn(W,Z,Math.max(1,H.length));t.push(W),n.push(Z);let te=new St;H.forEach((z,$)=>{te.position.set(z.x+1.9*Math.cos(z.angle||0),3.35,z.z-1.9*Math.sin(z.angle||0)),te.updateMatrix(),J.setMatrixAt($,te.matrix)}),J.count=H.length,J.instanceMatrix.needsUpdate=!0,i.add(J);let pe=S(cp.vertex,cp.fragment,{color:{value:new xe(11128063)},strength:{value:0}},{...x,side:Nt,fog:!1}),we=new ns(11,.7,280,20,1,!0);we.translate(0,140,0),t.push(we),n.push(pe);let Pe=[[-72,-168,0],[64,-186,2.1],[4,-232,4.2]].map(([z,$,j])=>{let ne=new Ke(we,pe);return ne.position.set(z,-3,$),ne.rotation.order="YXZ",ne.userData.offset=j,ne.frustumCulled=!1,i.add(ne),ne}),je=96,Xe=new Float32Array(je*3),Je=new Float32Array(je);for(let z=0;z<je;z++)Je[z]=L()*Math.PI*2;let q=new pt;q.setAttribute("position",new xt(Xe,3)),q.setAttribute("phase",new xt(Je,1)),q.setDrawRange(0,0);let it=S(hp.vertex,hp.fragment,{time:o,pointScale:a},{...x,fog:!1}),Ue=new _n(q,it);Ue.frustumCulled=!1,i.add(Ue),t.push(q),n.push(it);let M=rp(o),_=M.pass,O=!1,k="high",P=0;return{post:_,sea:U,setLighting(z,$,j){g.copy(j).normalize(),l.value=z,c.value=Math.min(1,$)*(1-z),y.set(330010).lerp(new xe(3108776),z).lerp(new xe(1909062),c.value*.8),m.set(1323596).lerp(new xe(11126484),z).lerp(new xe(12609598),c.value*.85),p.set($>.1?16747077:8011050),f.set(4860440).lerp(new xe(7027234),c.value),b.copy(s.fog.color),N.visible=z<.35,U.material.uniforms.sky.value.copy(m).lerp(y,c.value*.65).multiplyScalar(.7),U.material.uniforms.deep.value.set(398368).lerp(new xe(801368),z),U.material.uniforms.shallow.value.set(930640).lerp(new xe(2321036),z),E.material.uniforms.strength.value=.55*(1-z*.55),U.material.uniforms.sunColor.value.set(14676223).lerp(new xe(16752736),c.value)},setWeather(z){h.value=z==="rain"?.95:z==="fog"?.7:.25},setFlash(z){u.value=Math.max(0,Math.min(1,z))},setCinematic(z){M.setCinematic(z)},setAviation(z){let $=Math.min(je,z.length);for(let j=0;j<$;j++)Xe.set([z[j].x,z[j].y,z[j].z],j*3);q.attributes.position.needsUpdate=!0,q.setDrawRange(0,$)},setTier(z){k=z;let $=k!=="low";E.visible=$,be.visible=$,J.visible=$,Pe.forEach(j=>{j.visible=$}),d.value=k==="low"?3:k==="medium"?4:5,_.enabled=k==="high"||k==="ultra"},setBusy(z){O=!!z},setPointScale(z){a.value=z},update(z,$,j,ne){let de=ne?Math.min(.1,Math.max(0,z)):0;o.value+=de,s.fog&&(U.material.uniforms.fogColor.value.copy(s.fog.color),s.fog.isFogExp2&&(U.material.uniforms.fogDensity.value=s.fog.density)),U.position.x=j.position.x,U.position.z=j.position.z;let Ce=O?1.15:.32,Me=Le.userData.rate??Ce;Le.userData.rate=Me+(Ce-Me)*Math.min(1,de*1.5),Se.rotation.y+=de*Le.userData.rate,Re.uniforms.strength.value=O?.85:.45,ue.scale.setScalar(1+Math.sin(o.value*(O?6:2.2))*.18),P+=de;let ve=1-l.value;pe.uniforms.strength.value=ve*ve*.075*(1-c.value*.7)*(1-h.value*.45),Pe.forEach((Be,Ee)=>{let Ve=Be.userData.offset;Be.rotation.y=Math.PI+Math.sin(P*(.11+Ee*.025)+Ve)*.95,Be.rotation.x=.22+.14*Math.sin(P*.13+Ve*1.7)}),_.enabled&&M.update(de,j,g,{day:l.value,dusk:c.value,cloud:h.value,animated:ne})},stats:()=>({tier:k,busy:O,stars:T,dust:G,lamps:H.length,post:_.enabled,clouds:+h.value.toFixed(2),searchlights:Pe.length,aviation:q.drawRange.count,cinema:M.stats()}),dispose(){i.removeFromParent(),J.dispose(),t.forEach(z=>z.dispose()),n.forEach(z=>z.dispose()),M.dispose()}}}var dp=[{points:[[-62,34,-62],[8,44,-74],[62,38,-22],[56,47,42],[-8,41,62],[-64,36,12]],speed:9.5},{points:[[30,56,-12],[0,62,18],[-30,58,-12],[0,66,-42]],speed:8},{points:[[42,29,-56],[-28,33,-42],[-52,27,20],[18,31,52],[60,33,8]],speed:11}];function fp(s,e={}){let t=new ct;t.name="city-drones",s.add(t);let n=[],i=[],r=[],o=new B,a=new B,l=new B,c=new gi(.16,8,6);n.push(c);let h=x=>{let A=new bt({color:x,toneMapped:!1});return i.push(A),A},u=h(16730684),d=h(5046154),f=h(16777215);[...dp,...dp].forEach((x,A)=>{let T=new yr(x.points.map(w=>new B(w[0],w[1]+(A>=3?8:0),w[2])),!0,"centripetal",.6),I=new ct;I.name="city-drone-"+(A+1),I.visible=!1;let v=new ct;I.add(v);let D=[new Ke(c,u),new Ke(c,d),new Ke(c,f)];D[0].position.set(-2.6,.8,0),D[1].position.set(2.6,.8,0),D[2].position.set(0,2.1,-.4),I.add(...D),t.add(I),r.push({root:I,body:v,lights:D,curve:T,length:T.getLength(),speed:x.speed,t:A*.37%1,rotors:[],roll:0,dir:1,waiting:0})});let g=!0,y="high",m=!1,p=new Map,b=x=>{if(!p.has(x)){let A=x.clone();e.surfaces?.apply(A,"object",x.name),p.set(x,A)}return p.get(x)};function S(x,A){let{curve:T,root:I}=x,v=x.t;if(x.t=(x.t+A*x.speed*x.dir/x.length+1)%1,T.getPointAt(x.t,I.position),T.getTangentAt(x.t,o),o.multiplyScalar(x.dir),x.collider&&e.traffic){let R=x.collider,L=e.traffic;T.getPointAt((x.t+x.dir*20/x.length+1)%1,l),x.avoidTime=Math.max(0,(x.avoidTime||0)-A);let V=Math.max(I.position.y,L.ceiling(R,I.position.x,I.position.z),L.ceiling(R,l.x,l.z),x.avoidTime?x.avoidHeight:0);I.position.y=R.y+wt.clamp(V-R.y,-A*5,A*7);let C=Math.atan2(o.x,o.z),N=R.heading+Math.atan2(Math.sin(C-R.heading),Math.cos(C-R.heading))*Math.min(1,A*3),U={...I.position,heading:N};if(x.obstruction=L.obstruction(R,R,U),x.obstruction){let E=L.body(x.obstruction),G=x.obstruction.startsWith("drone-")&&(Math.abs(R.y-E.y)>1?R.y<E.y:R.id<E.id);G||(x.avoidHeight=Math.min(170,R.y+8),x.avoidTime=8),U={...R,y:G?R.y:Math.min(170,R.y+A*5)},x.t=v}L.propose(R,U,(E,G)=>{E?x.waiting=0:(x.t=v,x.waiting+=A,x.waiting>2.5&&(x.dir=-x.dir,x.waiting=0)),I.position.set(G.x,G.y,G.z),I.rotation.set(0,G.heading,0)})}T.getTangentAt((x.t+.015)%1,a),x.collider||I.rotation.set(0,Math.atan2(o.x,o.z),0);let D=Math.atan2(a.x,a.z)-Math.atan2(o.x,o.z),w=Math.atan2(Math.sin(D),Math.cos(D));x.roll+=(wt.clamp(w*12,-.55,.55)-x.roll)*(A>0?Math.min(1,A*3):1),x.body.rotation.z=x.roll}return{setTier(x){y=x},setTemplate(x){if(!x)return;m=!0;let A=p;p=new Map;for(let[T,I]of r.entries()){I.body.clear(),I.rotors.length=0;let v=x.clone(!0);v.scale.setScalar(1.9),v.updateMatrixWorld(!0);let D=new en().setFromObject(v);if(D.expandByScalar(.5),D.min.y-=1,D.max.y+=1,v.traverse(w=>{w.isMesh&&(w.castShadow=!1,w.receiveShadow=!1,w.material=Array.isArray(w.material)?w.material.map(b):b(w.material)),/^rotor_/.test(w.name)&&I.rotors.push(w)}),I.body.add(v),I.root.visible=T<(y==="low"?2:y==="medium"?3:6),e.traffic&&!I.collider){S(I,0);let w=Hi({min:D.min.toArray(),max:D.max.toArray()});I.root.position.y=Math.max(I.root.position.y,e.traffic.ceiling(w,I.root.position.x,I.root.position.z)),I.collider=e.traffic.register("drone-"+T,w,{...I.root.position,heading:I.root.rotation.y},0),I.collider||(I.root.visible=!1)}}A.forEach(T=>T.dispose())},update(x,A,T){g=T;let I=T?Math.min(.1,Math.max(0,x)):0;r.forEach((v,D)=>{if(v.root.visible=m&&D<(y==="low"?2:y==="medium"?3:6)&&(!e.traffic||!!v.collider),v.collider&&(v.root.visible&&!v.collider.enabled&&!e.traffic.relocate(v.collider,v.collider)&&(v.root.visible=!1),v.collider.enabled=v.root.visible),!v.root.visible)return;S(v,I),v.rotors.forEach((R,L)=>{R.rotation.y+=I*(L%2?-46:46)});let w=Math.sin(A*5+D*1.7);v.lights[0].visible=v.lights[1].visible=!T||w>-.2,v.lights[2].visible=!T||w>.93,v.lights[2].scale.setScalar(T?1.6:1)})},stats:()=>({drones:r.filter(x=>x.root.visible).length,animated:g,positions:r.map(x=>x.root.position.toArray().map(A=>Math.round(A))),obstructions:r.map(x=>x.obstruction)}),dispose(){t.removeFromParent(),n.forEach(x=>x.dispose()),i.forEach(x=>x.dispose()),p.forEach(x=>x.dispose()),r.forEach(x=>{x.collider&&e.traffic.remove(x.collider.id)}),r.length=0}}}var En={xs:[-67,-18,18,67],zs:[-77,-32,13,59],halfWidth:6,laneHalfWidth:3.3,minX:-85,maxX:85,minZ:-94,maxZ:80},ut={ground:0,road:.12,pavement:.5,quay:.08,room:.16,gallery:4.16},pn=["agent","memory","missions"].map((s,e)=>{let t=[0,-43,43][e],n=73;return{id:s,x:t,z:n,width:12,depth:12,front:-1,doorZ:n-6,liftX:t+4,liftZ:n+3}}),Zn=[{id:"infra",x:-67,z:-53,platformX:-74,platformZ:-53,angle:Math.PI/2},{id:"memory",x:-67,z:-10,platformX:-74,platformZ:-10,angle:Math.PI/2},{id:"graph",x:-43,z:59,platformX:-43,platformZ:64.5,angle:0},{id:"operations",x:0,z:59,platformX:0,platformZ:64.5,angle:0},{id:"missions",x:43,z:59,platformX:43,platformZ:64.5,angle:0},{id:"integrations",x:67,z:-10,platformX:74,platformZ:-10,angle:-Math.PI/2},{id:"agent",x:0,z:-77,platformX:0,platformZ:-84,angle:Math.PI}],Ru=[[-67,-77],[-67,59],[67,59],[67,-77]],wi={x:78,z:30},yc=[{x:30.5,z:-55,scale:1},{x:49,z:-55,scale:1.25},{x:-3,z:-57,scale:1.2}];function pp(s,e,t=En.halfWidth){return s>=En.minX&&s<=En.maxX&&e>=En.minZ&&e<=En.maxZ&&(En.xs.some(n=>Math.abs(s-n)<=t)||En.zs.some(n=>Math.abs(e-n)<=t))}function mp(s,e){return pp(s,e)?pp(s,e,En.laneHalfWidth)?ut.road:ut.pavement:ut.ground}function yy(s,e,t,n=0){return e>=s.x-5.85+n&&e<=s.x+2.4-n&&t>=s.z+n&&t<=s.z+5.85-n}function gp(s,e,t,n=0){return Math.abs(e-s.liftX)<=1.6-n&&Math.abs(t-s.liftZ)<=1.6-n}function xp(){let s=[],e=[2.4,1.4,5.6,4.6],t=(n,i,r,o)=>{r>n&&o>i&&s.push({x:(n+r)/2,z:(i+o)/2,sx:(r-n)/4,sz:(o-i)/4})};for(let n=-4;n<=4;n+=4)for(let i=-4;i<=4;i+=4){let[r,o,a,l]=[n-2,i-2,n+2,i+2],[c,h,u,d]=[Math.max(r,e[0]),Math.max(o,e[1]),Math.min(a,e[2]),Math.min(l,e[3])];if(c>=u||h>=d){t(r,o,a,l);continue}t(r,o,c,l),t(u,o,a,l),t(c,o,u,h),t(c,d,u,l)}return s}function Cu(s,e,t,n){return yy(s,e,t,.25)?!0:n?e>=s.x+1.8&&e<=s.liftX+1.25&&Math.abs(t-s.liftZ)<1.25:!1}var Pu={agent:[[-8,-8,8,8]],memory:[[-11.4,-10.4,11.4,8.4]],integrations:[[-11.4,-4.9,-4.6,4.9],[4.6,-4.9,11.4,4.9],[-4.8,-.8,-3.2,.8],[3.2,-.8,4.8,.8]],missions:[[-13.4,-8.9,13.4,6.9]],infra:[[-10.7,-9.4,10.7,3.4],[-10,-2.6,10,8.5]]},My={agent:[23,23],memory:[26,22],integrations:[26,16],missions:[30,21],infra:[25,23],graph:[24,24],operations:[9,9]};function _p(s,e,t){for(let n of t){let i=My[n.id];if(i&&Math.abs(s-n.x)<i[0]/2&&Math.abs(e-n.z)<i[1]/2)return .88}return 0}function by(s,e,t){for(let n of t){let i=s-n.x,r=e-n.z;if(n.id==="graph"&&Math.hypot(i,r)<10.2||n.id==="operations"&&Math.hypot(i,r)<3||(Pu[n.id]||[]).some(([o,a,l,c])=>i>o&&i<l&&r>a&&r<c))return!0}return yc.some(n=>Math.abs(s-n.x)<6&&Math.abs(e-n.z)<5)}function Hs(s,e){return pn.find(t=>Math.abs(s-t.x)<t.width/2&&Math.abs(e-t.z)<t.depth/2)}function fs(s,e){if(s>=-81.5&&s<=-78.5&&e>=46&&e<=52)return(52-e)/3;if(s>=-78.3&&s<=-75.3&&e>=46&&e<=52)return Math.min(2,Math.ceil((52-e)*2)/6);if(s>=-82&&s<=-75&&e>=34&&e<46)return 2;if(s>=-80&&s<=-77&&e>=28&&e<34)return(e-28)/3;if(Hs(s,e))return ut.room;for(let t of Zn){let n=!!t.angle&&Math.abs(t.angle)!==Math.PI;if(Math.abs(s-t.platformX)<(n?2:4.5)&&Math.abs(e-t.platformZ)<(n?4.5:2))return ut.pavement+.3}return Math.max(mp(s,e),e>=74&&e<=80&&s>=-80&&s<=80?ut.quay:ut.ground)}function vp(s,e,t,n,i){if(Math.abs(s)>84||e<-90||e>79)return!1;let r=Hs(s,e),o=Hs(t.x,t.z);if(r||o){let a=r||o;return!(r?.id!==o?.id&&(Math.abs(s-a.x)>1.15||Math.abs(t.x-a.x)>1.15||Math.min((e-a.doorZ)*a.front,(t.z-a.doorZ)*a.front)<-.9||!n(a.id))||r&&(Math.abs(s-a.x)>5.55||(e-a.z)*a.front<-5.55))}return!by(s,e,i)}function Mc(s,e=new Date){let t=s==="day"?12:s==="evening"?18.5:s==="night"?0:e.getHours()+e.getMinutes()/60;return{hour:t,amount:Math.max(0,Math.min(1,Math.sin((t-6)/12*Math.PI)*1.5)),evening:Math.max(0,1-Math.abs(t-18.5)/2)}}var ps=[{id:"repair-bay",district:"infra",x:-28,z:-62,role:"technician"},{id:"parcel-sorter",district:"missions",x:35,z:49,role:"courier"},{id:"relay-mast",district:"integrations",x:57,z:-20,role:"archivist"},{id:"kinetic-fountain",district:"graph",x:-28,z:47,role:"archivist"},{id:"glass-garden",district:"graph",x:40,z:-68,role:"technician"},{id:"meeting-charge",district:"operations",x:8,z:46,role:"courier"}];function yp(s,e){let t=[],n=new Map,i=-1;for(let o of[-73,-61,-24,-12,12,24,61,73])for(let a of[-83,-71,-38,-26,7,19,53,65])t.push({x:o,z:a,y:e(o,a),heading:0});function r(o,a){i!==s.revision()&&(n.clear(),i=s.revision());let l=o.circles.map(b=>[b.x,b.z,b.r].join(",")).join(";")+":"+o.maxY;if(!n.has(l)){let b=t.filter(x=>s.clear(o,x,x,!1)),S=b.map(()=>[]);for(let x=0;x<b.length;x++)for(let A=x+1;A<b.length;A++){let T=b[x],I=b[A],v=Math.hypot(T.x-I.x,T.z-I.z);v<48&&s.clear(o,T,I,!1)&&(S[x].push([A,v]),S[A].push([x,v]))}n.set(l,{nodes:b,edges:S})}let c=n.get(l),h=[...c.nodes,{x:o.x,y:o.y,z:o.z,heading:o.heading},{...a,heading:0}],u=h.length-2,d=h.length-1,f=c.edges.map(b=>[...b]);f.push([],[]);for(let b of[u,d])for(let S=0;S<b;S++){let x=h[b],A=h[S],T=Math.hypot(x.x-A.x,x.z-A.z);(T<48||S===u)&&s.clear(o,x,A,!1)&&(f[b].push([S,T]),f[S].push([b,T]))}let g=h.map(()=>1/0),y=h.map(()=>-1),m=new Set(h.map((b,S)=>S));for(g[u]=0;m.size;){let b=-1;for(let S of m)(b<0||g[S]<g[b])&&(b=S);if(!Number.isFinite(g[b])||b===d)break;m.delete(b);for(let[S,x]of f[b])g[b]+x<g[S]&&(g[S]=g[b]+x,y[S]=b)}if(!Number.isFinite(g[d]))return[];let p=[];for(let b=d;b!==u;b=y[b]){if(b<0)return[];p.unshift(h[b])}return p}return{route:r,points:t}}function Mp(s){let e=new ct;e.name="city-conversations",s.add(e);let t=new Nn(1,.025,5,40,Math.PI*1.45),n=new Ls(.92,1,48),i=new B(0,0,1),r=new B,o=[],a=0;for(let l=0;l<8;l++){let c=[];for(let h=0;h<4;h++){let u=new bt({color:8445392,transparent:!0,opacity:0,blending:Xt,depthWrite:!1,side:Nt,toneMapped:!1}),d=new Ke(h===3?n:t,u);d.visible=!1,e.add(d),c.push(d)}o.push({meshes:c,age:10,from:null,to:null})}return{send(l,c,h="ambient"){let u=o.find(d=>d.age>=2.4);return u?(Object.assign(u,{from:l,to:c,age:0,source:h}),a++,u.meshes.forEach(d=>d.material.color.setHex(h==="live"?16105851:8445392)),!0):!1},update(l,c=!0){for(let h of o){if((!c||h.from?.enabled===!1||h.to?.enabled===!1)&&(h.age=10),h.age+=l,h.meshes.forEach(m=>{m.visible=!1}),h.age>=2.4)continue;let u=h.from,d=h.to,f=d.x-u.x,g=d.z-u.z;r.set(f,d.y+Math.min(2,d.maxY*.7)-(u.y+Math.min(2,u.maxY*.7)),g).normalize();for(let m=0;m<3;m++){let p=(h.age-m*.18)/1.55;if(p<0||p>1)continue;let b=h.meshes[m];b.visible=!0,b.position.set(u.x+f*p,wt.lerp(u.y+Math.min(2,u.maxY*.7),d.y+Math.min(2,d.maxY*.7),p)+Math.sin(p*Math.PI)*.65,u.z+g*p),b.quaternion.setFromUnitVectors(i,r),b.rotateZ(m*.65),b.scale.setScalar(.25+Math.sin(p*Math.PI)*.65),b.material.opacity=Math.sin(Math.PI*p)*.75}let y=(h.age-1.55)/.85;if(y>0){let m=h.meshes[3];m.visible=!0,m.position.set(d.x,d.y+Math.min(2,d.maxY*.7),d.z),m.quaternion.setFromUnitVectors(i,r),m.scale.setScalar(1.25*(1-y)+.12),m.material.opacity=Math.sin(y*Math.PI)*.85}}},clear(){o.forEach(l=>{l.age=10,l.meshes.forEach(c=>{c.visible=!1})})},stats:()=>({sent:a,active:o.filter(l=>l.age<2.4).length,sources:o.filter(l=>l.age<2.4).map(l=>l.source)}),dispose(){e.removeFromParent(),t.dispose(),n.dispose(),o.forEach(l=>l.meshes.forEach(c=>c.material.dispose()))}}}var Iu=(s,e,t)=>s+Math.atan2(Math.sin(e-s),Math.cos(e-s))*Math.min(1,t*5);function bp(s,{traffic:e,camera:t,floor:n,onSelect:i,onDemonstrate:r,active:o}){let a=[],l=new Map,c=[],h=[],u=yp(e,n),d=Mp(s),f=0,g=null,y=!1,m="high",p=!1,b=0,S="",x=()=>m==="low"?3:m==="medium"?10:19;function A(C){C.slot&&(C.slot.owner=null,C.slot=null),C.path=[],C.goal=null}function T(C){return C?{id:C.id,role:C.role,state:C.state,source:C.source||"ambient",guided:!!C.guide}:null}function I(C){g=C?.id||null,S=C?.state||"",i?.(T(C))}function v(C,N,U){let E="resident-"+N,G=Hi(U,1.15),Y=["courier","technician","archivist"][N%3],X=null;G.circles=[{x:0,z:0,r:G.reach}];let ce=[...c.filter(Le=>Le.role===Y).map(Le=>({x:Le.x,z:Le.z+2.2,y:n(Le.x,Le.z+2.2),heading:Math.PI})),...u.points.map((Le,Re)=>u.points[(N*7+Re)%u.points.length])];for(let Le of ce)if(X=e.register(E,G,Le,1),X)break;if(!X)return C.node.visible=!1,null;let be={...C,id:E,index:N,body:X,role:Y,state:"idle",path:[],until:N*.3,trip:N,cooldown:8+N*.7,source:"ambient",carrying:!1,parcels:[]};return C.node.traverse(Le=>{Le.isMesh&&/parcel/.test(Le.name)&&(be.parcels.push(Le),Le.visible=!1)}),a.push(be),C.node.position.set(X.x,X.y,X.z),be}function D(C,N,U,E){let G={id:C,body:N,node:U,pause:E,role:"patrol",state:"patrol",cooldown:7+a.length,path:[]};return a.push(G),G}function w(C){A(C),C.source="ambient",C.state="idle",C.until=f+2;let N=c.filter(U=>!U.owner&&(C.role==="courier"?U.place===(C.carrying?"meeting-charge":"parcel-sorter"):U.role===C.role||U.place==="meeting-charge"));for(let U=0;U<N.length;U++){let E=N[(C.trip+U)%N.length],G=u.route(C.body,E);if(G.length){E.owner=C.id,C.slot=E,C.path=G,C.goal=E,C.trip++,C.state=C.carrying?"carry":"walk";return}}for(let U=0;U<8;U++){let E=u.points[(C.trip++*13+C.index)%u.points.length],G=u.route(C.body,E);if(G.length){C.path=G,C.state="walk";return}}}function R(C){for(let N of[C.a,C.b])A(N),N.meeting=null,N.cooldown=f+12+(N.index||0),N.pause?.(!1),N.state=N.pause?"patrol":"idle",N.until=f+1}function L(){h.forEach(R),h.length=0,d.clear(),g=null,i?.(null);for(let C of a)C.guide=null,C.pause?.(!1,C.body),C.trafficYield=!1,C.yieldTram=null,C.pause||(A(C),C.state="idle",C.until=f+1)}function V(C,N){if(p)return;N&&(f+=C);for(let E of a){let G=E.pause||E.index<x();if(G&&!E.body.enabled){if(!e.relocate(E.body,E.body)){E.node.visible=!1;continue}E.body.enabled=!0,E.node.position.set(E.body.x,E.body.y,E.body.z)}if(!G&&E.body.enabled&&(A(E),E.body.enabled=!1,E.meeting&&R(E.meeting)),E.node.visible=!!G,!G||!N)continue;let Y=e.neighbors(E.body,22).sort((X,ie)=>Math.hypot(X.x-E.body.x,X.z-E.body.z)-Math.hypot(ie.x-E.body.x,ie.z-E.body.z)).find(X=>{if(!X.id.startsWith("tram-"))return!1;let ie=E.body.x-X.x,ce=E.body.z-X.z,be=Math.cos(X.heading),Le=Math.sin(X.heading);return Math.abs(ie*be-ce*Le)<E.body.reach+(E.trafficYield?4:2.3)&&ie*Le+ce*be>-8-E.body.reach&&ie*Le+ce*be<14});if(!Y&&E.yieldTram){let X=e.body(E.yieldTram),ie=X?E.body.x-X.x:0,ce=X?E.body.z-X.z:0;if(X?.enabled&&Math.hypot(ie,ce)<26&&ie*Math.sin(X.heading)+ce*Math.cos(X.heading)>-X.reach-E.body.reach-1){E.pause?.(!0),E.play?.("idle");continue}E.yieldTram=null}if(Y){if(E.trafficYield=!0,E.yieldTram=Y.id,E.meeting){let ae=E.meeting;R(ae),h.splice(h.indexOf(ae),1)}E.pause?.(!0);let X=Math.cos(Y.heading),ie=Math.sin(Y.heading),ce=Math.sign((E.body.x-Y.x)*X-(E.body.z-Y.z)*ie)||1,be=(E.body.x-Y.x)*X-(E.body.z-Y.z)*ie,Le=Math.sign((E.body.x-Y.x)*ie+(E.body.z-Y.z)*X)||1,Re=[[X*ce,-ie*ce,E.body.reach+4.5-Math.abs(be)],[-X*ce,ie*ce,E.body.reach+4.5+Math.abs(be)],[ie*Le,X*Le,1.2]],oe=Array.from({length:16},(ae,Se)=>[Math.cos(Se*Math.PI/8),Math.sin(Se*Math.PI/8),2]),ge=([ae,Se])=>ae*(E.body.x-Y.x)+Se*(E.body.z-Y.z);Re.push(...oe.filter(ae=>ge(ae)>.2).sort((ae,Se)=>ge(Se)-ge(ae)));for(let[ae,Se,he]of Re){let ue={x:E.body.x+ae*he,y:E.body.y,z:E.body.z+Se*he,heading:E.body.heading};if(!e.clear(E.body,E.body,ue))continue;let H={...ue,x:E.body.x+ae*C*1.8,z:E.body.z+Se*C*1.8};H.y=E.pause?E.body.y:n(H.x,H.z),e.propose(E.body,H,(Z,W)=>{E.node.position.set(W.x,W.y,W.z),E.play?.("walk")}),E.state="yield",E.cooldown=f+5;break}continue}if(E.trafficYield&&(E.trafficYield=!1,E.pause?.(!1,E.body),E.state=E.pause?"patrol":E.path.length?"walk":"idle"),!E.meeting){if(E.pause){E.state==="greet"&&f>E.until&&(E.pause(!1),E.state="patrol");continue}if(E.guide&&Math.hypot(t.position.x-E.body.x,t.position.z-E.body.z)>9){E.play("idle");continue}if(E.path.length){let X=E.path[0],ie=X.x-E.body.x,ce=X.z-E.body.z,be=Math.hypot(ie,ce),Le=E.guide?1.9:1.65;if(be<.18){E.path.shift();continue}let Re=Math.min(be,Le*C),oe={x:ie/be,z:ce/be},ge=E.body.x+oe.x*Re,ae=E.body.z+oe.z*Re,Se=Iu(E.body.heading,Math.atan2(ie,ce),C),he={x:E.body.x+oe.x*1.8,y:n(ge,ae),z:E.body.z+oe.z*1.8,heading:Se};if(!e.clear(E.body,E.body,he)){let H=!1;for(let[Z,W]of[[oe.z,-oe.x],[-oe.z,oe.x],[-oe.x,-oe.z]]){let J={x:E.body.x+Z*1.3,y:E.body.y,z:E.body.z+W*1.3,heading:E.body.heading};if(e.clear(E.body,E.body,J)){ge=E.body.x+Z*Re,ae=E.body.z+W*Re,Se=Iu(E.body.heading,Math.atan2(Z,W),C),H=!0;break}}H||(ge=E.body.x,ae=E.body.z)}e.clear(E.body,E.body,{x:ge,y:n(ge,ae),z:ae,heading:Se},!1)||(Se=E.body.heading);let ue=Math.hypot(ge-E.body.x,ae-E.body.z)>.001;e.propose(E.body,{x:ge,y:n(ge,ae),z:ae,heading:Se},(H,Z)=>{E.node.position.set(Z.x,Z.y,Z.z),E.node.rotation.y=Z.heading,!H||!ue?(E.wait=(E.wait||0)+C,E.play("idle"),E.state="yield"):(E.wait=0,E.state=E.guide?"guide":E.carrying?"carry":"walk",E.play(E.carrying?"carry":"walk")),E.wait>2.5&&(E.wait=0,A(E),E.state=E.guide?"guide":"idle",E.until=f+.5,E.guide&&(E.path=u.route(E.body,E.guide)))})}else if(E.state==="walk"||E.state==="carry"||E.state==="guide"||E.state==="yield"){if(E.guide&&Math.hypot(E.body.x-E.guide.x,E.body.z-E.guide.z)>.5){E.play("idle"),f>E.until&&(E.path=u.route(E.body,E.guide),E.until=f+2.5);continue}E.guide=null,E.state=E.slot?.place==="meeting-charge"?"charge":E.slot?"work":"idle",E.until=f+5+E.index%4,E.play(E.state==="work"?"work":"idle"),E.slot&&r?.(E.slot.place,5),b++}else f>E.until&&(E.role==="courier"&&E.slot&&(E.state==="work"||E.state==="charge")&&(E.carrying=E.slot.place==="parcel-sorter",E.parcels.forEach(X=>{X.visible=E.carrying})),w(E))}}if(!N){d.update(0,!1);return}for(let E of a){if(!E.body.enabled||E.trafficYield||E.meeting||E.guide||E.state==="greet"||f<E.cooldown)continue;let G=a.find(X=>X!==E&&X.body.enabled&&!X.trafficYield&&X.state!=="greet"&&!X.meeting&&!X.guide&&f>=X.cooldown&&Math.abs(E.body.y-X.body.y)<2&&Math.hypot(E.body.x-X.body.x,E.body.z-X.body.z)>2.8&&Math.hypot(E.body.x-X.body.x,E.body.z-X.body.z)<6);if(!G)continue;let Y={a:E,b:G,at:f,sent:!1,replied:!1};h.push(Y);for(let X of[E,G])X.meeting=Y,X.state="exchange",X.pause?.(!0),X.play?.("greet")}for(let E=h.length-1;E>=0;E--){let G=h[E],Y=f-G.at;if(Y>4.8||!G.a.body.enabled||!G.b.body.enabled){R(G),h.splice(E,1);continue}for(let[X,ie]of[[G.a,G.b],[G.b,G.a]])e.propose(X.body,{...X.body,heading:Iu(X.body.heading,Math.atan2(ie.body.x-X.body.x,ie.body.z-X.body.z),C)},(ce,be)=>{X.node.rotation.y=be.heading});Y>.6&&!G.sent&&(d.send(G.a.body,G.b.body),G.sent=!0),Y>2.4&&!G.replied&&(d.send(G.b.body,G.a.body),G.replied=!0)}d.update(C,!0);let U=a.find(E=>E.id===g);U&&U.state!==S&&(S=U.state,i?.({...T(U),refresh:!0}))}return{add:v,patrol:D,update:V,suspend:L,addPlace(C,N){if(!l.has(C.id)){l.set(C.id,C);for(let U of N.navigation.workpoints||[])c.push({id:C.id+":"+U.id,place:C.id,role:C.role,x:C.x+U.position[0],y:n(C.x,C.z),z:C.z+U.position[2],owner:null})}},nearby(){let C=[];for(let N of a)if(N.body.enabled){let U=Math.hypot(t.position.x-N.body.x,t.position.z-N.body.z);U<5&&Math.abs(t.position.y-N.body.y)<8&&C.push({kind:"resident",id:N.id,distance:U,x:N.body.x,z:N.body.z})}for(let N of l.values()){let U=Math.hypot(t.position.x-N.x,t.position.z-N.z);U<7&&C.push({kind:"demonstrate",id:N.id,distance:U,x:N.x,z:N.z})}return C},inspect(C){let N=a.find(U=>U.id===C);return I(N),T(N)},action(C,N){let U=a.find(E=>E.id===g);if(!U)return null;if(U.meeting){let E=U.meeting,G=h.indexOf(E);R(E),G>=0&&h.splice(G,1)}if(C==="greet"&&(A(U),U.guide=null,U.pause?.(!0),U.state="greet",U.until=f+3,U.cooldown=f+5,U.play?.("greet")),C==="guide"&&!U.pause&&N){A(U);for(let E of[0,1.5,3]){for(let G=0;G<(E?8:1)&&!U.path.length;G++){let Y=N.x+Math.cos(G*Math.PI/4)*E,X=N.z+Math.sin(G*Math.PI/4)*E;U.path=u.route(U.body,{x:Y,z:X,y:n(Y,X)})}if(U.path.length)break}U.guide=U.path.length?U.path[U.path.length-1]:null,U.state=U.guide?"guide":"idle"}return C==="cancel"&&(A(U),U.guide=null,U.pause?.(!1),U.state=U.pause?"patrol":"idle",U.until=f+1),I(U),T(U)},demonstrate(C,N=!1){l.has(C)&&r?.(C,N?0:6)},setTier(C){m=C},setReplay(C){y!==C&&(y=C,L())},event(C){if(y||!o()||!["started","succeeded","failed","sanitized","progress"].includes(C.state)||!Number.isFinite(C.at)||Date.now()-C.at>2500||C.at>Date.now())return;let N=ps.find(U=>U.district===(C.to==="agent"?C.from:C.to));N&&l.has(N.id)&&r?.(N.id,2,C.state)},stats:()=>({completed:b,meetings:h.length,reservations:c.filter(C=>C.owner).length,signals:d.stats(),residents:a.filter(C=>C.body.enabled).map(C=>({...T(C),x:C.body.x,z:C.body.z})),places:[...l.keys()]}),dispose(){p=!0,L(),a.forEach(C=>e.remove(C.id)),a.length=0,l.clear(),c.length=0,d.dispose()}}}function Sp(s){let e=new ct;e.name="living-machinery",s.add(e);let t=new Set,n=new Set,i=new Map,r=0,o="high",a=S=>(t.add(S),S),l=S=>(n.add(S),S),c=a(new Nn(1,.025,6,64)),h=a(new vr(1,48)),u=a(new pt),d=new Float32Array(288),f=new Float32Array(96);for(let S=0;S<96;S++)d[S*3]=Math.cos(S*2.399)*Math.sqrt(S/96),d[S*3+2]=Math.sin(S*2.399)*Math.sqrt(S/96),f[S]=S/96;u.setAttribute("position",new xt(d,3)),u.setAttribute("phase",new xt(f,1));for(let S of ps){let x=new ct;x.name=S.id,x.position.set(S.x,.12,S.z),x.visible=!1,e.add(x);let A=l(new bt({color:7529695,transparent:!0,opacity:.15,depthWrite:!1,blending:Xt,toneMapped:!1})),T=[];for(let D=0;D<3;D++){let w=new Ke(c,l(A.clone()));w.rotation.x=-Math.PI/2,x.add(w),T.push(w)}let I=new Ke(h,l(A.clone()));I.rotation.x=-Math.PI/2,I.scale.setScalar(2.5),I.position.y=.2,x.add(I);let v=new _n(u,l(new mt({transparent:!0,depthWrite:!1,blending:Xt,uniforms:{time:{value:0},strength:{value:0},color:{value:new xe(10414816)}},vertexShader:"attribute float phase;uniform float time;uniform float strength;varying float a;void main(){float t=fract(phase+time*.31);vec3 p=position*vec3(2.,1.,2.);p.y=t*3.;p.xz*=1.-t*.7;vec4 v=modelViewMatrix*vec4(p,1.);a=sin(t*3.14159)*strength;gl_PointSize=clamp(65./max(1.,-v.z),1.,5.);gl_Position=projectionMatrix*v;}",fragmentShader:"uniform vec3 color;varying float a;void main(){float d=length(gl_PointCoord-.5);gl_FragColor=vec4(color,a*(1.-smoothstep(.05,.5,d)));}"})));x.add(v),i.set(S.id,{group:x,waves:T,base:I,motes:v,district:S.district,started:0,until:0,source:"ambient",model:null})}let g=new Ke(a(new ln(3.9,2.3)),l(new mt({transparent:!0,depthWrite:!1,side:Nt,blending:Xt,uniforms:{time:{value:0},strength:{value:0}},vertexShader:"varying vec2 q;void main(){q=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 q;uniform float time;uniform float strength;void main(){float line=pow(max(0.,1.-abs(q.y-fract(time*.4))*14.),3.);float grid=step(.94,fract(q.x*24.))*step(.9,fract(q.y*18.));gl_FragColor=vec4(.18,.9,.8,(line*.36+grid*.1)*sin(q.x*3.14159)*strength);}"})));g.position.set(0,1.4,.8),i.get("repair-bay").group.add(g);let y=i.get("kinetic-fountain"),m=l(new mt({transparent:!0,depthWrite:!1,side:Nt,uniforms:{time:{value:0}},vertexShader:"varying vec2 vUv;void main(){vUv=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:"varying vec2 vUv;uniform float time;void main(){float pulse=.5+.5*sin(vUv.x*95.-time*9.);float edge=.35+.65*pow(max(0.,sin(vUv.y*3.14159)),2.);gl_FragColor=vec4(mix(vec3(.09,.35,.42),vec3(.6,.95,1.),pow(max(0.,pulse),5.)),edge*.56);}"}));for(let S=0;S<8;S++){let x=S*Math.PI/4,A=new B(Math.cos(x)*2.3,.5,Math.sin(x)*2.3),T=new B(Math.cos(x+.5)*.7,.55,Math.sin(x+.5)*.7),I=new ti(A,new B(Math.cos(x)*1.5,3.7,Math.sin(x)*1.5),T);y.group.add(new Ke(a(new po(I,24,.045,5,!1)),m))}let p=l(new mt({transparent:!0,depthWrite:!1,uniforms:{time:{value:0}},vertexShader:"varying vec2 q;uniform float time;void main(){q=position.xy;vec3 p=position;p.z+=sin(length(q)*18.-time*4.)*.012;gl_Position=projectionMatrix*modelViewMatrix*vec4(p,1.);}",fragmentShader:"varying vec2 q;uniform float time;void main(){float r=length(q);float a=.5+.5*sin(r*16.-time*3.+sin(q.x*3.+q.y*2.+time*.7)*.35);float rim=1.-smoothstep(2.35,2.58,r);gl_FragColor=vec4(mix(vec3(.03,.13,.17),vec3(.22,.55,.58),pow(max(0.,a),9.)*.3),.78*rim);}"})),b=new Ke(a(new vr(2.58,64)),p);return b.rotation.x=-Math.PI/2,b.position.y=.3,y.group.add(b),{attach(S,x){let A=i.get(S);A&&(A.model=x,A.group.visible=!0)},demonstrate(S,x=6,A=null){let T=i.get(S);T&&(T.until<=r&&(T.started=r),T.until=r+x,T.source=A?"live":"ambient",T.status=A,T.model?.play("operate"))},clear(){i.forEach(S=>{S.until=0,S.source="ambient",S.status=null})},setTier(S){o=S},update(S,x){x&&(r+=S),m.uniforms.time.value=p.uniforms.time.value=g.material.uniforms.time.value=r;for(let[A,T]of i){let I=T.until>r,v=A==="kinetic-fountain",D=I?wt.smoothstep(r-T.started,0,.35)*wt.smoothstep(T.until-r,0,.65):0,w=v?Math.max(.25,D):D;I||(T.source="ambient",T.status=null);let R=T.status==="failed"?16737881:T.source==="live"?{infra:7978495,integrations:7728086,missions:10268415,graph:16765844,operations:16758915}[T.district]:7529695;if(T.waves.forEach((L,V)=>{L.visible=I;let C=(r*.6+V/3)%1;L.position.y=.4+C*2,L.scale.setScalar(.4+C*2.6),L.material.opacity=(1-C)*.22*D,L.material.color.setHex(R)}),A==="repair-bay"&&(g.visible=I,g.material.uniforms.strength.value=D),A==="relay-mast"&&I){let L=T.model?.node.getObjectByName("antenna");L&&(L.rotation.y=wt.damp(L.rotation.y,Math.atan2(-T.group.position.x,-12-T.group.position.z),3,x?S:0))}T.base.material.opacity=.08*D,T.motes.visible=o!=="low"&&w>0,T.motes.material.uniforms.time.value=r,T.motes.material.uniforms.strength.value=w,T.model?.mixer&&(T.model.mixer.timeScale=x&&(I||v)?.65:0)}},stats:()=>({places:[...i].filter(([,S])=>S.model).map(([S,x])=>({id:S,operating:x.until>r,source:x.source})),time:r}),dispose(){e.removeFromParent(),t.forEach(S=>S.dispose()),n.forEach(S=>S.dispose()),i.clear()}}}function wp(s,e){let t=new ct;t.name="world-2",s.add(t);let n=new Map,i=new Set,r=new Set,o=[],a=[],l=[],c=new Map,h=new Map,u=new Map,d=new Map,f=new Set,g=[],y=new Set,m=[],p=new Map,b=new AbortController,S=[],x=0,A=!1,T=!1,I=e.tier||"high",v=null,D=0,w=0,R=null,L=null,V=0,C=new Set,N=null,U=0,E=new B,G=new B,Y=new B,X=e.camera,ie=e.traffic||_c(),ce=Sp(s),be=new Map;function Le(M,_){let O=fs(M,_);for(let k of ps)for(let P of v?.assets.find(z=>z.id===k.id)?.navigation.surfaces||[]){let[z,$,j,ne]=P.rect;M>=k.x+z&&M<=k.x+j&&_>=k.z+$&&_<=k.z+ne&&(O=Math.max(O,fs(k.x,k.z)+P.height))}return O}let Re=bp(s,{traffic:ie,camera:X,floor:Le,onSelect:e.onSociety,onDemonstrate:ce.demonstrate,active:e.active}),oe=0,ge=0;try{C=new Set(JSON.parse(localStorage.getItem("aurago.desktop.sysworld.discoveries")||"[]").filter(M=>Zn.some(_=>_.id===M)))}catch{}let ae=Su(Ru),Se=ae.getLength(),he=Zn.map(M=>{let _=1/0,O=0;for(let k=0;k<3e3;k++){ae.getPointAt(k/3e3,E);let P=Math.hypot(E.x-M.x,E.z-M.z);P<_&&(_=P,O=k/3e3)}return{...M,u:O}}),ue=(M,_)=>e.onSound?.(M,_?.x||0,_?.y||0,_?.z||0,!!N),H=new yn({color:11834208,metalness:.8,roughness:.35});r.add(H);for(let M of[-1,1]){let _=[],O=[];for(let z=0;z<=512;z++){ae.getPointAt(z/512,E),ae.getTangentAt(z/512,G);for(let $ of[-.045,.045]){let j=M*.85+$;_.push(E.x-G.z*j,.21,E.z+G.x*j)}if(z<512){let $=z*2;O.push($,$+1,$+2,$+1,$+3,$+2)}}let P=new pt;P.setAttribute("position",new nt(_,3)),P.setIndex(O),P.computeVertexNormals(),i.add(P),t.add(new Ke(P,H))}async function Z(M,_=I==="low"?2:I==="medium"?1:0){let O=M+":"+_;return n.has(O)||n.set(O,(async()=>{let k=v.assets.find(de=>de.id===M),P=k?.lods.find(de=>de.level===_);if(!P||!/^[\w-]+\.lod[0-2]\.glb$/.test(P.file))throw Error("Invalid world asset");let z=await fetch(e.assetURL(P.file),{signal:b.signal});if(!z.ok)throw Error("World asset unavailable");let $=await z.arrayBuffer();if(T)throw Error("Disposed");D+=$.byteLength;let j=await new ds().parseAsync($,""),ne=j.animations.length>0||/^(tram|service-cart|robot-|door|lift)/.test(M);if(j.scene.traverse(de=>{if(de.isMesh){i.add(de.geometry);for(let Ce of Array.isArray(de.material)?de.material:[de.material])r.add(Ce);de.castShadow=!0,de.receiveShadow=!0,e.surfaces?.prepare(de,ne)}}),T)throw i.forEach(de=>de.dispose()),r.forEach(de=>de.dispose()),Error("Disposed");return f.add(M),j})()),n.get(O)}async function W(M,_,O,k=0,P=0,z=t,$=[1,1,1],j){let ne=await Z(M,j);if(T)return null;let de=ne.scene.clone(!0);de.position.set(_,k,O),de.rotation.y=P,de.scale.set(...$),z.add(de),de.traverse(Ee=>{Ee.isMesh&&(Ee.userData.worldAsset=M,Ee.userData.worldPart=Ee.name,g.push(Ee))});let Ce=v.assets.find(Ee=>Ee.id===M),Me="furnishing:"+oe++;for(let[Ee,Ve]of(Ce.navigation.colliders||[]).entries())ie.solid(Me+":"+Ee,{owner:Me,x:_,z:O,heading:P,min:[Ve[0]*$[0],k+Ve[1]*$[1],Ve[2]*$[2]],max:[Ve[3]*$[0],k+Ve[4]*$[1],Ve[5]*$[2]]});let ve=ne.animations.length?new To(de):null;ve&&o.push(ve);let Be=null;return{node:de,mixer:ve,owner:Me,bounds:Ce.motion_bounds||Ce.lods[0].bounds,clips:ne.animations,play(Ee,Ve=!1){if(!ve)return;let K=ne.animations.find(fe=>fe.name===Ee);if(!K)return;let ye=ve.clipAction(K);return Be===ye||(ye.reset(),Ve&&(ye.setLoop(Jl,1),ye.clampWhenFinished=!0),ye.play(),Be&&ye.crossFadeFrom(Be,.25,!1),Be=ye),ye}}}function J(M){M.updateMatrixWorld(!0);let _=new Map;M.traverse(k=>{if(!k.isMesh)return;let P=k.geometry.uuid+":"+(Array.isArray(k.material)?k.material.map(z=>z.uuid).join(","):k.material.uuid);_.has(P)||_.set(P,{n:k,matrices:[]}),_.get(P).matrices.push(k.matrixWorld.clone())});let O=new ct;t.add(O);for(let{n:k,matrices:P}of _.values()){let z=new vn(k.geometry,k.material,P.length);P.forEach(($,j)=>z.setMatrixAt(j,$)),z.castShadow=!0,z.receiveShadow=!0,z.userData={...k.userData},g.push(z),O.add(z)}return M.traverse(k=>{let P=g.indexOf(k);P>=0&&g.splice(P,1)}),M.removeFromParent(),u.set(O.uuid,O),O}function te(M){if(d.has(M.id))return d.get(M.id);let _=(async()=>{let O=new ct;t.add(O);let k=[];for(let P of xp())k.push(W("floor",M.x+P.x,M.z+P.z,ut.room,0,O,[P.sx,1,P.sz]));for(let P=-4;P<=4;P+=4)for(let z=-4;z<=4;z+=4)k.push(W("ceiling",M.x+P,M.z+z,8.15+ut.room,0,O));for(let P=-4;P<=4;P+=4)k.push(W("wall",M.x+P,M.z+6,ut.room,0,O));for(let P=-4;P<=4;P+=4)for(let z of[-1,1])for(let $ of[0,4])k.push(W("window",M.x+z*6,M.z+P,ut.room+$,Math.PI/2,O));for(let P=-4;P<=4;P+=4)for(let z of[-1,1])k.push(W("window",M.x+P,M.z+z*6,ut.gallery,0,O));for(let P of[-4,4])k.push(W("wall",M.x+P,M.doorZ,ut.room,0,O));for(let[P,z]of[[-4,1],[.2,1.1]])k.push(W("floor",M.x+P,M.z+3,ut.gallery,0,O,[z,1,1.5])),k.push(W("railing",M.x+P,M.z,ut.gallery,0,O,[z,1,1]));for(let P of[.7,5.3])k.push(W("railing",M.x+2.4,M.z+P,ut.gallery,Math.PI/2,O,[.35,1,1]));for(let P of[-5.6,1.9])k.push(W("wall",M.x+P,M.z+.15,ut.room,0,O,[.05,1,.6]));if(await Promise.all(k),!T)return J(O)})();return d.set(M.id,_),_}async function pe(M){if(!(c.has(M.id)||T)){c.set(M.id,{ready:!1,lift:null,liftValue:0,liftTarget:0}),U++;try{await te(M);let _=new ct;t.add(_);let O=[];for(let P of[-3,0])O.push(W(M.id==="missions"?"cargo":M.id==="memory"?"archive-shelf":"console",M.x+P,M.z+3,ut.room,Math.PI,_));if(O.push(W("bench",M.x-4,M.z-1,ut.room,Math.PI/2,_)),O.push(W("bench",M.x-4,M.z+4.8,ut.gallery,0,_)),O.push(W("console",M.x,M.z+4.8,ut.gallery,Math.PI,_)),await Promise.all(O),T)return;J(_);let k=c.get(M.id);k.hologram=await W("hologram",M.x-1,M.z,ut.room),k.hologram?.play("operate"),M.id==="memory"&&je(),k.lift=await W("lift",M.liftX,M.liftZ,ut.room),k.liftAction=k.lift?.play("operate",!0),k.liftAction&&(k.liftAction.paused=!0),k.ready=!0}catch(_){T||e.onError?.(_)}finally{U--}}}async function we(){try{let M=await fetch(e.assetURL("manifest.json"),{signal:b.signal});if(!M.ok)throw Error("World manifest unavailable");v=await M.json();let _=new ct;t.add(_);let O=[];for(let P of Zn)O.push(W("station",P.platformX,P.platformZ,ut.pavement,P.angle,_));for(let P of pn){O.push(te(P));let z=await W("door",P.x,P.doorZ,ut.room,Math.PI);if(T)return;let $=z.play("open",!0);$.paused=!0,h.set(P.id,{...z,action:$,value:0,open:!1})}for(let P=-76;P<=76;P+=8)O.push(W("quay",P,77,ut.quay,0,_));for(let[P,z]of[[-79,68],[-79,-67],[55,2],[8,40],[55,-24]])O.push(W("garden",P,z,0,0,_)),O.push(W("bench",P+3,z,0,Math.PI/2,_));for(let[P,z]of[[-30,23],[30,23],[-28,72]])O.push(W("arcade",P,z,0,0,_));O.push(W("bridge",-78.5,40,2,0,_,[1.75,1,1]),W("ramp",-80,49,0,Math.PI,_),W("stairs",-76.8,49,0,Math.PI,_),W("ramp",-78.5,31,0,0,_)),O.push(W("pad",wi.x,wi.z,0,0,_));for(let[P,z]of[[-60,-56],[-60,-46],[51,49]])O.push(W("charger",P,z,0,0,_));if(await Promise.all(O),T)return;J(_);for(let[P,z]of[[-57,-61],[-57,-52]])(await W("cooler",P,z))?.play("operate");for(let P of ps){let z=await W(P.id,P.x,P.z,fs(P.x,P.z),0,t,[1,1,1],2);if(T)return;z&&(z.level=2,be.set(P.id,z),ce.attach(P.id,z),Re.addPlace(P,v.assets.find($=>$.id===P.id)),z.play("operate"))}let k=await W("service-cart",55,49,ut.ground,Math.PI/2);k&&k.play("open",!0);for(let P=0;P<19;P++){let z=["courier","technician","archivist"][P%3],$=await W("robot-"+z,0,0);if(!$)return;$.node.scale.setScalar(1.15),a.push($),Re.add($,P,$.bounds),$.play("idle")}for(let P=0;P<2;P++){let z=await W("tram",0,0);if(!z)return;let $=z.play("open",!0);$.paused=!0;let j=he[P?3:0].u;ae.getPointAt(j,E),ae.getTangentAt(j,G);let ne=ie.register("tram-"+P,Hi(z.bounds),{x:E.x,y:ut.road+.03,z:E.z,heading:Math.atan2(G.x,G.z)},3);l.push({...z,body:ne,u:j,speed:0,dwell:5,stop:P?3:0,door:$})}for(let P=0;P<3;P++){let z=await W("cargo",43,70);z&&(ie.removeOwner(z.owner),z.node.visible=!1,m.push({...z,body:null,elapsed:9,direction:1}))}e.onReady?.()}catch(M){T||e.onError?.(M)}}we();async function Pe(M,_,O){if(!(_.level===O||_.requested===O)){_.requested=O;try{let k=await Z(M,O);if(T||_.requested!==O)return;let P=new Map;k.scene.traverse(z=>{z.isMesh&&P.set(z.name,z)}),_.node.traverse(z=>{let $=P.get(z.name);z.isMesh&&$&&(z.geometry=$.geometry,z.material=$.material)}),_.level=O,_.requested=null,e.onReady?.()}catch(k){_.requested=null,T||e.onError?.(k)}}}function je(){let M=c.get("memory");if(!M?.hologram||!M.text&&!S.length)return;if(!M.text){let z=document.createElement("canvas");z.width=1024,z.height=512;let $=new Ps(z);$.colorSpace=zt,y.add($);let j=new ln(4.8,2.4),ne=new bt({map:$,transparent:!0,depthWrite:!1,side:Nt,toneMapped:!1});i.add(j),r.add(ne);let de=new Ke(j,ne),Ce=pn.find(Me=>Me.id==="memory");de.position.set(Ce.x-1,3.6+ut.room,Ce.z+.3),de.rotation.y=Math.PI,t.add(de),M.text={canvas:z,texture:$,mesh:de}}let{canvas:_,texture:O,mesh:k}=M.text,P=_.getContext("2d");P.clearRect(0,0,1024,512),k.visible=S.length>0,P.fillStyle="rgba(5,24,32,.87)",P.fillRect(0,0,1024,512),P.fillStyle="#b9f4ef",P.font="28px sans-serif",S.slice(0,4).forEach((z,$)=>{let j=Array.from(z).slice(0,96);P.fillText(j.slice(0,48).join(""),30,55+$*118),P.fillText(j.slice(48).join(""),30,94+$*118)}),O.needsUpdate=!0}function Xe(){if(R){if(R.kind==="tram"&&l[R.index].dwell<=1){R.exitRequested=!0;return}Je();return}if(L){if(L.kind==="resident"){Re.inspect(L.id);return}if(L.kind==="demonstrate"){Re.demonstrate(L.id,e.reduced()),e.onSociety?.({id:L.id,role:"installation",state:e.reduced()?"idle":"work",source:"ambient"});return}if(L.kind==="door"){let M=h.get(L.id);if(M){M.open=!M.open,ue("door",M.node.position);let _=pn.find(O=>O.id===L.id);pe(_)}}if(L.kind==="discover"){C.add(L.id);try{localStorage.setItem("aurago.desktop.sysworld.discoveries",JSON.stringify([...C]))}catch{}e.onDiscover?.(L.id),ue("discover",X.position)}if(L.kind==="tram"&&!e.reduced()&&(R={kind:"waiting",station:L.id},e.onRide?.("waiting")),L.kind==="terminal"&&e.onTerminal?.(L.id),L.kind==="drone"&&!e.reduced()&&(R={kind:"drone",elapsed:0,origin:X.position.clone()},e.onRide?.("drone"),ue("tram",X.position)),L.kind==="lift"){let M=c.get(L.id);if(M){let _=X.position.y>5?4:0;if(Math.abs(M.liftValue-_)>.02){M.liftTarget=_;return}M.liftTarget=_?0:4,R={kind:"lift",id:L.id},e.onRide?.("lift"),ue("lift",X.position)}}}}function Je(){if(R){if(R.kind==="lift"){let M=c.get(R.id),_=pn.find(O=>O.id===R.id);M.liftTarget=M.liftValue<2?0:4,X.position.set(_.x+1.5,2.4+ut.room+M.liftTarget,_.liftZ)}if(R.kind==="tram"){let M=Zn.find(_=>_.id===R.station)||Zn[0];X.position.set(M.platformX,2.4+ut.pavement+.3,M.platformZ)}R.kind==="drone"&&X.position.copy(R.origin),R=null,e.onRide?.(null)}}function q(){if(R)return[{kind:"exit",id:R.kind,distance:0}];X.getWorldDirection(E);let M=Re.nearby().filter(O=>(O.x-X.position.x)*E.x+(O.z-X.position.z)*E.z>O.distance*.3),_=(O,k,P,z,$)=>{let j=Math.hypot(X.position.x-P,X.position.z-z);j<$&&M.push({kind:O,id:k,distance:j})};for(let O of pn)X.position.y<5&&_("door",O.id,O.x,O.doorZ,3),c.get(O.id)?.ready&&(_("lift",O.id,O.liftX,O.liftZ,2.3),X.position.y<5&&_("terminal",O.id,O.x-2,O.z+3,2.5));if(X.position.y<5)for(let O of Zn)_("tram",O.id,O.platformX,O.platformZ,4),C.has(O.id)||_("discover",O.id,O.platformX+(O.angle===0?5:0),O.platformZ+(O.angle===0?0:5),3);return _("drone","drone",wi.x,wi.z,5),M.sort((O,k)=>O.distance-k.distance)}function it(M,_,O){if(T||!v)return;let k=_?Math.min(M,.05):0;if(w+=k,N=Hs(X.position.x,X.position.z)?.id||null,ge+=M,ge>.5){ge=0;for(let[P,z]of be){let $=X.position.distanceTo(z.node.position),j=I==="low"||$>55?2:I==="medium"||$>32?1:0;Pe(P,z,j)}}e.traffic||ie.begin();for(let P of pn)Math.hypot(X.position.x-P.x,X.position.z-P.z)<32&&pe(P);o.forEach(P=>{P.getRoot().visible&&P.update(k)});for(let P of h.values()){let z=wt.damp(P.value,P.open?1:0,5,Math.min(M,.1));Math.abs(z-(P.open?1:0))<.001&&(z=P.open?1:0);let $=j=>[-1,1].map(ne=>({owner:P.owner,x:P.node.position.x,z:P.node.position.z,min:[ne*.8+ne*1.6*j-.8,ut.room,-.13],max:[ne*.8+ne*1.6*j+.8,ut.room+3.6,.13]}));(P.open||$(z).every(j=>ie.solidClear(j)))&&(P.value=z),$(P.value).forEach((j,ne)=>ie.solid(P.owner+":panel:"+ne,j)),P.action.time=P.value*P.action.getClip().duration,P.mixer.update(0)}for(let[P,z]of c)if(z.liftAction&&(z.liftValue=wt.damp(z.liftValue,z.liftTarget,1.8,Math.min(M,.1)),Math.abs(z.liftValue-z.liftTarget)<.02&&(z.liftValue=z.liftTarget),z.liftAction.time=z.liftValue/4*z.liftAction.getClip().duration,z.lift.mixer.update(0),R?.kind==="lift"&&R.id===P)){let $=pn.find(j=>j.id===P);X.position.set($.liftX,2.4+ut.room+z.liftValue,$.liftZ),z.liftValue===z.liftTarget&&(R=null,e.onRide?.(null))}if(Re.update(k,_),ce.update(k,_),l.forEach((P,z)=>{if(P.node.visible=!!P.body&&z<(I==="low"?1:2),P.body&&(P.node.visible&&!P.body.enabled&&!ie.clear({...P.body,enabled:!0},P.body,P.body)&&(P.node.visible=!1),P.body.enabled=P.node.visible),!P.node.visible)return;let $={u:P.u,dwell:P.dwell,stop:P.stop},j=!1;if(P.dwell>0)P.dwell-=k,P.speed=0;else{let ne=Ee=>(Ee.u-P.u+1)%1*Se,de=he.reduce((Ee,Ve)=>ne(Ve)>1e-5&&ne(Ve)<ne(Ee)?Ve:Ee,he.find(Ee=>ne(Ee)>1e-5)||he[0]),Ce=Math.min(11,Math.sqrt(ne(de)*8));for(let Ee=1;Ee<=3;Ee++){let Ve=(P.u+(1.2+P.speed*.7)*Ee/3/Se)%1;ae.getPointAt(Ve,E),ae.getTangentAt(Ve,G);let K={x:E.x,y:ut.road+.03,z:E.z,heading:Math.atan2(G.x,G.z)};if(!ie.clear(P.body,K,K)){Ce=0;break}}P.speed+=wt.clamp(Ce-P.speed,-k*8,k*3.5);let Me=k*P.speed/Se,ve=P.u;P.u=(P.u+Me)%1;let Be=he.findIndex(Ee=>(Ee.u-ve+1)%1>0&&(Ee.u-ve+1)%1<=Me+1e-5);Be>=0&&(P.stop=Be,P.u=he[Be].u,P.dwell=6,P.speed=0,j=!0)}if(ae.getPointAt(P.u,P.node.position),P.node.position.y=ut.road+.03,ae.getTangentAt(P.u,G),P.node.rotation.y=Math.atan2(G.x,G.z),P.body&&(P.body.enabled=P.node.visible,ie.propose(P.body,{...P.node.position,heading:P.node.rotation.y},(ne,de)=>{ne||(Object.assign(P,$),P.speed=0),P.node.position.set(de.x,de.y,de.z),P.node.rotation.y=de.heading,ne&&j&&ue("tram",P.node.position)})),P.door.time=(P.dwell>1?1:0)*P.door.getClip().duration,P.mixer.update(0),R?.kind==="waiting"&&he[P.stop].id===R.station&&P.dwell>1&&(R={kind:"tram",index:z,station:R.station,offset:0},e.onRide?.("tram")),R?.kind==="tram"&&R.index===z){if(P.dwell>1&&(R.station=he[P.stop].id,R.exitRequested)){Je();return}X.position.copy(P.node.position).addScaledVector(G,R.offset),X.position.y+=.9+1.4,X.lookAt(P.node.position.x+G.x*16,X.position.y,P.node.position.z+G.z*16)}}),R?.kind==="drone"){R.elapsed+=Math.min(M,.1);let P=R.elapsed/40*Math.PI*2;X.position.set(Math.cos(P)*100,45+Math.sin(P*2)*12,Math.sin(P)*95-10),X.lookAt(0,22,-12),R.elapsed>=40&&Je()}if(L=O==="street"&&q()[0]||null,O==="street"&&!R){let P=X.position.distanceTo(Y);P<3&&(V+=P),V>1.8&&(ue(N?"step_inside":"step",X.position),V=0)}Y.copy(X.position),e.onEnvironment?.(!!N);for(let[P,z]of m.entries()){if(z.elapsed+=k,z.node.visible=_&&z.elapsed<8,!z.node.visible){z.body&&ie.remove(z.body.id),z.body=null;continue}let $=z.direction>0?z.elapsed/8:1-z.elapsed/8,j={x:40+$*3,y:ut.room,z:70,heading:0};if(z.body||(z.body=ie.register("freight-"+P,Hi(z.bounds),j,0)),!z.body){z.elapsed=9,z.node.visible=!1;continue}ie.propose(z.body,j,(ne,de)=>{ne||(z.elapsed-=k),z.node.position.set(de.x,de.y,de.z)})}e.traffic||ie.solve(k),e.onInteraction?.(L,C.size,N)}async function Ue(M){if(I=M,!v)return;let _=M==="low"?2:M==="medium"?1:0;Re.setTier(M),ce.setTier(M);try{let O=new Map(await Promise.all([...f].filter(k=>!be.has(k)).map(async k=>{let P=await Z(k,_),z=new Map;return P.scene.traverse($=>{$.isMesh&&z.set($.name,$)}),[k,z]})));if(T||I!==M)return;for(let k of g){let P=O.get(k.userData.worldAsset)?.get(k.userData.worldPart);P&&(k.geometry=P.geometry,k.material=P.material)}e.onReady?.()}catch(O){T||e.onError?.(O)}}return{update:it,interact:Xe,endRide:Je,setTier:Ue,society:Re,suspend(){Re.suspend(),ce.clear(),A=!1;for(let M of m)M.elapsed=9,M.node.visible=!1,M.body&&ie.remove(M.body.id),M.body=null},syncRide(){if(R?.kind==="tram"){let M=l[R.index];ae.getTangentAt(M.u,G),X.position.copy(M.node.position).addScaledVector(G,R.offset),X.position.y+=2.3,X.lookAt(M.node.position.x+G.x*16,X.position.y,M.node.position.z+G.z*16)}},socialAction(M,_){let O=Zn.find(k=>k.id===_);return Re.action(e.reduced()&&M==="guide"?"cancel":M,O?{x:O.platformX,z:O.platformZ}:null)},setMemory(M){S=(Array.isArray(M)?M:[]).filter(_=>typeof _=="string").slice(0,8),je()},setWorld(M,_=!1){_?(Re.setReplay(!0),ce.clear()):Re.setReplay(!1);let O=M?.entities?.filter(P=>P.kind==="mission")||[],k=new Set;for(let P of O){k.add(P.id);let z=p.get(P.id);if(!_&&e.active()&&A&&z!=null&&z!==P.state&&["running","completed","failed","cancelled"].includes(P.state)&&Date.now()-P.at<3e4){let $=m.find(j=>j.elapsed>=8);$&&($.elapsed=0,$.direction=P.state==="running"?1:-1,x++)}p.set(P.id,P.state)}for(let P of p.keys())k.has(P)||p.delete(P);if(A=!_,_)for(let P of m)P.elapsed=9,P.node.visible=!1},walkRide(M,_){return R?(R.kind==="tram"&&(R.offset=wt.clamp(R.offset+M*_*3,-2.5,2.5)),!0):!1},move(M,_,O){if(R)return!1;let k=Hs(O.x,O.z);return k&&O.y>5?Math.abs(_-k.z-4.8)<.85&&(Math.abs(M-k.x)<1.1||Math.abs(M-k.x+4)<1.75)?!1:Cu(k,M,_,c.get(k.id)?.liftValue===4):k&&O.y<5&&([-5.6,1.9].some(P=>Math.abs(M-k.x-P)<.25&&Math.abs(_-k.z-.15)<.25)||gp(k,M,_)&&!(M<k.liftX+1.25&&Math.abs(_-k.liftZ)<1.25&&c.get(k.id)?.liftValue===0)||Math.hypot(M-(k.x-1),_-k.z)<1.55||[-3,0].some(P=>Math.abs(M-k.x-P)<1.1&&Math.abs(_-k.z-3)<.8))?!1:vp(M,_,O,P=>h.get(P)?.value>.85&&!!c.get(P)?.ready,e.districts)},floor(M,_){let O=Hs(M,_);return O&&X.position.y>5&&Cu(O,M,_,c.get(O.id)?.liftValue===4)?ut.gallery:Math.max(Le(M,_),_p(M,_,e.districts))},visit(M){Je(),R=null;let _=ps.find(k=>k.id===M);if(_){let k=v?.assets.find(P=>P.id===M)?.navigation.interaction[0]?.position||[0,1.5,4];X.position.set(_.x,2.4+fs(_.x,_.z),_.z+k[2]+1),X.lookAt(_.x,2,_.z);return}if(M==="drone"){X.position.set(wi.x,2.7,wi.z+3),X.lookAt(wi.x,2,wi.z);return}let O=Zn.find(k=>k.id===M);if(O){let k=C.has(M)?0:5,P=O.platformX+(O.angle===0?k:0),z=O.platformZ+(O.angle===0?0:k);X.position.set(P,2.4+fs(P,z),z),X.lookAt(O.platformX,2,O.platformZ===z?O.platformZ+1:O.platformZ)}},destination(M){let _=pn.find(O=>O.id===M);if(_){Je(),R=null;let O=_.doorZ+_.front*4;X.position.set(_.x,2.4+fs(_.x,O),O),X.lookAt(_.x,2.4+fs(_.x,O),_.z)}},isRiding:()=>!!R,rideBody:()=>R?.kind==="tram"?"tram-"+R.index:null,interaction:()=>L,stats:()=>({society:Re.stats(),machinery:ce.stats(),details:[...be].map(([M,_])=>({id:M,level:_.level})),loaded:[...f],bytes:D,freightEvents:x,residents:a.filter(M=>M.node.visible).length,trams:l.filter(M=>M.node.visible).length,rooms:[...c].filter(([,M])=>M.ready).map(([M])=>M),inside:N,ride:R?.kind||null,station:R?.station||null,discovered:[...C],loading:U,interactions:L?{kind:L.kind,id:L.id}:null}),dispose(){T||(T=!0,b.abort(),Re.dispose(),ce.dispose(),e.traffic||ie.dispose(),S=[],p.clear(),t.removeFromParent(),o.forEach(M=>{M.stopAllAction(),M.uncacheRoot(M.getRoot())}),t.traverse(M=>{M.isInstancedMesh&&M.dispose()}),i.forEach(M=>M.dispose()),r.forEach(M=>M.dispose()),y.forEach(M=>M.dispose()),n.clear())}}}var Sy=s=>.5+(s-6)/12*3.62,Ep=new B;function Tp(s,{sun:e,rim:t,hemisphere:n,atmosphere:i,surfaces:r,onThunder:o}){let a="local",l="clear",c="high",h=!1,u=-100,d=!1,f=n.intensity,g=14,y=-1,m=0,p={value:0},b=new pt,S=[],x=317,A=()=>(x=Math.imul(x,1664525)+1013904223>>>0)/4294967296;for(let C=0;C<1200;C++)S.push((A()-.5)*170,A()*60,(A()-.5)*174-7);b.setAttribute("position",new nt(S,3));let T=new mt({transparent:!0,depthWrite:!1,uniforms:{time:p,flash:{value:0}},vertexShader:"uniform float time;varying float vFade;void main(){vec3 p=position;p.y=mod(p.y-time*20.0,60.0);p.x+=sin(time*.4)*2.0;vec4 mv=modelViewMatrix*vec4(p,1.0);gl_Position=projectionMatrix*mv;gl_PointSize=clamp(260.0/max(1.0,-mv.z),2.0,26.0);vFade=smoothstep(0.0,4.0,p.y)*smoothstep(460.0,60.0,-mv.z);}",fragmentShader:"uniform float flash;varying float vFade;void main(){vec2 c=gl_PointCoord-.5;float a=(1.0-smoothstep(0.0,0.07,abs(c.x)))*(1.0-smoothstep(0.2,0.5,abs(c.y)))*.42*vFade;if(a<.004)discard;gl_FragColor=vec4(mix(vec3(.6,.78,.86),vec3(.95,.97,1.0),flash),a*(1.0+flash));}"}),I=new _n(b,T);I.frustumCulled=!1,s.add(I);let v=new xe(10205129),D=new xe(528926),w=new xe(2762296);function R(){let C=Mc(a),N=C.amount,U=Sy(C.hour);e.intensity=.8+N*2.6,e.color.set(C.evening>.1?16757370:13164287),e.position.set(Math.cos(C.hour/24*Math.PI*2)*120,40+N*115,85),Ep.set(Math.cos(U)*120,40-C.evening*18+N*115,Math.sin(U)*120),t.intensity=.7+C.evening*1.6,f=n.intensity=.5+N*.65,s.fog.color.copy(D).lerp(v,N).lerp(w,C.evening*(1-N)*.5),s.fog.density=l==="fog"?.018:l==="rain"?.008:.003-N*8e-4,i.setLighting?.(N,C.evening,Ep),i.setWeather?.(l),r?.setLighting(N,C.evening,l==="clear"?0:1,s.fog.color),r?.setWeather(l),I.visible=l==="rain"&&!h,b.setDrawRange(0,c==="low"?200:1200)}function L(C,N){if(l!=="rain"||h||!N){y>=0&&(y=-1,V(0));return}if(g-=C,y<0&&g<=0&&(y=0,m++,g=9+A()*16,o?.(.6+A()*3.2)),y<0)return;y+=C;let U=y<.09?1:y<.16?.25:y<.24?.75:Math.max(0,1-(y-.24)/.35)*.5;V(U),y>.6&&(y=-1,V(0))}function V(C){n.intensity=f+C*2.4,T.uniforms.flash.value=C,i.setFlash?.(C)}return R(),{set(C={}){["local","day","evening","night"].includes(C.time)&&(a=C.time),["clear","rain","fog"].includes(C.weather)&&(l=C.weather),R()},setTier(C){c=C,R()},setIndoor(C){h!==C&&(h=C,R())},update(C,N,U){if(!d)return I.visible=l==="rain"&&!h&&U,U&&(p.value+=Math.min(C,.1)),L(Math.min(C,.1),U),N-u>30?(u=N,R(),!0):!1},mood(){let C=Mc(a);return{day:C.amount,evening:C.evening,weather:l,indoor:h}},stats:()=>({time:a,weather:l,indoor:h,daylight:Mc(a).amount,strikes:m}),dispose(){d=!0,I.removeFromParent(),b.dispose(),T.dispose()}}}function Ap(s,e,t,n){for(let i of t){let r=e.get(i.asset).lods[0].bounds;(Pu[i.id]||(i.id==="graph"?[[-10.2,-10.2,10.2,10.2]]:[[-3,-3,3,3]])).forEach(([a,l,c,h],u)=>s.solid("district:"+i.id+":"+u,{x:i.x,z:i.z,min:[a,0,l],max:[c,8,h]})),s.solid("district:"+i.id+":upper",{x:i.x,z:i.z,min:[r.min[0],8,r.min[2]],max:r.max})}n.forEach((i,r)=>{if(i.asset==="street-tile"||i.asset==="street-crossing")return;let o=e.get(i.asset)?.lods[0].bounds;if(!o)return;let a=o.min.map((c,h)=>c*i.scale[h]),l=o.max.map((c,h)=>c*i.scale[h]);a[1]+=i.y,l[1]+=i.y,i.asset==="street-lamp"&&(s.solid("city:"+r+":pole",{x:i.x,z:i.z,min:[-.2,i.y,-.2],max:[.2,l[1],.2]}),a[1]=Math.max(a[1],l[1]-1)),s.solid("city:"+r,{x:i.x,z:i.z,heading:i.angle,min:a,max:l})})}var Qo={"city.road":{top:"asphalt",side:"concrete",albedo:.55,rough:.7,bump:.012,wet:1,puddle:[.447,.442]},"city.stone":{top:"pavers",side:"concrete",albedo:.5,rough:.6,bump:.014,wet:1,puddle:[.523,.516]},"city.graphite":{top:"gravel",side:"panels",albedo:.42,rough:.5,bump:.016,wet:.8,metalTop:.3},"city.titanium":{top:"panels",side:"panels",albedo:.2,rough:.45,bump:.006,wet:.6},"city.bronze":{top:"panels",side:"panels",albedo:.18,rough:.4,bump:.006,wet:.6},"city.ceramic":{top:"concrete",side:"concrete",albedo:.3,rough:.4,bump:.008,wet:.8,puddle:[.464,.46]},"city.leaf":{top:"foliage",side:"foliage",albedo:.7,rough:.3,bump:.03,wet:.5},ground:{top:"pavers",side:"concrete",albedo:.55,rough:.6,bump:.014,wet:1,topScale:.5,puddle:[.523,.516]}},Rp=new Set(["city.ivory","city.warm"]),wy={low:0,medium:1,high:2,ultra:2},Vs=s=>Number(s).toFixed(4),Ey=`varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
attribute float swSeed;varying float vSwSeed;
#endif`,Ty=`
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
#endif`,Ay=`uniform float swWet;uniform float swRain;uniform float swTime;uniform vec4 swWindow;uniform vec2 swFade;uniform vec4 swShelter[3];uniform vec3 swSky;
varying vec3 vSwPos;varying vec3 vSwNrm;
#ifdef SW_WINDOWS
varying float vSwSeed;
#endif
#ifdef SW_SURFACE
uniform sampler2D swTop;uniform sampler2D swSide;uniform vec2 swScale;
float swHash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}
vec3 swPerturb(vec3 pos,vec3 n,vec2 dh,float face){vec3 sx=dFdx(pos),sy=dFdy(pos),r1=cross(sy,n),r2=cross(n,sx);float det=dot(sx,r1)*face;return normalize(abs(det)*n-sign(det)*(dh.x*r1+dh.y*r2));}
#endif`,Ry=`
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
#endif`,Cy=`
#ifdef SW_SURFACE
diffuseColor.rgb*=swAlbedoF*mix(1.0,0.56,swWetness)*mix(1.0,0.7,swPuddle);
#endif`,Py=`
#ifdef SW_SURFACE
roughnessFactor=clamp(roughnessFactor*swRoughF,0.04,1.0);roughnessFactor=mix(roughnessFactor,roughnessFactor*0.6,swWetness);roughnessFactor=mix(roughnessFactor,0.04,swPuddle);
#endif`,Iy=`
#ifdef SW_SURFACE
metalnessFactor*=mix(1.0,SW_METAL_TOP,swW.y);
#endif`,Ly=`
#if SW_LEVEL>1
normal=swPerturb(-vViewPosition,normal,vec2(dFdx(swH),dFdy(swH))*SW_BUMP*swDetail,faceDirection);
#endif`,Dy=`
#ifdef SW_SURFACE
float swFresnel=pow(1.0-clamp(dot(normal,normalize(vViewPosition)),0.0,1.0),5.0)*0.95+0.05;
totalEmissiveRadiance+=swSky*swFresnel*(swPuddle*0.6+swWetness*0.1);
#endif
#ifdef SW_WINDOWS
float swPane=(1.0-smoothstep(0.35,0.65,abs(swN.y)))*(1.0-step(1.5,vSwSeed));
totalEmissiveRadiance*=mix(swWindow.w,mix(swWindow.z,swWindow.y,step(vSwSeed,swWindow.x)),swPane);
#endif`;function Ny(s){let e=s?.getAttribute("position");if(!e||s.getAttribute("swSeed"))return;let t=e.count,n=new Int32Array(t),i=s.getIndex();for(let u=0;u<t;u++)n[u]=u;let r=u=>{for(;n[u]!==u;)n[u]=n[n[u]],u=n[u];return u},o=u=>i?i.getX(u):u,a=i?i.count:t;for(let u=0;u+2<a;u+=3){let d=r(o(u));n[r(o(u+1))]=d,n[r(o(u+2))]=d}let l=new Map,c=new Map;for(let u=0;u<t;u++){let d=r(u),f=c.get(d)||[0,0,0];l.set(d,(l.get(d)||0)+1),f[0]+=e.getX(u),f[1]+=e.getY(u),f[2]+=e.getZ(u),c.set(d,f)}let h=new Float32Array(t);for(let u=0;u<t;u++){let d=r(u),f=l.get(d),g=c.get(d);if(f>6){h[u]=2;continue}let y=Math.sin(g[0]/f*12.9898+g[1]/f*78.233+g[2]/f*37.719)*43758.5453;h[u]=y-Math.floor(y)}s.setAttribute("swSeed",new xt(h,1))}function Cp(s,e={}){let t=new AbortController,n=new Map,i=[],r=new es(new Uint8Array([128,128,128,255]),1,1);r.colorSpace=Xn,r.needsUpdate=!0;let o={},a={},l={};for(let L of Object.values(Qo))for(let V of[L.top,L.side])o[V]||={value:r};for(let L of Object.keys(Qo))l[L]={value:new _e(1/4,1/4)};let c=(e.shelters||[]).slice(0,3).map(L=>new _t(...L));for(;c.length<3;)c.push(new _t(1e6,1e6,-1e6,-1e6));let h={swWet:{value:0},swRain:{value:0},swTime:{value:0},swShelter:{value:c},swSky:{value:new xe(528926)},swWindow:{value:new _t(.8,1,.06,1)},swFade:{value:new _e(35,140)}},u=2,d=null,f=!1,g=0,y=0,m=0,p=0,b=!1,S=s.capabilities.getMaxAnisotropy(),x=()=>u>1?Math.min(8,S):Math.min(4,S);function A(L,V){let{kind:C,windows:N,space:U}=n.get(V),E=Qo[C];Object.assign(L.uniforms,h);let G=["#define SW_LEVEL "+(E?u:0)];U==="world"&&G.push("#define SW_WORLD"),N&&G.push("#define SW_WINDOWS"),E&&(L.uniforms.swTop=o[E.top],L.uniforms.swSide=o[E.side],L.uniforms.swScale=l[C],G.push("#define SW_SURFACE","#define SW_ALBEDO "+Vs(E.albedo),"#define SW_ROUGH "+Vs(E.rough),"#define SW_BUMP "+Vs(E.bump),"#define SW_WETK "+Vs(U==="world"?E.wet:E.wet*.5),"#define SW_METAL_TOP "+Vs(E.metalTop??1),"#define SW_PUDDLE_DRY "+Vs(E.puddle?.[0]??-1),"#define SW_PUDDLE_FULL "+Vs(E.puddle?.[1]??-2)));let Y=G.join(`
`)+`
`;L.vertexShader=Y+L.vertexShader.replace("#include <common>",`#include <common>
`+Ey).replace("#include <begin_vertex>","#include <begin_vertex>"+Ty),L.fragmentShader=Y+L.fragmentShader.replace("#include <common>",`#include <common>
`+Ay).replace("#include <map_fragment>","#include <map_fragment>"+Ry).replace("#include <color_fragment>","#include <color_fragment>"+Cy).replace("#include <roughnessmap_fragment>","#include <roughnessmap_fragment>"+Py).replace("#include <metalnessmap_fragment>","#include <metalnessmap_fragment>"+Iy).replace("#include <normal_fragment_maps>","#include <normal_fragment_maps>"+Ly).replace("#include <emissivemap_fragment>","#include <emissivemap_fragment>"+Dy)}function T(L,V="world",C=L?.name){if(!L?.isMeshStandardMaterial||n.has(L))return;let N=Qo[C]?C:"",U=Rp.has(L.name);!N&&!U||(n.set(L,{kind:N,windows:U,space:V}),L.addEventListener("dispose",()=>n.delete(L)),L.onBeforeCompile=E=>A(E,L),L.customProgramCacheKey=()=>["sw",N,U,V,N?u:0].join("|"),L.needsUpdate=!0)}function I(L,V=!1){if(L?.isMesh)for(let C of Array.isArray(L.material)?L.material:[L.material])T(C,V?"object":"world"),Rp.has(C?.name)&&Ny(L.geometry)}let v=L=>e.url?.(L),D=null;async function w(L){let V=await fetch(v(L.file),{signal:t.signal});if(!V.ok)throw Error("Surface texture unavailable");let C=await V.blob();if(f)return;let N=await createImageBitmap(C,{premultiplyAlpha:"none",colorSpaceConversion:"none"});if(f){N.close?.();return}let U=new jt(N);U.wrapS=U.wrapT=ui,U.colorSpace=Xn,U.flipY=!1,U.anisotropy=x(),U.needsUpdate=!0,i.push(U),o[L.id].value=U,m++,g+=C.size,e.onProgress?.(g,y)}function R(){return d||f||!e.url||(d=(async()=>{if(!D){let C=await fetch(v("manifest.json"),{signal:t.signal});if(!C.ok)throw Error("Surface manifest unavailable");D=await C.json();let N=(D.textures||[]).filter(U=>o[U.id]&&/^[a-z-]+\.webp$/.test(U.file)&&U.tile_metres>0);D.entries=N,y=N.reduce((U,E)=>U+(E.bytes||0),0);for(let U of N)a[U.id]=U.tile_metres;for(let[U,E]of Object.entries(Qo))l[U].value.set(1/((a[E.top]||4)/(E.topScale||1)),1/(a[E.side]||4))}let L=D.entries.filter(C=>o[C.id].value===r),V=await Promise.allSettled(L.map(w));p+=V.filter(C=>C.status==="rejected").length})().catch(()=>{p++}).finally(()=>{d=null})),d}return{apply:T,prepare:I,setTier(L){let V=wy[L]??2;if(V!==u){u=V;for(let[C,N]of n)N.kind&&(C.needsUpdate=!0)}for(let C of i)C.anisotropy!==x()&&(C.anisotropy=x(),C.needsUpdate=!0);u>0&&R()},setLighting(L,V,C=0,N=null){N&&h.swSky.value.copy(N).multiplyScalar(1.25);let U=1-L,E=wt.smoothstep(U,.1,.9);h.swWindow.value.set(Math.min(.9,.12+.72*E+.12*C),.22+U*.98+Math.min(1,V)*.1,.05,.55+.45*U)},setWeather(L){b=L==="rain",h.swRain.value=b?1:0},update(L,V){let C=b?1:0,N=h.swWet.value;if(!V){h.swWet.value=C;return}let U=Math.min(.1,Math.max(0,L));h.swTime.value+=U,h.swWet.value=N+(C-N)*Math.min(1,U/(C>N?14:70))},progress:()=>({bytes:g,expected:y}),stats:()=>({detail:u,textures:m,failures:p,bytes:g,materials:n.size,wet:+h.swWet.value.toFixed(2),windows:+h.swWindow.value.x.toFixed(2)}),dispose(){f=!0,t.abort();for(let L of i)L.dispose(),L.source.data?.close?.();i.length=0,r.dispose()}}}function Uy(s){let e=Math.max(64,Math.round(s.sampleRate*.03)||1440),t=[],n=0;for(let o=0;o<s.numberOfChannels;o++){let a=s.getChannelData(o);for(let l=0;l<a.length;l+=e){let c=0,h=Math.min(l+e,a.length);for(let u=l;u<h;u++){let d=a[u];if(!Number.isFinite(d))throw Error("Invalid voice");n=Math.max(n,Math.abs(d)),c+=d*d}t.push(Math.sqrt(c/(h-l)))}}if(n<1e-4)throw Error("Silent voice");let i=t.filter(o=>o>Math.max(.003,n*.003)).sort((o,a)=>o-a),r=i[Math.floor((i.length-1)*.7)]||n*.5;return Math.min(8,Math.max(1/n,.26/Math.max(r,.001)))}function Pp(s,e){let t=!1,n=!1,i=0,r=null,o=null,a=[],l=null,c=null,h=null,u=250,d=0,f=0,g=0,y="idle",m=null,p=()=>.04+.46/(1+(u/110)**2);function b(){let v=s.currentTime;l?.gain.setTargetAtTime(p(),v,.18),c?.frequency.setTargetAtTime(1800+4400/(1+u/95),v,.18),h?.pan.setTargetAtTime(d,v,.18)}function S(){if(o){o.onended=null;try{o.stop()}catch{}o=null}a.forEach(v=>v.disconnect()),a=[],l=c=h=null}function x(){f++,clearTimeout(i),i=0,r?.abort(),r=null,S(),y="idle"}function A(v=2e4+Math.random()*2e4){clearTimeout(i),t&&!n&&(i=setTimeout(()=>{i=0,I()},v))}function T(){if(m)return m;m=s.createBuffer(2,Math.ceil(s.sampleRate*.85),s.sampleRate);let v=42;for(let D=0;D<2;D++){let w=m.getChannelData(D),R=Math.round(s.sampleRate*.015),L=0;for(let C=0;C<w.length;C++){v=Math.imul(v,1664525)+1013904223>>>0;let N=(C-R)/(w.length-R);w[C]=C<R?0:(v/4294967296*2-1)*Math.pow(1-N,2.25),L+=w[C]*w[C]}let V=.65/Math.sqrt(L||1);for(let C=0;C<w.length;C++)w[C]*=V}return m}async function I(){if(!t||n||r||o||s.state!=="running"){A();return}let v=f,D=new AbortController;r=D,y="loading";let w=setTimeout(()=>D.abort(),32e3);try{let R=await fetch("/api/desktop/system-world/voice",{method:"POST",credentials:"same-origin",cache:"no-store",signal:D.signal});if(R.status===204){y="idle";return}if(!R.ok||!R.headers.get("Content-Type")?.startsWith("audio/"))throw Error("Voice unavailable");if(Number(R.headers.get("Content-Length"))>8*1024*1024)throw Error("Voice too large");let L=await R.arrayBuffer();if(L.byteLength>8*1024*1024)throw Error("Voice too large");if(v!==f||!t||n)return;let V=await s.decodeAudioData(L);if(v!==f||!t||n)return;if(!Number.isFinite(V.duration)||V.duration<=0||V.duration>25||V.numberOfChannels>2)throw Error("Invalid voice");let C=Uy(V),N=oe=>(a.push(oe),oe);o=N(s.createBufferSource()),o.buffer=V;let U=N(s.createGain()),E=N(s.createBiquadFilter());U.gain.value=C,E.type="highpass",E.frequency.value=90,c=N(s.createBiquadFilter()),c.type="lowpass",c.Q.value=.55;let G=N(s.createGain()),Y=N(s.createGain()),X=N(s.createGain()),ie=N(s.createGain());G.gain.value=.76,Y.gain.value=.22,X.gain.value=.28;let ce=N(s.createDelay(.2)),be=N(s.createConvolver());ce.delayTime.value=.085,be.normalize=!1,be.buffer=T();let Le=N(s.createWaveShaper()),Re=new Float32Array(1024);for(let oe=0;oe<Re.length;oe++)Re[oe]=Math.max(-.75,Math.min(.75,oe*2/(Re.length-1)-1));Le.curve=Re,l=N(s.createGain()),l.gain.value=p(),h=N(s.createStereoPanner()),h.pan.value=d,o.connect(U).connect(E).connect(c),c.connect(G).connect(ie),c.connect(ce).connect(Y).connect(ie),c.connect(be).connect(X).connect(ie),ie.connect(Le).connect(l).connect(h).connect(e),b(),y="speaking",g++,o.onended=()=>{v!==f||!t||n||(y="tail",i=setTimeout(()=>{S(),y="idle",A()},900))},o.start()}catch{v===f&&!n&&(y=D.signal.aborted?"idle":"unavailable")}finally{clearTimeout(w),r===D&&(r=null),v===f&&!n&&!o&&A(y==="unavailable"?6e4:void 0)}}return{setActive(v){t===v||n||(t=v,t?A(3e3+Math.random()*2e3):x())},setListener(v,D,w,R,L){if(!Number.isFinite(v)||!Number.isFinite(D)||!Number.isFinite(w)||!Number.isFinite(R)||!Number.isFinite(L))return;let V=-v,C=-12-w,N=Math.max(0,Math.hypot(V,C)-13);u=Math.hypot(N,Math.max(0,D-87,-D));let U=Math.hypot(V,C),E=Math.hypot(R,L);d=U&&E?Math.max(-.8,Math.min(.8,(C*R-V*L)/U/E))*.8:0,b()},stats(){return{state:y,phrases:g,distance:u,gain:p(),pan:d,pending:!!r,scheduled:!!i}},dispose(){n||(n=!0,t=!1,x(),m=null)}}}function Ip(s,e,t=e){let n=new Set,i={x:0,z:0,fx:0,fz:-1},r=!1,o=!1,a="clear",l=s.createBuffer(1,s.sampleRate*2,s.sampleRate),c=31,h=l.getChannelData(0);for(let p=0;p<h.length;p++)c=Math.imul(c,1664525)+1013904223|0,h[p]=(c>>>0)/2147483648-1;let u=s.createBufferSource(),d=s.createBiquadFilter(),f=s.createGain();u.buffer=l,u.loop=!0,d.type="lowpass",d.frequency.value=450,f.gain.value=0,u.connect(d).connect(f).connect(t),u.start();let g=[{x:-57,z:-57,f:94,g:.055},{x:43,z:62,f:143,g:.035},{x:0,z:14,f:220,g:.016},{x:0,z:79,f:340,g:.06}].map((p,b)=>{let S=b===3?s.createBufferSource():s.createOscillator(),x=s.createGain(),A=s.createStereoPanner(),T=s.createBiquadFilter();return b===3?(S.buffer=l,S.loop=!0):(S.type="sine",S.frequency.value=p.f),T.type="lowpass",T.frequency.value=p.f*2,x.gain.value=0,S.connect(T).connect(x).connect(A).connect(t),S.start(),{...p,source:S,level:x,pan:A,low:T}});function y(){f.gain.setTargetAtTime(r?(a==="rain"?.07:.015)*(o?.12:1):0,s.currentTime,.1),d.frequency.setTargetAtTime(o?280:a==="rain"?2100:600,s.currentTime,.2)}function m(){for(let p of[...n]){try{p.source.stop()}catch{}p.release()}}return{setActive(p){if(r=p,y(),!p){m();for(let b of g)b.level.gain.setTargetAtTime(0,s.currentTime,.05)}},environment(p,b){o=p,b&&(a=b),y()},listener(p,b,S,x,A){Object.assign(i,{x:p,z:S,fx:x,fz:A});for(let T of g){let I=Math.hypot(p-T.x,S-T.z);T.level.gain.setTargetAtTime(r?T.g/(1+I*I*.025)*(o?.25:1):0,s.currentTime,.1),T.pan.pan.value=Math.max(-1,Math.min(1,((T.x-p)*-A+(T.z-S)*x)/Math.max(1,I)))}},play(p,b=0,S=0,x=0){if(!r||n.size>=16)return;let A=Math.hypot(b-i.x,x-i.z);if(A>90)return;let T={step:[.12,240,.16],step_inside:[.1,550,.12],door:[.7,850,.06],lift:[1.5,120,.05],tram:[.8,330,.08],discover:[.45,880,.08]}[p];if(!T)return;let[I,v,D]=T,w=p.startsWith("step")||p==="door"?s.createBufferSource():s.createOscillator();"buffer"in w?w.buffer=l:(w.type="sine",w.frequency.value=v);let R=s.createBiquadFilter(),L=s.createGain(),V=s.createStereoPanner();R.type="lowpass",R.frequency.value=o?v*.7:v,L.gain.setValueAtTime(0,s.currentTime),L.gain.linearRampToValueAtTime(D/(1+A*.08),s.currentTime+.015),L.gain.exponentialRampToValueAtTime(1e-4,s.currentTime+I),V.pan.value=Math.max(-1,Math.min(1,((b-i.x)*-i.fz+(x-i.z)*i.fx)/Math.max(A,1))),w.connect(R).connect(L).connect(V).connect(e);let C={source:w,release(){if(n.delete(C))for(let N of[w,R,L,V])N.disconnect()}};n.add(C),w.onended=C.release,w.start(),w.stop(s.currentTime+I+.02)},stats:()=>({voices:n.size,weather:a,inside:o}),dispose(){r=!1,m(),u.stop();for(let p of g){p.source.stop();for(let b of[p.source,p.level,p.pan,p.low])b.disconnect()}for(let p of[u,d,f])p.disconnect()}}}var Lu=[[110,164.81,246.94,261.63],[87.31,130.81,220,329.63],[130.81,196,246.94,329.63],[98,146.83,246.94,293.66]],Lp=[440,523.25,587.33,659.25,783.99,880,1046.5],Du=12;function Fy(s,e,t){let n=Math.floor(s.sampleRate*e),i=s.createBuffer(2,n,s.sampleRate),r=977;for(let o=0;o<2;o++){let a=i.getChannelData(o);for(let l=0;l<n;l++)r=Math.imul(r,1664525)+1013904223|0,a[l]=((r>>>0)/2147483648-1)*Math.pow(1-l/n,t)}return i}function Oy(s=()=>{}){let e=null,t=null,n=null,i=!1,r=!1,o=!1,a=0,l=null,c=[],h=[],u=new Float32Array(256),d=new Set,f=.18,g=null,y=null,m=null,p=null,b=!1,S="clear",x=[],A=null,T=null,I=null,v=null,D=null,w=null,R=0,L=0,V=0,C=0,N={busy:!1,day:0,evening:0,weather:"clear"},U={ambience:1,effects:1,voice:1};try{o=localStorage.getItem("aurago.desktop.sysworld.sound")==="true"}catch{}function E(){if(e||i)return;e=new(window.AudioContext||window.webkitAudioContext);let H=M=>(c.push(M),M),Z=M=>(h.push(M),H(M));t=H(e.createGain()),t.gain.value=0,n=H(e.createAnalyser()),n.fftSize=512,t.connect(n),n.connect(e.destination),y=H(e.createGain()),y.gain.value=U.ambience,y.connect(t),m=H(e.createGain()),m.gain.value=U.effects,m.connect(t),p=H(e.createGain()),p.gain.value=U.voice,p.connect(t),D=H(e.createConvolver()),D.buffer=Fy(e,3.4,2.6);let W=H(e.createGain());W.gain.value=.55,D.connect(W).connect(y),l=Pp(e,p),g=Ip(e,m,y),g.environment(b,S);for(let[M,_]of[[55,.07],[82.4069,.022],[110,.01]]){let O=Z(e.createOscillator()),k=H(e.createGain());O.frequency.value=M,k.gain.value=_,O.connect(k).connect(y),O.start()}A=H(e.createBiquadFilter()),A.type="lowpass",A.frequency.value=650,A.Q.value=.6,T=H(e.createGain()),T.gain.value=.042;let J=H(e.createGain());J.gain.value=.9,A.connect(T).connect(y),T.connect(J).connect(D),x=Lu[0].map(M=>[-6,6].map(_=>{let O=Z(e.createOscillator()),k=H(e.createGain());return O.type="sawtooth",O.frequency.value=M,O.detune.value=_,k.gain.value=.22,O.connect(k).connect(A),O.start(),O}));let te=Z(e.createOscillator()),pe=H(e.createGain());te.frequency.value=.05,pe.gain.value=180,te.connect(pe).connect(A.frequency),te.start();let we=e.createBuffer(1,e.sampleRate*4,e.sampleRate),Pe=we.getChannelData(0),je=0;for(let M=0;M<Pe.length;M++)je=(je+(Math.random()*2-1)*.018)/1.018,Pe[M]=je;let Xe=Pe[Pe.length-1]-Pe[0];for(let M=0;M<Pe.length;M++)Pe[M]-=Xe*M/(Pe.length-1);let Je=Z(e.createBufferSource());I=H(e.createBiquadFilter()),v=H(e.createGain()),Je.buffer=we,Je.loop=!0,I.type="lowpass",I.frequency.value=680,I.Q.value=.25,v.gain.value=.32,Je.connect(I).connect(v).connect(y),Je.start();let q=Z(e.createOscillator()),it=H(e.createGain());q.frequency.value=.075,it.gain.value=160,q.connect(it).connect(I.frequency),q.start(),w=e.createBuffer(1,e.sampleRate*2,e.sampleRate);let Ue=w.getChannelData(0);for(let M=0;M<Ue.length;M++)Ue[M]=Math.random()*2-1;he()}let G=()=>!!e&&o&&r&&!i&&f>0;function Y(H){let Z={release(){if(d.delete(Z))for(let W of H){try{W.stop?.()}catch{}try{W.disconnect()}catch{}}}};return d.add(Z),H[0].onended=Z.release,C++,Z}function X(H,Z,W){let J=e.createStereoPanner(),te=e.createGain();return J.pan.value=Z,te.gain.value=W,H.connect(J).connect(y),J.connect(te).connect(D),[J,te]}function ie(){let H=e.currentTime,Z=Lp[Math.floor(Math.random()*Lp.length)],W=e.createOscillator(),J=e.createOscillator(),te=e.createGain(),pe=e.createGain();W.frequency.value=Z,J.frequency.value=Z*2.76,te.gain.value=.22,pe.gain.setValueAtTime(0,H),pe.gain.linearRampToValueAtTime(N.busy?.02:.014,H+.008),pe.gain.exponentialRampToValueAtTime(1e-4,H+2.8),W.connect(pe),J.connect(te).connect(pe),Y([W,J,te,pe,...X(pe,Math.random()*1.4-.7,1.4)]),W.start(H),J.start(H),W.stop(H+2.9),J.stop(H+2.9)}function ce(){let H=e.currentTime,Z=e.createBufferSource(),W=e.createBiquadFilter(),J=e.createGain(),te=Math.random()<.5?-1:1;Z.buffer=w,Z.loop=!0,W.type="bandpass",W.Q.value=1.4,W.frequency.setValueAtTime(260,H),W.frequency.exponentialRampToValueAtTime(1100,H+1.6),W.frequency.exponentialRampToValueAtTime(380,H+3.4),J.gain.setValueAtTime(0,H),J.gain.linearRampToValueAtTime(.03,H+1.6),J.gain.exponentialRampToValueAtTime(1e-4,H+3.6),Z.connect(W).connect(J);let[pe,we]=X(J,-.9*te,.5);pe.pan.setValueAtTime(-.9*te,H),pe.pan.linearRampToValueAtTime(.9*te,H+3.4),Y([Z,W,J,pe,we]),Z.start(H),Z.stop(H+3.7)}function be(){let H=e.currentTime,Z=S==="rain"?.36:.32;I.frequency.cancelScheduledValues(H),v.gain.cancelScheduledValues(H),I.frequency.setTargetAtTime(1200+Math.random()*700,H,.8),I.frequency.setTargetAtTime(680,H+2.6,1.4),v.gain.setTargetAtTime(Z*1.7,H,.7),v.gain.setTargetAtTime(Z,H+2.6,1.5),C++}function Le(){let H=e.currentTime,Z=e.createBiquadFilter(),W=e.createGain();Z.type="lowpass",Z.frequency.value=420,W.gain.setValueAtTime(0,H),W.gain.linearRampToValueAtTime(.028,H+.9),W.gain.setValueAtTime(.028,H+3.2),W.gain.exponentialRampToValueAtTime(1e-4,H+5.5);let J=[69.3,103.83].map(te=>{let pe=e.createOscillator();return pe.type="sawtooth",pe.frequency.value=te,pe.detune.value=Math.random()*8-4,pe.connect(Z),pe});Z.connect(W),Y([...J,Z,W,...X(W,-.55,1.6)]),J.forEach(te=>{te.start(H),te.stop(H+5.6)})}function Re(){let H=e.currentTime,Z=2+Math.floor(Math.random()*3),W=Math.random()*1.2-.6;for(let J=0;J<Z&&d.size<Du;J++){let te=H+J*(.12+Math.random()*.1),pe=e.createOscillator(),we=e.createGain(),Pe=2600+Math.random()*900;pe.frequency.setValueAtTime(Pe,te),pe.frequency.exponentialRampToValueAtTime(Pe*1.5,te+.07),we.gain.setValueAtTime(0,te),we.gain.linearRampToValueAtTime(.006,te+.015),we.gain.exponentialRampToValueAtTime(1e-4,te+.13),pe.connect(we),Y([pe,we,...X(we,W,.6)]),pe.start(te),pe.stop(te+.15)}}function oe(){if(d.size>=Du)return;let H=Math.random(),Z=1-N.day;H<.34?ie():H<.62?ce():H<.8?be():Z>.5&&N.weather!=="rain"&&H<.88?Le():N.day>.4&&N.weather==="clear"?Re():ie()}function ge(){clearTimeout(R),R=0,G()&&(R=setTimeout(()=>{R=0,G()&&(oe(),ge())},(N.busy?2200:4200)+Math.random()*6e3))}function ae(){clearTimeout(L),L=0,G()&&(L=setTimeout(()=>{L=0,G()&&(V=(V+1)%Lu.length,x.forEach((H,Z)=>H.forEach(W=>W.frequency.setTargetAtTime(Lu[V][Z],e.currentTime,1.8))),ae())},16e3))}function Se(){clearTimeout(R),clearTimeout(L),R=L=0;for(let H of[...d])H.release()}function he(){if(!e)return;let H=e.currentTime;A.frequency.setTargetAtTime(N.busy?1500:650+N.day*450+N.evening*150,H,2.5),T.gain.setTargetAtTime((N.busy?.055:.042)*(N.weather==="rain"?.75:1),H,2)}function ue(H=!1){if(clearTimeout(a),!i){if(H&&o)try{E()}catch{o=!1,s(!1);return}e&&(l?.setActive(o&&r&&f>0&&U.voice>0),g?.setActive(o&&r&&f>0),o&&r?(e.resume().catch(()=>{}),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(f,e.currentTime,.25),R||ge(),L||ae()):(Se(),t.gain.cancelScheduledValues(e.currentTime),t.gain.setTargetAtTime(0,e.currentTime,.035),a=setTimeout(()=>{!i&&(!o||!r)&&e.suspend().catch(()=>{})},180)))}}return{enabled:()=>o,toggle(){o=!o;try{localStorage.setItem("aurago.desktop.sysworld.sound",String(o))}catch{}ue(!0),s(o)},unlock(){ue(!0)},setActive(H){r!==H&&(r=H,ue())},setVolume(H){Number.isFinite(H)&&(f=Math.max(0,Math.min(.35,H)),ue())},setListener(H,Z,W,J,te){l?.setListener(H,Z,W,J,te),g?.listener(H,Z,W,J,te)},setChannel(H,Z){if(!(H in U)||!Number.isFinite(Z))return;U[H]=Math.max(0,Math.min(1,Z));let W={ambience:y,effects:m,voice:p}[H];W&&W.gain.setTargetAtTime(U[H],e.currentTime,.05),ue()},effect(H,Z,W,J){g?.play(H,Z,W,J)},setEnvironment(H,Z){b=!!H,["clear","rain","fog"].includes(Z)&&(S=Z),g?.environment(b,S)},setMood(H={}){Object.assign(N,{busy:!!H.busy,day:Number.isFinite(H.day)?H.day:N.day,evening:Number.isFinite(H.evening)?H.evening:N.evening,weather:["clear","rain","fog"].includes(H.weather)?H.weather:N.weather}),he()},thunder(H=1){if(!G()||b||d.size>=Du)return;let Z=Math.max(0,Math.min(5,H)),W=e.currentTime+Z,J=Math.min(1,Z/4),te=e.createBufferSource(),pe=e.createBiquadFilter(),we=e.createGain();te.buffer=w,te.loop=!0,pe.type="lowpass",pe.Q.value=.7,pe.frequency.setValueAtTime(900-J*500,W),pe.frequency.exponentialRampToValueAtTime(70,W+3.5),we.gain.setValueAtTime(0,W),we.gain.linearRampToValueAtTime(.09*(1-J*.5),W+.06+J*.25),we.gain.exponentialRampToValueAtTime(.025,W+1.3),we.gain.exponentialRampToValueAtTime(1e-4,W+5.5),te.connect(pe).connect(we),Y([te,pe,we,...X(we,Math.random()*.8-.4,.8)]),te.start(W),te.stop(W+5.6)},stats(){let H=0;if(n&&e.state==="running"){n.getFloatTimeDomainData(u);for(let Z of u)H+=Z*Z}return{voice:l?.stats(),effects:g?.stats(),channels:{...U},enabled:o,active:r,state:e?.state||"uninitialized",volume:f,rms:Math.sqrt(H/u.length),soundscape:{chord:V,events:C,transient:d.size,scheduled:!!R,mood:{...N}}}},dispose(){i||(i=!0,Se(),g?.dispose(),l?.dispose(),clearTimeout(a),h.forEach(H=>{try{H.stop()}catch{}}),c.forEach(H=>H.disconnect()),e&&e.close().catch(()=>{}))}}}var Tn=[{id:"agent",asset:"agent-spire",x:0,z:-12,height:87,radius:13},{id:"infra",asset:"compute-foundry",x:-43,z:-53,height:24,radius:18},{id:"integrations",asset:"integration-gate",x:43,z:-10,height:34,radius:14},{id:"missions",asset:"mission-terminal",x:43,z:36,height:23,radius:15},{id:"memory",asset:"memory-archive",x:-43,z:-10,height:37,radius:14},{id:"graph",asset:"knowledge-atrium",x:-43,z:36,height:28,radius:17},{id:"operations",asset:"operations-beacon",x:0,z:36,height:22,radius:6}],bc={low:{lod:2,dpr:1,shadow:0,bloom:!1},medium:{lod:1,dpr:1.25,shadow:1024,bloom:!1},high:{lod:0,dpr:1.5,shadow:2048,bloom:!0},ultra:{lod:0,dpr:2,shadow:4096,bloom:!0}},Dp=new B(122,106,183),Nu=class extends On{constructor(e,t,n){super(),this.scene=e,this.camera=t,this.needsSwap=!1,this.target=new Wt(1,1,{type:nn,samples:n}),this.quad=new Si(new mt({uniforms:oi.clone(ki.uniforms),vertexShader:ki.vertexShader,depthTest:!1,depthWrite:!1,fragmentShader:"uniform sampler2D tDiffuse;varying vec2 vUv;void main(){vec4 c=texture2D(tDiffuse,vUv);if(any(isnan(c))||any(isinf(c)))c=vec4(0.0,0.0,0.0,1.0);gl_FragColor=c;}"}))}render(e,t,n){let i=e.autoClear;e.autoClear=!1,e.setRenderTarget(this.target),e.clear(),e.render(this.scene,this.camera),this.quad.material.uniforms.tDiffuse.value=this.target.texture,e.setRenderTarget(this.renderToScreen?null:n),this.quad.render(e),e.autoClear=i}setSize(e,t){this.target.setSize(e,t)}dispose(){this.target.dispose(),this.quad.material.dispose(),this.quad.dispose()}},Np=new B(0,22,-12),Gs=[];{let s=(n,i,r,o=0,a=0,l=[1,1,1])=>Gs.push({asset:n,x:i,z:r,y:o,angle:a,scale:l}),{xs:e,zs:t}=En;for(let n of t){for(let r of e)s("street-crossing",r,n);let i=[En.minX,...e,En.maxX];for(let r=0;r<i.length-1;r++){let o=i[r]+(r?6:0),a=i[r+1]-(r<i.length-2?6:0);s("street-tile",(o+a)/2,n,0,0,[(a-o)/16,1,1])}}for(let n of e){let i=[En.minZ,...t,En.maxZ];for(let r=0;r<i.length-1;r++){let o=i[r]+(r?6:0),a=i[r+1]-(r<i.length-2?6:0);s("street-tile",n,(o+a)/2,0,Math.PI/2,[(a-o)/16,1,1])}}for(let n of t)for(let i of[-55,-31,31,55])s("street-lamp",i,n-4.5,.45),s("planter",i+4,n-4.5,.45);for(let{x:n,z:i,scale:r}of yc)s("data-tower-a",n,i,0,0,[1,r,1]);s("skybridge",39.75,-55,15),s("server-rack",-55,-36.7,.5),s("server-rack",-49,-36.7,.5);for(let n=0;n<48;n++){let i=(n%12-5.5)*22,r=-143-Math.floor(n/12)*29,o=.7+n*17%13/12;s(n%2?"data-tower-a":"data-tower-b",i,r,-3,0,[.8,o,.8])}}var By=[...Jf(Gs),...pn.flatMap(s=>[-3,3].flatMap(e=>[-3,3].map(t=>({x:s.x+e,z:s.z+t,r:4.25,asset:"interior"}))))];async function e1(s,e){let t=!1,n=!0,i="orbit",r=e.quality||"auto",o=r==="auto"?"high":r,a=0,l=null,c=0,h=0,u=0,d=0,f=0,g=0,y=0,m=null,p=e.reducedMotion,b=1,S=1,x=[],A=!1,T=null,I=null,v=new AbortController,D=new Set,w=new Map,R=new Map,L=new Set,V=new Set,C=new Set,N=[],U=new sc({antialias:!0,alpha:!1});U.toneMapping=Us,U.toneMappingExposure=.98,U.shadowMap.type=cl,U.shadowMap.autoUpdate=!1,U.info.autoReset=!1;let E=U.domElement;E.className="sysworld-gl",E.tabIndex=0,E.setAttribute("aria-label",e.label),s.append(E);let G=new Rs;G.background=new xe(528926),G.fog=new io(528926,.004);let Y=new Qt(43,1,.3,1800);Y.position.copy(Dp);let X=_c(),ie=X.register("visitor",{circles:[{x:0,z:0,r:.38}],minY:-.4,maxY:.4,reach:.38},Y.position,100),ce=new hc(Y,E);ce.target.copy(Np),ce.enableDamping=!0,ce.dampingFactor=.085,ce.minDistance=12,ce.maxDistance=410,ce.maxPolarAngle=Math.PI*.485,ce.update();let be=Cp(U,{url:ee=>e.resourceURL("/3d/system-world/textures/v1/"+ee),shelters:pn.map(ee=>[ee.x-ee.width/2,ee.z-ee.depth/2,ee.x+ee.width/2,ee.z+ee.depth/2])}),Le=new Dr(U),Re=new uc,oe=Le.fromScene(Re,.05);G.environment=oe.texture,G.environmentIntensity=.48,Re.dispose(),Le.dispose();let ge=new yo(12639487,1056813,.8);G.add(ge);let ae=new ss(13164287,3.2);ae.position.set(-70,145,85),ae.castShadow=!0,Object.assign(ae.shadow.camera,{left:-120,right:120,top:120,bottom:-120,near:1,far:360}),ae.shadow.normalBias=.09,ae.shadow.bias=-15e-5,G.add(ae);let Se=new ss(16758130,2.4);Se.position.set(90,80,-120),G.add(Se);let he=new ni(6479871,0,75,2);he.position.set(0,34,-12),G.add(he);let ue=new Wt(1,1,{type:nn}),H=new fc(U,ue);H.addPass(new Nu(G,Y,Math.min(4,U.capabilities.maxSamples)));let Z=new Br(new _e(1,1),.28,.55,1.25);H.addPass(Z),H.addPass(new pc);let W=new ct,J=new ct,te=new ct;G.add(W),W.add(J,te);function pe(ee,Oe){return L.add(ee),V.add(Oe),new Ke(ee,Oe)}let we=pe(new mi(170,3,174),new yn({color:1911345,metalness:.05,roughness:.82}));we.position.set(0,-1.5,-7),we.receiveShadow=!0,W.add(we),be.apply(we.material,"world","ground");let Pe=up(G,{sunDirection:ae.position,lamps:Gs.filter(ee=>ee.asset==="street-lamp")});H.addPass(Pe.post);let je=Tn.find(ee=>ee.id==="memory"),Xe=sp(G,je,{roof:21,label:e.memoryLabel}),Je=fp(G,{traffic:X,surfaces:be}),q=Tp(G,{sun:ae,rim:Se,hemisphere:ge,atmosphere:Pe,surfaces:be,onThunder:ee=>e.onThunder?.(ee)}),it="",Ue=null,M=pe(new Ls(1,1.035,64),new bt({color:9365742,transparent:!0,opacity:.85,depthWrite:!1,side:Nt}));M.rotation.x=-Math.PI/2,M.visible=!1,W.add(M);let _=new bt({color:6742230,toneMapped:!1}),O=new vn(new gi(.65,10,6),_,Tn.length);L.add(O.geometry),V.add(_),W.add(O);let k=new St,P=new B,z=new Ao,$=new _e;Tn.forEach((ee,Oe)=>{k.position.set(ee.x,ee.height+2,ee.z),k.updateMatrix(),O.setMatrixAt(Oe,k.matrix),O.setColorAt(Oe,new xe(8425630))}),e.signal?.addEventListener("abort",Vi,{once:!0});let j;try{if(e.signal?.aborted)throw Error("Disposed");let ee=await fetch(e.assetURL("manifest.json"),{signal:v.signal});if(!ee.ok)throw Error("City manifest unavailable");j=await ee.json()}catch(ee){throw Vi(),ee}let ne=new Map(j.assets.map(ee=>[ee.id,ee]));Ap(X,ne,Tn,Gs),Ue=wp(G,{camera:Y,districts:Tn,tier:o,traffic:X,reduced:()=>p,active:()=>n&&!A&&i!=="map"&&!p&&!e.replaying?.(),assetURL:ee=>e.resourceURL("/3d/system-world/v2/"+ee),surfaces:be,onInteraction:e.onInteraction,onDiscover:e.onDiscover,onSound:e.onSound,onTerminal:e.onTerminal,onSociety:e.onSociety,onEnvironment:ee=>{q.setIndoor(ee),e.onEnvironment?.(ee)},onError:e.onError,onReady:()=>{U.shadowMap.needsUpdate=!0}});async function de(ee,Oe){let st=ee+":"+Oe;return w.has(st)||w.set(st,(async()=>{let lt=ne.get(ee)?.lods.find(It=>It.level===Oe);if(!lt||!/^[a-z0-9-]+\.lod[0-2]\.glb$/.test(lt.file))throw Error("Invalid city asset");let tt=new AbortController;D.add(tt);let Ht=setTimeout(()=>tt.abort(),15e3);try{let It=await fetch(e.assetURL(lt.file),{signal:tt.signal});if(!It.ok)throw Error("City asset unavailable");let Ft=await It.arrayBuffer();if(t)throw Error("Disposed");let vt=await new ds().parseAsync(Ft,"");if(t)throw vt.scene.traverse(Fe=>{Fe.isMesh&&(Fe.geometry.dispose(),Fe.material.dispose())}),Error("Disposed");return f+=Ft.byteLength,vt.scene.traverse(Fe=>{if(!Fe.isMesh)return;L.add(Fe.geometry),Fe.castShadow=!0,Fe.receiveShadow=!0;let Ot=(Array.isArray(Fe.material)?Fe.material:[Fe.material]).map(dt=>{if(R.has(dt.name)){let Vt=dt;dt=R.get(dt.name),Vt.dispose()}else R.set(dt.name,dt),V.add(dt);return dt});Fe.material=Array.isArray(Fe.material)?Ot:Ot[0],be.prepare(Fe)}),vt.scene}finally{clearTimeout(Ht),D.delete(tt)}})().catch(lt=>{throw w.delete(st),lt})),w.get(st)}function Ce(ee){ee.traverse(Oe=>{Oe.isInstancedMesh&&Oe.dispose()}),ee.clear()}async function Me(){let ee=++a,Oe=bc[o],st=Oe.lod,lt=[...new Set([...Gs.map(tt=>tt.asset),...Tn.map(tt=>tt.asset),"service-drone"])];try{let tt=new Map(await Promise.all(lt.map(async Fe=>[Fe,await de(Fe,st)]))),Ht=new Map(await Promise.all(["data-tower-a","data-tower-b"].map(async Fe=>[Fe,await de(Fe,2)])));if(t||ee!==a)return;Ce(J),Ce(te),x=[];let It=new Map;for(let Fe of Gs){let At=(Fe.z<-100?Ht:tt).get(Fe.asset);At.updateMatrixWorld(!0);let Ot=new Qe().compose(new B(Fe.x,Fe.y,Fe.z),new Zt().setFromAxisAngle(new B(0,1,0),Fe.angle),new B(...Fe.scale));At.traverse(dt=>{if(!dt.isMesh)return;let Vt=dt.uuid;It.has(Vt)||It.set(Vt,{node:dt,matrices:[]}),It.get(Vt).matrices.push(new Qe().multiplyMatrices(Ot,dt.matrixWorld))})}for(let{node:Fe,matrices:At}of It.values()){let Ot=new vn(Fe.geometry,Fe.material,At.length);At.forEach((dt,Vt)=>Ot.setMatrixAt(Vt,dt)),Ot.castShadow=!0,Ot.receiveShadow=!0,J.add(Ot)}for(let Fe of Tn){let At=tt.get(Fe.asset).clone(!0);At.position.set(Fe.x,0,Fe.z),At.userData.district=Fe.id,te.add(At),x.push(At)}I?.attachLandmarks(te),Je.setTemplate(tt.get("service-drone"));let Ft=new en,vt=new Map;for(let Fe of["data-tower-a","data-tower-b"])vt.set(Fe,Ft.setFromObject(Ht.get(Fe)).max.y);Pe.setAviation(Gs.filter(Fe=>vt.has(Fe.asset)&&(Fe.z<-100||Fe.scale[1]>1.1)).map(Fe=>({x:Fe.x,y:Fe.y+vt.get(Fe.asset)*Fe.scale[1]+.6,z:Fe.z}))),U.shadowMap.needsUpdate=!0,e.onReady?.()}catch(tt){!t&&ee===a&&e.onError?.(tt)}}function ve(){if(t)return;b=Math.max(1,s.clientWidth),S=Math.max(1,s.clientHeight);let ee=Math.min(devicePixelRatio||1,bc[o].dpr,Math.sqrt(3840*2160/(b*S)));U.setPixelRatio(ee),U.setSize(b,S,!1),H.setPixelRatio(ee),H.setSize(b,S),Y.aspect=b/S,Y.updateProjectionMatrix(),Xe.setPointScale(S*ee),Pe.setPointScale(S*ee)}function Be(ee){return r=ee in bc||ee==="auto"?ee:"auto",o=r==="auto"?"high":r,Ee(),Me()}function Ee(){let ee=bc[o];U.shadowMap.enabled=ee.shadow>0,ee.shadow&&ae.shadow.mapSize.x!==ee.shadow&&(ae.shadow.mapSize.set(ee.shadow,ee.shadow),ae.shadow.map?.dispose(),ae.shadow.map=null),U.shadowMap.needsUpdate=!0,Z.enabled=ee.bloom,be.setTier(o),Pe.setTier(o),Ue.setTier(o),q.setTier(o),Je.setTier(o),ve(),e.onQuality?.(r,o)}function Ve(ee,Oe){l=null;let st=X.findFree(ie,ee);if(!st)return;if(ee=new B(st.x,st.y,st.z),p){Y.position.copy(ee),Object.assign(ie,st),ce.target.copy(Oe),ce.update(),l=null;return}let lt=(Ft,vt)=>X.clear(ie,{...Ft,heading:0},{...vt,heading:0},!1),tt=Y.position.clone(),Ht=Math.max(tt.y,ee.y,110),It=[tt,ee];if(!lt(tt,ee)){let Ft=[tt],vt=new B(ee.x,Ht,ee.z),Fe=[-1],At=[0],Ot=-1;if(!lt(vt,ee))return;for(let dt of[3,8,16])for(let Vt=0;Vt<8;Vt++)Ft.push(new B(tt.x+Math.cos(Vt*Math.PI/4)*dt,tt.y,tt.z+Math.sin(Vt*Math.PI/4)*dt));for(let dt of pn)if(Math.hypot(tt.x-dt.x,tt.z-dt.z)<24)for(let Vt of[dt.z,dt.doorZ-dt.front*2,dt.doorZ+dt.front*3])Ft.push(new B(dt.x,tt.y,Vt));for(;At.length;){let dt=At.shift(),Vt=Ft[dt],ea=new B(Vt.x,Ht,Vt.z);if(lt(Vt,ea)&&lt(ea,vt)){Ot=dt;break}for(let Ei=1;Ei<Ft.length;Ei++)Fe[Ei]===void 0&&Vt.distanceTo(Ft[Ei])<=20&&lt(Vt,Ft[Ei])&&(Fe[Ei]=dt,At.push(Ei))}if(Ot<0)return;It=[];for(let dt=Ot;dt>=0;dt=Fe[dt])It.unshift(Ft[dt]);It.push(new B(Ft[Ot].x,Ht,Ft[Ot].z),vt,ee)}l={start:Y.position.clone(),targetStart:ce.target.clone(),end:ee.clone(),target:Oe.clone(),time:0,waypoints:It}}function K(){X.relocate(ie,Y.position),Y.position.set(ie.x,ie.y,ie.z)}function ye(ee){let Oe=Tn.find(st=>st.id===ee);Oe&&(m=ee,M.position.set(Oe.x,.55,Oe.z),M.scale.setScalar(Oe.radius*1.3),M.visible=!0,i==="street"?(Y.position.set(Oe.x,2.4,Oe.z+Oe.radius+7),Y.lookAt(Oe.x,Oe.height*.4,Oe.z),K()):Ve(new B(Oe.x+Oe.radius*2.7,Oe.height*.7+18,Oe.z+Oe.radius*4),new B(Oe.x,Oe.height*.4,Oe.z)))}function fe(ee){Ue.endRide(),Ue.society.suspend(),C.clear(),l=null,i=ee,ce.enabled=i==="orbit"||i==="tour",ie.minY=i==="street"?-2.3:-.4,ie.ignore=ie.follow=null,ie.circles[0].r=ie.reach=i==="street"?.24:.38,document.pointerLockElement===E&&document.exitPointerLock(),i==="street"?(Y.position.set(18,2.4,57),Y.lookAt(0,26,-12),K()):i!=="map"&&(K(),Ve(Dp.clone().multiplyScalar(Y.aspect<1?1.3:1),Np)),g=0,y=0,e.onMode?.(i)}let Ae=()=>{l=null,i==="tour"&&(i="orbit",e.onMode?.(i))};ce.addEventListener("start",Ae);function Te(ee,Oe,st,lt){ee.addEventListener(Oe,st,lt),N.push(()=>ee.removeEventListener(Oe,st,lt))}let me=null,Ge=!1,ze=new Gn(0,0,0,"YXZ");function Bt(ee,Oe){ze.setFromQuaternion(Y.quaternion),ze.y-=ee*.0025,ze.x=wt.clamp(ze.x-Oe*.0025,-1.35,1.35),Y.quaternion.setFromEuler(ze)}Te(E,"pointerdown",ee=>{E.focus({preventScroll:!0}),Ae(),me=[ee.clientX,ee.clientY],Ge=!1,i==="street"&&E.setPointerCapture(ee.pointerId)}),Te(E,"pointermove",ee=>{i==="street"&&(document.pointerLockElement===E||me)&&Bt(ee.movementX,ee.movementY),me&&Math.hypot(ee.clientX-me[0],ee.clientY-me[1])>5&&(Ge=!0)}),Te(E,"pointerup",ee=>{if(me&&!Ge&&i!=="street"){let Oe=E.getBoundingClientRect();$.set((ee.clientX-Oe.left)/Oe.width*2-1,1-(ee.clientY-Oe.top)/Oe.height*2),z.setFromCamera($,Y);let st=z.intersectObjects(x,!0)[0];if(st){let lt=st.object;for(;lt&&!lt.userData.district;)lt=lt.parent;lt&&e.onSelect?.(lt.userData.district)}}me=null}),Te(E,"pointercancel",()=>{me=null,C.clear()}),Te(E,"keydown",ee=>{if(ee.key==="Escape"){fe("orbit"),ee.preventDefault();return}if(i==="street"&&ee.code==="KeyE"&&!ee.repeat){Ue.interact(),ee.preventDefault();return}i==="street"&&["KeyW","KeyA","KeyS","KeyD","ArrowUp","ArrowDown","ArrowLeft","ArrowRight","ShiftLeft"].includes(ee.code)&&(C.add(ee.code),ee.preventDefault(),ee.stopPropagation())}),Te(E,"keyup",ee=>C.delete(ee.code)),Te(E,"blur",()=>C.clear()),Te(window,"blur",()=>{C.clear(),me=null}),Te(E,"webglcontextlost",ee=>{ee.preventDefault(),A=!0,C.clear(),e.onContextLost?.()}),T=new ResizeObserver(ve),T.observe(s);function Tt(ee){let Oe=(C.has("ShiftLeft")?25:11)*ee,st=Number(C.has("KeyW")||C.has("ArrowUp"))-Number(C.has("KeyS")||C.has("ArrowDown")),lt=Number(C.has("KeyD")||C.has("ArrowRight"))-Number(C.has("KeyA")||C.has("ArrowLeft"));if(!st&&!lt||Ue.walkRide(st,ee))return;let tt=Oe/Math.hypot(st,lt);Y.getWorldDirection(P),P.y=0,P.normalize();let Ht=(P.x*st-P.z*lt)*tt,It=(P.z*st+P.x*lt)*tt,Ft=(vt,Fe)=>Ue.move(vt,Fe,Y.position)&&X.clear(ie,ie,{x:vt,y:2.4+Ue.floor(vt,Fe),z:Fe,heading:0});Ft(Y.position.x+Ht,Y.position.z)&&(Y.position.x+=Ht),Ft(Y.position.x,Y.position.z+It)&&(Y.position.z+=It),Ue.isRiding()||(Y.position.y=2.4+Ue.floor(Y.position.x,Y.position.z))}let An=0,Rn=Qf(ee=>{X.begin(),An+=p?0:ee,i==="street"&&Tt(ee),Ue.update(ee,!p,i);let Oe=ie.follow;ie.follow=Ue.rideBody(),ie.ignore=ie.follow||Oe,I?.update(ee,!p),Je.update(ee,An,!p),X.propose(ie,{...Y.position,heading:0},(st,lt)=>Y.position.set(lt.x,lt.y,lt.z)),X.solve(ee),Ue.syncRide?.(),ie.follow&&Object.assign(ie,{x:Y.position.x,y:Y.position.y,z:Y.position.z}),ie.ignore=ie.follow});function Sc(ee,Oe){if(t||!n||A||i==="map")return;let st=null,lt=0,tt=null;if(i!=="street"){if(i==="tour"&&(g-=ee,g<=0)){let vt=Tn[y++%Tn.length].id;ye(vt),e.onTourFocus?.(vt),g=7}if(l){st=l,lt=l.time,l.time+=ee;let vt=Math.min(1,l.time/(l.waypoints.length===2?1.1:3.2)),Fe=vt*vt*(3-2*vt),At=Math.min(l.waypoints.length-1-1e-6,Fe*(l.waypoints.length-1)),Ot=Math.floor(At);Y.position.lerpVectors(l.waypoints[Ot],l.waypoints[Ot+1],At-Ot),ce.target.lerpVectors(l.targetStart,l.target,Fe),Y.lookAt(ce.target),tt=Y.position.clone(),vt===1&&(l=null)}else ce.update()}Rn(Math.min(.1,Math.max(0,ee)))||(X.begin(),X.propose(ie,{...Y.position,heading:0},(vt,Fe)=>Y.position.set(Fe.x,Fe.y,Fe.z)),X.solve(0)),st&&(Y.position.distanceToSquared(tt)>1e-6?(st.wait=(st.wait||0)+ee,st.time=lt,l=st.wait<6?st:null):st.wait=0),Y.getWorldDirection(P),e.onListener?.(Y.position.x,Y.position.y,Y.position.z,P.x,P.z);let Ht=!!e.busy?.();q.update(ee,Oe,!p)&&(U.shadowMap.needsUpdate=!0),he.intensity=p?0:Ht?180+Math.sin(Oe*2)*35:0,Pe.setBusy(Ht),Pe.setCinematic(i==="tour"),Pe.update(ee,Oe,Y,!p),be.update(ee,!p);let It=q.mood(),Ft=[Ht,It.day.toFixed(1),It.evening.toFixed(1),It.weather].join();if(Ft!==it&&(it=Ft,e.onMood?.({busy:Ht,day:It.day,evening:It.evening,weather:It.weather})),Xe.update(ee,Y,!p),U.info.reset(),H.render(),c++,r==="auto"&&ee>0&&ee<.2&&Oe-d>12&&(h+=ee,u++,u>=180)){let vt=h/u,Fe=["low","medium","high"],At=Fe.indexOf(o),Ot=vt>.028&&At>0?Fe[At-1]:vt<.017&&At<2?Fe[At+1]:o;u=0,h=0,Ot!==o&&(o=Ot,d=Oe,Ee(),Me())}}function wc(ee,Oe){I?.setData(ee,Oe),Tn.forEach((st,lt)=>{let tt=ee.find(Ht=>Ht.id===st.id);O.setColorAt(lt,new xe(!tt||tt.stale?8096667:tt.state==="error"?16742504:tt.state==="running"?7730385:8175587))}),O.instanceColor.needsUpdate=!0}function Vi(){t||(t=!0,a++,v.abort(),D.forEach(ee=>ee.abort()),C.clear(),document.pointerLockElement===E&&document.exitPointerLock(),e.signal?.removeEventListener("abort",Vi),N.forEach(ee=>ee()),T?.disconnect(),ce.dispose(),I?.dispose(),Ue?.dispose(),q.dispose(),Xe.dispose(),Je.dispose(),X.dispose(),Pe.dispose(),be.dispose(),Ce(J),Ce(te),O.dispose(),L.forEach(ee=>ee.dispose()),V.forEach(ee=>ee.dispose()),H.passes.forEach(ee=>ee.dispose?.()),H.dispose(),oe.dispose(),ae.shadow.dispose(),U.dispose(),U.forceContextLoss(),E.remove(),w.clear(),R.clear())}I=tp(G,Tn,{traffic:X,society:Ue.society,robotURL:e.resourceURL("/3d/system-world/white-robot.glb"),signal:e.signal,onError:e.onRobotError,active:()=>n&&!A&&i!=="map"&&!e.replaying?.(),obstacles:By}),Xe.setReducedMotion(!!p);try{Ee(),await Me()}catch(ee){throw Vi(),ee}return{districts:Tn,canvas:E,update:Sc,focus(ee){Ae(),ye(ee)},setMode:fe,setQuality:Be,setData:wc,dispose:Vi,interact(){Ue.interact()},socialAction:Ue.socialAction,visit(ee){fe("street"),Ue.visit(ee),K()},enter(ee){fe("street"),Ue.destination(ee),K()},setEnvironment(ee){q.set(ee),U.shadowMap.needsUpdate=!0},setVisible(ee){n=ee,ee||(Rn(0),Ue.suspend(),I?.update(0,!1),C.clear(),me=null,document.pointerLockElement===E&&document.exitPointerLock())},setReducedMotion(ee){p=ee,Xe.setReducedMotion(!!ee),ee&&(l=null,Ue.endRide(),Ue.society.suspend()),ee&&i==="tour"&&fe("orbit")},setHologram(ee,Oe){Xe.setTexts(ee,Oe),Ue.setMemory(ee)},setWorld(ee,Oe){Ue.setWorld(ee,Oe)},moveKey(ee,Oe){Oe?C.add(ee):C.delete(ee)},lockPointer(){if(i==="street")return E.requestPointerLock()},project(ee){let Oe=Tn.find(st=>st.id===ee);return Oe?(P.set(Oe.x,Oe.height+4,Oe.z).project(Y),{x:(P.x+1)*b/2,y:(1-P.y)*S/2,visible:P.z<1&&P.z>-1}):null},stats(){return{flying:!!l,traffic:X.stats(),experience:Ue.stats(),weather:q.stats(),life:I?.stats(),hologram:Xe.stats(),atmosphere:Pe.stats(),surfaces:be.stats(),drones:Je.stats(),frames:c,tier:o,mode:i,focusedDistrict:m,loadedBytes:f+Ue.stats().bytes+be.stats().bytes,cachedModels:w.size,calls:U.info.render.calls,triangles:U.info.render.triangles,geometries:U.info.memory.geometries,position:Y.position.toArray(),renderer:U.getContext().getParameter(U.getContext().getExtension("WEBGL_debug_renderer_info")?.UNMASKED_RENDERER_WEBGL||U.getContext().RENDERER),disposed:t}}}}export{e1 as createCity,Oy as createCityAmbience,Tn as districts,By as obstacles,Gs as placements};
/*! Bundled license information:

three/build/three.core.js:
three/build/three.module.js:
  (**
   * @license
   * Copyright 2010-2026 Three.js Authors
   * SPDX-License-Identifier: MIT
   *)
*/
