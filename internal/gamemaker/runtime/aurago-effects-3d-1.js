import*as h from"./three-0.185.1.module.min.js";import{BackSide as at,BoxGeometry as st,Mesh as nt,ShaderMaterial as lt,UniformsUtils as ut,Vector3 as He}from"./three-0.185.1.module.min.js";var Ae=class x extends nt{constructor(){let e=x.SkyShader,a=new lt({name:e.name,uniforms:ut.clone(e.uniforms),vertexShader:e.vertexShader,fragmentShader:e.fragmentShader,side:at,depthWrite:!1});super(new st(1,1,1),a),this.isSky=!0}};Ae.SkyShader={name:"SkyShader",uniforms:{turbidity:{value:2},rayleigh:{value:1},mieCoefficient:{value:.005},mieDirectionalG:{value:.8},sunPosition:{value:new He},up:{value:new He(0,1,0)},cloudScale:{value:2e-4},cloudSpeed:{value:1e-4},cloudCoverage:{value:.4},cloudDensity:{value:.4},cloudElevation:{value:.5},showSunDisc:{value:1},time:{value:0}},vertexShader:`
		uniform vec3 sunPosition;
		uniform float rayleigh;
		uniform float turbidity;
		uniform float mieCoefficient;
		uniform vec3 up;

		varying vec3 vWorldPosition;
		varying vec3 vSunDirection;
		varying float vSunfade;
		varying vec3 vBetaR;
		varying vec3 vBetaM;
		varying float vSunE;

		// constants for atmospheric scattering
		const float e = 2.71828182845904523536028747135266249775724709369995957;
		const float pi = 3.141592653589793238462643383279502884197169;

		// wavelength of used primaries, according to preetham
		const vec3 lambda = vec3( 680E-9, 550E-9, 450E-9 );
		// this pre-calculation replaces older TotalRayleigh(vec3 lambda) function:
		// (8.0 * pow(pi, 3.0) * pow(pow(n, 2.0) - 1.0, 2.0) * (6.0 + 3.0 * pn)) / (3.0 * N * pow(lambda, vec3(4.0)) * (6.0 - 7.0 * pn))
		const vec3 totalRayleigh = vec3( 5.804542996261093E-6, 1.3562911419845635E-5, 3.0265902468824876E-5 );

		// mie stuff
		// K coefficient for the primaries
		const float v = 4.0;
		const vec3 K = vec3( 0.686, 0.678, 0.666 );
		// MieConst = pi * pow( ( 2.0 * pi ) / lambda, vec3( v - 2.0 ) ) * K
		const vec3 MieConst = vec3( 1.8399918514433978E14, 2.7798023919660528E14, 4.0790479543861094E14 );

		// earth shadow hack
		// cutoffAngle = pi / 1.95;
		const float cutoffAngle = 1.6110731556870734;
		const float steepness = 1.5;
		const float EE = 1000.0;

		float sunIntensity( float zenithAngleCos ) {
			zenithAngleCos = clamp( zenithAngleCos, -1.0, 1.0 );
			return EE * max( 0.0, 1.0 - pow( e, -( ( cutoffAngle - acos( zenithAngleCos ) ) / steepness ) ) );
		}

		vec3 totalMie( float T ) {
			float c = ( 0.2 * T ) * 10E-18;
			return 0.434 * c * MieConst;
		}

		void main() {

			vec4 worldPosition = modelMatrix * vec4( position, 1.0 );
			vWorldPosition = worldPosition.xyz;

			gl_Position = projectionMatrix * modelViewMatrix * vec4( position, 1.0 );
			gl_Position.z = gl_Position.w; // set z to camera.far

			vSunDirection = normalize( sunPosition );

			vSunE = sunIntensity( dot( vSunDirection, up ) );

			vSunfade = 1.0 - clamp( 1.0 - exp( ( sunPosition.y / 450000.0 ) ), 0.0, 1.0 );

			float rayleighCoefficient = rayleigh - ( 1.0 * ( 1.0 - vSunfade ) );

			// extinction (absorption + out scattering)
			// rayleigh coefficients
			vBetaR = totalRayleigh * rayleighCoefficient;

			// mie coefficients
			vBetaM = totalMie( turbidity ) * mieCoefficient;

		}`,fragmentShader:`
		varying vec3 vWorldPosition;
		varying vec3 vSunDirection;
		varying vec3 vBetaR;
		varying vec3 vBetaM;
		varying float vSunE;

		uniform float mieDirectionalG;
		uniform vec3 up;
		uniform float cloudScale;
		uniform float cloudSpeed;
		uniform float cloudCoverage;
		uniform float cloudDensity;
		uniform float cloudElevation;
		uniform float showSunDisc;
		uniform float time;

		// Cloud noise functions
		float hash( vec2 p ) {
			return fract( sin( dot( p, vec2( 127.1, 311.7 ) ) ) * 43758.5453123 );
		}

		float noise( vec2 p ) {
			vec2 i = floor( p );
			vec2 f = fract( p );
			f = f * f * ( 3.0 - 2.0 * f );
			float a = hash( i );
			float b = hash( i + vec2( 1.0, 0.0 ) );
			float c = hash( i + vec2( 0.0, 1.0 ) );
			float d = hash( i + vec2( 1.0, 1.0 ) );
			return mix( mix( a, b, f.x ), mix( c, d, f.x ), f.y );
		}

		float fbm( vec2 p ) {
			float value = 0.0;
			float amplitude = 0.5;
			for ( int i = 0; i < 5; i ++ ) {
				value += amplitude * noise( p );
				p *= 2.0;
				amplitude *= 0.5;
			}
			return value;
		}

		// constants for atmospheric scattering
		const float pi = 3.141592653589793238462643383279502884197169;

		const float n = 1.0003; // refractive index of air
		const float N = 2.545E25; // number of molecules per unit volume for air at 288.15K and 1013mb (sea level -45 celsius)

		// optical length at zenith for molecules
		const float rayleighZenithLength = 8.4E3;
		const float mieZenithLength = 1.25E3;
		// 66 arc seconds -> degrees, and the cosine of that
		const float sunAngularDiameterCos = 0.999956676946448443553574619906976478926848692873900859324;

		// 3.0 / ( 16.0 * pi )
		const float THREE_OVER_SIXTEENPI = 0.05968310365946075;
		// 1.0 / ( 4.0 * pi )
		const float ONE_OVER_FOURPI = 0.07957747154594767;

		float rayleighPhase( float cosTheta ) {
			return THREE_OVER_SIXTEENPI * ( 1.0 + pow( cosTheta, 2.0 ) );
		}

		float hgPhase( float cosTheta, float g ) {
			float g2 = pow( g, 2.0 );
			float inverse = 1.0 / pow( 1.0 - 2.0 * g * cosTheta + g2, 1.5 );
			return ONE_OVER_FOURPI * ( ( 1.0 - g2 ) * inverse );
		}

		void main() {

			vec3 direction = normalize( vWorldPosition - cameraPosition );

			// optical length
			// cutoff angle at 90 to avoid singularity in next formula.
			float zenithAngle = acos( max( 0.0, dot( up, direction ) ) );
			float inverse = 1.0 / ( cos( zenithAngle ) + 0.15 * pow( 93.885 - ( ( zenithAngle * 180.0 ) / pi ), -1.253 ) );
			float sR = rayleighZenithLength * inverse;
			float sM = mieZenithLength * inverse;

			// combined extinction factor
			vec3 Fex = exp( -( vBetaR * sR + vBetaM * sM ) );

			// in scattering
			float cosTheta = dot( direction, vSunDirection );

			float rPhase = rayleighPhase( cosTheta * 0.5 + 0.5 );
			vec3 betaRTheta = vBetaR * rPhase;

			float mPhase = hgPhase( cosTheta, mieDirectionalG );
			vec3 betaMTheta = vBetaM * mPhase;

			vec3 Lin = pow( vSunE * ( ( betaRTheta + betaMTheta ) / ( vBetaR + vBetaM ) ) * ( 1.0 - Fex ), vec3( 1.5 ) );
			Lin *= mix( vec3( 1.0 ), pow( vSunE * ( ( betaRTheta + betaMTheta ) / ( vBetaR + vBetaM ) ) * Fex, vec3( 1.0 / 2.0 ) ), clamp( pow( 1.0 - dot( up, vSunDirection ), 5.0 ), 0.0, 1.0 ) );

			// nightsky
			float theta = acos( direction.y ); // elevation --> y-axis, [-pi/2, pi/2]
			float phi = atan( direction.z, direction.x ); // azimuth --> x-axis [-pi/2, pi/2]
			vec2 uv = vec2( phi, theta ) / vec2( 2.0 * pi, pi ) + vec2( 0.5, 0.0 );
			vec3 L0 = vec3( 0.1 ) * Fex;

			// composition + solar disc
			float sundisc = smoothstep( sunAngularDiameterCos, sunAngularDiameterCos + 0.00002, cosTheta ) * showSunDisc;
			L0 += ( vSunE * 19000.0 * Fex ) * sundisc;

			vec3 texColor = ( Lin + L0 ) * 0.04 + vec3( 0.0, 0.0003, 0.00075 );

			// Clouds
			if ( direction.y > 0.0 && cloudCoverage > 0.0 ) {

				// Project to cloud plane (higher elevation = clouds appear lower/closer)
				float elevation = mix( 1.0, 0.1, cloudElevation );
				vec2 cloudUV = direction.xz / ( direction.y * elevation );
				cloudUV *= cloudScale;
				cloudUV += time * cloudSpeed;

				// Multi-octave noise for fluffy clouds
				float cloudNoise = fbm( cloudUV * 1000.0 );
				cloudNoise += 0.5 * fbm( cloudUV * 2000.0 + 3.7 );
				cloudNoise = cloudNoise * 0.5 + 0.5;

				// Apply coverage threshold
				float cloudMask = smoothstep( 1.0 - cloudCoverage, 1.0 - cloudCoverage + 0.3, cloudNoise );

				// Fade clouds near horizon (adjusted by elevation)
				float horizonFade = smoothstep( 0.0, 0.1 + 0.2 * cloudElevation, direction.y );
				cloudMask *= horizonFade;

				// Cloud lighting based on sun position
				float sunInfluence = dot( direction, vSunDirection ) * 0.5 + 0.5;
				float daylight = max( 0.0, vSunDirection.y * 2.0 );

				// Base cloud color affected by atmosphere
				vec3 atmosphereColor = Lin * 0.04;
				vec3 cloudColor = mix( vec3( 0.3 ), vec3( 1.0 ), daylight );
				cloudColor = mix( cloudColor, atmosphereColor + vec3( 1.0 ), sunInfluence * 0.5 );
				cloudColor *= vSunE * 0.00002;

				// Blend clouds with sky
				texColor = mix( texColor, cloudColor, cloudMask * cloudDensity );

			}

			gl_FragColor = vec4( texColor, 1.0 );

			#include <tonemapping_fragment>
			#include <colorspace_fragment>

		}`};import{Color as De,FrontSide as ct,HalfFloatType as ft,Matrix4 as Ve,Mesh as ht,PerspectiveCamera as mt,Plane as dt,ShaderMaterial as pt,UniformsLib as qe,UniformsUtils as Xe,Vector3 as ve,Vector4 as Ke,WebGLRenderTarget as gt}from"./three-0.185.1.module.min.js";var Fe=class extends ht{constructor(e,a={}){super(e),this.isWater=!0;let u=this,v=a.textureWidth!==void 0?a.textureWidth:512,l=a.textureHeight!==void 0?a.textureHeight:512,d=a.clipBias!==void 0?a.clipBias:0,E=a.alpha!==void 0?a.alpha:1,m=a.time!==void 0?a.time:0,A=a.waterNormals!==void 0?a.waterNormals:null,P=a.sunDirection!==void 0?a.sunDirection:new ve(.70707,.70707,0),H=new De(a.sunColor!==void 0?a.sunColor:16777215),c=new De(a.waterColor!==void 0?a.waterColor:8355711),W=a.eye!==void 0?a.eye:new ve(0,0,0),I=a.distortionScale!==void 0?a.distortionScale:20,se=a.side!==void 0?a.side:ct,q=a.fog!==void 0?a.fog:!1,$=new dt,C=new ve,N=new ve,Q=new ve,M=new Ve,Z=new ve(0,0,-1),L=new Ke,oe=new ve,ee=new ve,X=new Ke,ne=new Ve,F=new mt,pe=new gt(v,l,{type:ft}),fe={name:"MirrorShader",uniforms:Xe.merge([qe.fog,qe.lights,{normalSampler:{value:null},mirrorSampler:{value:null},alpha:{value:1},time:{value:0},size:{value:1},distortionScale:{value:20},textureMatrix:{value:new Ve},sunColor:{value:new De(8355711)},sunDirection:{value:new ve(.70707,.70707,0)},eye:{value:new ve},waterColor:{value:new De(5592405)}}]),vertexShader:`
				uniform mat4 textureMatrix;
				uniform float time;

				varying vec4 mirrorCoord;
				varying vec4 worldPosition;

				#include <common>
				#include <fog_pars_vertex>
				#include <shadowmap_pars_vertex>
				#include <logdepthbuf_pars_vertex>

				void main() {
					mirrorCoord = modelMatrix * vec4( position, 1.0 );
					worldPosition = mirrorCoord.xyzw;
					mirrorCoord = textureMatrix * mirrorCoord;
					vec4 mvPosition =  modelViewMatrix * vec4( position, 1.0 );
					gl_Position = projectionMatrix * mvPosition;

				#include <beginnormal_vertex>
				#include <defaultnormal_vertex>
				#include <logdepthbuf_vertex>
				#include <fog_vertex>
				#include <shadowmap_vertex>
			}`,fragmentShader:`
				uniform sampler2D mirrorSampler;
				uniform float alpha;
				uniform float time;
				uniform float size;
				uniform float distortionScale;
				uniform sampler2D normalSampler;
				uniform vec3 sunColor;
				uniform vec3 sunDirection;
				uniform vec3 eye;
				uniform vec3 waterColor;

				varying vec4 mirrorCoord;
				varying vec4 worldPosition;

				vec4 getNoise( vec2 uv ) {
					vec2 uv0 = ( uv / 103.0 ) + vec2(time / 17.0, time / 29.0);
					vec2 uv1 = uv / 107.0-vec2( time / -19.0, time / 31.0 );
					vec2 uv2 = uv / vec2( 8907.0, 9803.0 ) + vec2( time / 101.0, time / 97.0 );
					vec2 uv3 = uv / vec2( 1091.0, 1027.0 ) - vec2( time / 109.0, time / -113.0 );
					vec4 noise = texture2D( normalSampler, uv0 ) +
						texture2D( normalSampler, uv1 ) +
						texture2D( normalSampler, uv2 ) +
						texture2D( normalSampler, uv3 );
					return noise * 0.5 - 1.0;
				}

				void sunLight( const vec3 surfaceNormal, const vec3 eyeDirection, float shiny, float spec, float diffuse, inout vec3 diffuseColor, inout vec3 specularColor ) {
					vec3 reflection = normalize( reflect( -sunDirection, surfaceNormal ) );
					float direction = max( 0.0, dot( eyeDirection, reflection ) );
					specularColor += pow( direction, shiny ) * sunColor * spec;
					diffuseColor += max( dot( sunDirection, surfaceNormal ), 0.0 ) * sunColor * diffuse;
				}

				#include <common>
				#include <packing>
				#include <bsdfs>
				#include <fog_pars_fragment>
				#include <logdepthbuf_pars_fragment>
				#include <lights_pars_begin>
				#include <shadowmap_pars_fragment>
				#include <shadowmask_pars_fragment>

				void main() {

					#include <logdepthbuf_fragment>
					vec4 noise = getNoise( worldPosition.xz * size );
					vec3 surfaceNormal = normalize( noise.xzy * vec3( 1.5, 1.0, 1.5 ) );

					vec3 diffuseLight = vec3(0.0);
					vec3 specularLight = vec3(0.0);

					vec3 worldToEye = eye-worldPosition.xyz;
					vec3 eyeDirection = normalize( worldToEye );
					sunLight( surfaceNormal, eyeDirection, 100.0, 2.0, 0.5, diffuseLight, specularLight );

					float distance = length(worldToEye);

					vec2 distortion = surfaceNormal.xz * ( 0.001 + 1.0 / distance ) * distortionScale;
					vec3 reflectionSample = vec3( texture2D( mirrorSampler, mirrorCoord.xy / mirrorCoord.w + distortion ) );

					float theta = max( dot( eyeDirection, surfaceNormal ), 0.0 );
					float rf0 = 0.02;
					float reflectance = rf0 + ( 1.0 - rf0 ) * pow( ( 1.0 - theta ), 5.0 );
					vec3 scatter = max( 0.0, dot( surfaceNormal, eyeDirection ) ) * waterColor;
					vec3 albedo = mix( ( sunColor * diffuseLight * 0.3 + scatter ) * getShadowMask(), reflectionSample + specularLight, reflectance );
					vec3 outgoingLight = albedo;
					gl_FragColor = vec4( outgoingLight, alpha );

					#include <tonemapping_fragment>
					#include <colorspace_fragment>
					#include <fog_fragment>
				}`},Y=new pt({name:fe.name,uniforms:Xe.clone(fe.uniforms),vertexShader:fe.vertexShader,fragmentShader:fe.fragmentShader,lights:!0,side:se,fog:q});Y.uniforms.mirrorSampler.value=pe.texture,this.dispose=()=>pe.dispose(),Y.uniforms.textureMatrix.value=ne,Y.uniforms.alpha.value=E,Y.uniforms.time.value=m,Y.uniforms.normalSampler.value=A,Y.uniforms.sunColor.value=H,Y.uniforms.waterColor.value=c,Y.uniforms.sunDirection.value=P,Y.uniforms.distortionScale.value=I,Y.uniforms.eye.value=W,u.material=Y,u.onBeforeRender=function(J,G,ie){if(N.setFromMatrixPosition(u.matrixWorld),Q.setFromMatrixPosition(ie.matrixWorld),M.extractRotation(u.matrixWorld),C.set(0,0,1),C.applyMatrix4(M),oe.subVectors(N,Q),oe.dot(C)>0)return;oe.reflect(C).negate(),oe.add(N),M.extractRotation(ie.matrixWorld),Z.set(0,0,-1),Z.applyMatrix4(M),Z.add(Q),ee.subVectors(N,Z),ee.reflect(C).negate(),ee.add(N),F.position.copy(oe),F.up.set(0,1,0),F.up.applyMatrix4(M),F.up.reflect(C),F.lookAt(ee),F.far=ie.far,F.updateMatrixWorld(),F.projectionMatrix.copy(ie.projectionMatrix),ne.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),ne.multiply(F.projectionMatrix),ne.multiply(F.matrixWorldInverse),$.setFromNormalAndCoplanarPoint(C,N),$.applyMatrix4(F.matrixWorldInverse),L.set($.normal.x,$.normal.y,$.normal.z,$.constant);let B=F.projectionMatrix;X.x=(Math.sign(L.x)+B.elements[8])/B.elements[0],X.y=(Math.sign(L.y)+B.elements[9])/B.elements[5],X.z=-1,X.w=(1+B.elements[10])/B.elements[14],L.multiplyScalar(2/L.dot(X)),B.elements[2]=L.x,B.elements[6]=L.y,B.elements[10]=L.z+1-d,B.elements[14]=L.w,W.setFromMatrixPosition(ie.matrixWorld);let i=J.getRenderTarget(),g=J.xr.enabled,p=J.shadowMap.autoUpdate;u.visible=!1,J.xr.enabled=!1,J.shadowMap.autoUpdate=!1,J.setRenderTarget(pe),J.state.buffers.depth.setMask(!0),J.autoClear===!1&&J.clear(),J.render(G,F),u.visible=!0,J.xr.enabled=g,J.shadowMap.autoUpdate=p,J.setRenderTarget(i);let S=ie.viewport;S!==void 0&&J.state.viewport(S)}}};import{HalfFloatType as Tt,NoBlending as St,Timer as Ct,Vector2 as $e,WebGLRenderTarget as _t}from"./three-0.185.1.module.min.js";var Se={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};import{ShaderMaterial as Ye,UniformsUtils as Mt}from"./three-0.185.1.module.min.js";import{BufferGeometry as vt,Float32BufferAttribute as Ze,OrthographicCamera as xt,Mesh as yt}from"./three-0.185.1.module.min.js";var me=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},wt=new xt(-1,1,1,-1,0,1),Ie=class extends vt{constructor(){super(),this.setAttribute("position",new Ze([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new Ze([0,2,0,0,2,0],2))}},bt=new Ie,we=class{constructor(e){this._mesh=new yt(bt,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,wt)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Ce=class extends me{constructor(e,a="tDiffuse"){super(),this.textureID=a,this.uniforms=null,this.material=null,e instanceof Ye?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=Mt.clone(e.uniforms),this.material=new Ye({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new we(this.material)}render(e,a,u){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=u.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(a),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var ke=class extends me{constructor(e,a){super(),this.scene=e,this.camera=a,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,a,u){let v=e.getContext(),l=e.state;l.buffers.color.setMask(!1),l.buffers.depth.setMask(!1),l.buffers.color.setLocked(!0),l.buffers.depth.setLocked(!0);let d,E;this.inverse?(d=0,E=1):(d=1,E=0),l.buffers.stencil.setTest(!0),l.buffers.stencil.setOp(v.REPLACE,v.REPLACE,v.REPLACE),l.buffers.stencil.setFunc(v.ALWAYS,d,4294967295),l.buffers.stencil.setClear(E),l.buffers.stencil.setLocked(!0),e.setRenderTarget(u),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(a),this.clear&&e.clear(),e.render(this.scene,this.camera),l.buffers.color.setLocked(!1),l.buffers.depth.setLocked(!1),l.buffers.color.setMask(!0),l.buffers.depth.setMask(!0),l.buffers.stencil.setLocked(!1),l.buffers.stencil.setFunc(v.EQUAL,1,4294967295),l.buffers.stencil.setOp(v.KEEP,v.KEEP,v.KEEP),l.buffers.stencil.setLocked(!0)}},Be=class extends me{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Ue=class{constructor(e,a){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),a===void 0){let u=e.getSize(new $e);this._width=u.width,this._height=u.height,a=new _t(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Tt}),a.texture.name="EffectComposer.rt1"}else this._width=a.width,this._height=a.height;this.renderTarget1=a,this.renderTarget2=a.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Ce(Se),this.copyPass.material.blending=St,this.timer=new Ct}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,a){this.passes.splice(a,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let a=this.passes.indexOf(e);a!==-1&&this.passes.splice(a,1)}isLastEnabledPass(e){for(let a=e+1;a<this.passes.length;a++)if(this.passes[a].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let a=this.renderer.getRenderTarget(),u=!1;for(let v=0,l=this.passes.length;v<l;v++){let d=this.passes[v];if(d.enabled!==!1){if(d.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(v),d.render(this.renderer,this.writeBuffer,this.readBuffer,e,u),d.needsSwap){if(u){let E=this.renderer.getContext(),m=this.renderer.state.buffers.stencil;m.setFunc(E.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),m.setFunc(E.EQUAL,1,4294967295)}this.swapBuffers()}ke!==void 0&&(d instanceof ke?u=!0:d instanceof Be&&(u=!1))}}this.renderer.setRenderTarget(a)}reset(e){if(e===void 0){let a=this.renderer.getSize(new $e);this._pixelRatio=this.renderer.getPixelRatio(),this._width=a.width,this._height=a.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,a){this._width=e,this._height=a;let u=this._width*this._pixelRatio,v=this._height*this._pixelRatio;this.renderTarget1.setSize(u,v),this.renderTarget2.setSize(u,v);for(let l=0;l<this.passes.length;l++)this.passes[l].setSize(u,v)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};import{Color as Pt}from"./three-0.185.1.module.min.js";var Ne=class extends me{constructor(e,a,u=null,v=null,l=null){super(),this.scene=e,this.camera=a,this.overrideMaterial=u,this.clearColor=v,this.clearAlpha=l,this.clear=!0,this.clearDepth=!1,this.needsSwap=!1,this.isRenderPass=!0,this._oldClearColor=new Pt}render(e,a,u){let v=e.autoClear;e.autoClear=!1;let l,d;this.overrideMaterial!==null&&(d=this.scene.overrideMaterial,this.scene.overrideMaterial=this.overrideMaterial),this.clearColor!==null&&(e.getClearColor(this._oldClearColor),e.setClearColor(this.clearColor,e.getClearAlpha())),this.clearAlpha!==null&&(l=e.getClearAlpha(),e.setClearAlpha(this.clearAlpha)),this.clearDepth==!0&&e.clearDepth(),e.setRenderTarget(this.renderToScreen?null:u),this.clear===!0&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),e.render(this.scene,this.camera),this.clearColor!==null&&e.setClearColor(this._oldClearColor),this.clearAlpha!==null&&e.setClearAlpha(l),this.overrideMaterial!==null&&(this.scene.overrideMaterial=d),e.autoClear=v}};import{AdditiveBlending as At,Color as et,HalfFloatType as Ge,MeshBasicMaterial as kt,ShaderMaterial as Le,UniformsUtils as tt,Vector2 as be,Vector3 as ze,WebGLRenderTarget as je}from"./three-0.185.1.module.min.js";import{Color as Et}from"./three-0.185.1.module.min.js";var Je={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Et(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var _e=class x extends me{constructor(e,a=1,u,v){super(),this.strength=a,this.radius=u,this.threshold=v,this.resolution=e!==void 0?new be(e.x,e.y):new be(256,256),this.clearColor=new et(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let l=Math.round(this.resolution.x/2),d=Math.round(this.resolution.y/2);this.renderTargetBright=new je(l,d,{type:Ge}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let P=0;P<this.nMips;P++){let H=new je(l,d,{type:Ge});H.texture.name="UnrealBloomPass.h"+P,H.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(H);let c=new je(l,d,{type:Ge});c.texture.name="UnrealBloomPass.v"+P,c.texture.generateMipmaps=!1,this.renderTargetsVertical.push(c),l=Math.round(l/2),d=Math.round(d/2)}let E=Je;this.highPassUniforms=tt.clone(E.uniforms),this.highPassUniforms.luminosityThreshold.value=v,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new Le({uniforms:this.highPassUniforms,vertexShader:E.vertexShader,fragmentShader:E.fragmentShader}),this.separableBlurMaterials=[];let m=[6,10,14,18,22];l=Math.round(this.resolution.x/2),d=Math.round(this.resolution.y/2);for(let P=0;P<this.nMips;P++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(m[P])),this.separableBlurMaterials[P].uniforms.invSize.value=new be(1/l,1/d),l=Math.round(l/2),d=Math.round(d/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=a,this.compositeMaterial.uniforms.bloomRadius.value=.1;let A=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=A,this.bloomTintColors=[new ze(1,1,1),new ze(1,1,1),new ze(1,1,1),new ze(1,1,1),new ze(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=tt.clone(Se.uniforms),this.blendMaterial=new Le({uniforms:this.copyUniforms,vertexShader:Se.vertexShader,fragmentShader:Se.fragmentShader,premultipliedAlpha:!0,blending:At,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new et,this._oldClearAlpha=1,this._basic=new kt,this._fsQuad=new we(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,a){let u=Math.round(e/2),v=Math.round(a/2);this.renderTargetBright.setSize(u,v);for(let l=0;l<this.nMips;l++)this.renderTargetsHorizontal[l].setSize(u,v),this.renderTargetsVertical[l].setSize(u,v),this.separableBlurMaterials[l].uniforms.invSize.value=new be(1/u,1/v),u=Math.round(u/2),v=Math.round(v/2)}render(e,a,u,v,l){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let d=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),l&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=u.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=u.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let E=this.renderTargetBright;for(let m=0;m<this.nMips;m++)this._fsQuad.material=this.separableBlurMaterials[m],this.separableBlurMaterials[m].uniforms.colorTexture.value=E.texture,this.separableBlurMaterials[m].uniforms.direction.value=x.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[m]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[m].uniforms.colorTexture.value=this.renderTargetsHorizontal[m].texture,this.separableBlurMaterials[m].uniforms.direction.value=x.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[m]),e.clear(),this._fsQuad.render(e),E=this.renderTargetsVertical[m];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,l&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(u),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=d}_getSeparableBlurMaterial(e){let a=[],u=e/3;for(let v=0;v<e;v++)a.push(.39894*Math.exp(-.5*v*v/(u*u))/u);return new Le({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new be(.5,.5)},direction:{value:new be(.5,.5)},gaussianCoefficients:{value:a}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new Le({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};_e.BlurDirectionX=new be(1,0);_e.BlurDirectionY=new be(0,1);import{ColorManagement as zt,RawShaderMaterial as Rt,UniformsUtils as Dt,LinearToneMapping as Ft,ReinhardToneMapping as Bt,CineonToneMapping as Ut,AgXToneMapping as Nt,ACESFilmicToneMapping as Lt,NeutralToneMapping as Ot,CustomToneMapping as Wt,SRGBTransfer as Vt}from"./three-0.185.1.module.min.js";var Re={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var Oe=class extends me{constructor(){super(),this.isOutputPass=!0,this.uniforms=Dt.clone(Re.uniforms),this.material=new Rt({name:Re.name,uniforms:this.uniforms,vertexShader:Re.vertexShader,fragmentShader:Re.fragmentShader}),this._fsQuad=new we(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,a,u){this.uniforms.tDiffuse.value=u.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},zt.getTransfer(this._outputColorSpace)===Vt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Ft?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Bt?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Ut?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Lt?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Nt?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Ot?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Wt&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(a),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Qe=new WeakMap;function it({sounds:x=[],base:e,root:a,report:u=console.warn,dimension:v="3d"}){let l=new Map(x.map(t=>[t.id,t])),d=new Map,E=new Map,m=new Set,A=new Map,P=new Map,H=new AbortController,c,W,I,se,q,$,C=!1,N=!1,Q=!1,M=.55,Z=Qe.get(a);Z&&(Q=Z.muted,M=Z.volume);let L={},oe=[0,0,0],ee=new Set,X=new Set,ne="outside",F=0;function pe(){if(!(c||C)){c=new AudioContext({sampleRate:48e3}),W=c.createGain(),I=c.createDynamicsCompressor(),I.threshold.value=-12,I.knee.value=12,I.ratio.value=8,I.attack.value=.003,I.release.value=.2,W.connect(I).connect(c.destination);for(let t of["effects","ambience","music"])L[t]=c.createGain(),L[t].gain.value=t==="ambience"?.45:1,L[t].connect(W);q=c.createConvolver(),se=c.createGain(),$=c.createBiquadFilter(),$.type="lowpass",$.frequency.value=8e3,q.connect($).connect(se).connect(W),Y(ne),fe()}}function fe(){W&&W.gain.setTargetAtTime(Q||N?0:M*M,c.currentTime,.03)}function Y(t){if(!["outside","small-room","hall"].includes(t))throw Error("audio: unknown room");if(ne=t,!c)return;se.gain.value=t==="outside"?0:t==="hall"?.2:.1;let y=Math.floor(c.sampleRate*(t==="hall"?1.7:.35)),w=c.createBuffer(2,y,c.sampleRate),O=1973;for(let U=0;U<2;U++){let te=w.getChannelData(U);for(let T=0;T<y;T++)O=Math.imul(O,1664525)+1013904223>>>0,te[T]=(O/2147483648-1)*Math.pow(1-T/y,3)}q.buffer=w}async function J(t){if(d.has(t))return d.get(t);if(E.has(t))return E.get(t);let y=l.get(t);if(!y)throw Error("audio: unknown imported sound "+t);let w=(async()=>{let O=y.files?.[0];if(!O||!O.file.startsWith("sounds/")||!/^sounds\/[a-z0-9-]+\.wav$/.test(O.file))throw Error("audio: invalid sample path");let U=new URL(e,document.baseURI),te=new URL(O.file,U);if(te.origin!==new URL(document.baseURI).origin||!te.pathname.startsWith(U.pathname))throw Error("audio: sample outside game");let T=await fetch(te,{signal:H.signal});if(!T.ok)throw Error("audio: "+t+" HTTP "+T.status);let le=await T.arrayBuffer();if(le.byteLength!==O.bytes)throw Error("audio: size mismatch "+t);if(crypto.subtle&&[...new Uint8Array(await crypto.subtle.digest("SHA-256",le))].map(V=>V.toString(16).padStart(2,"0")).join("")!==O.sha256)throw Error("audio: checksum mismatch "+t);let ae=await c.decodeAudioData(le);return C?null:(d.set(t,ae),ae)})();E.set(t,w);try{return await w}finally{E.delete(t)}}function G(t){if(m.has(t)){m.delete(t);try{t.source.stop()}catch{}t.source.disconnect(),t.gain.disconnect(),t.pan?.disconnect()}}function ie(t,y=.18){if(!(!m.has(t)||t.fading)){t.fading=!0,t.gain.gain.setTargetAtTime(0,c.currentTime,y/3);try{t.source.stop(c.currentTime+y)}catch{}}}function B(t){!t.position||!t.pan?.pan||t.fading||(t.pan.pan.value=Math.max(-1,Math.min(1,(t.position[0]-oe[0])/450)),t.gain.gain.setTargetAtTime(t.volume/(1+Math.hypot(t.position[0]-oe[0],t.position[1]-oe[1])/500),c.currentTime,.03))}function i(t,y,w={}){if(!y||C||N||Q||c.state!=="running")return null;let O=l.get(t),U=w.loop??O?.loop??!1,te=w.bus||(U&&t!=="engine"?"ambience":"effects");if(!L[te])throw Error("audio: unknown bus");let T=U&&[...m].find(he=>he.loop&&he.id===t&&!he.fading);if(T)return w.position&&(T.position=w.position.slice(),T.pan?.positionX?[T.pan.positionX.value,T.pan.positionY.value,T.pan.positionZ.value]=w.position:B(T)),T;if(te==="ambience"&&[...m].filter(he=>he.bus==="ambience").length>=4)return null;if(m.size>=32){let he=[...m].filter(ye=>!ye.loop).sort((ye,Me)=>ye.priority-Me.priority||ye.start-Me.start)[0];if(!he||he.priority>(w.priority??1))return null;G(he)}let le=c.createBufferSource(),ae=c.createGain();le.buffer=y,le.loop=U,le.playbackRate.value=w.rate??(U?1:1+(Math.random()-.5)*.06);let de=Math.min(1,Math.max(0,w.gain??O?.gain??.6));ae.gain.setValueAtTime(U?0:de,c.currentTime),U&&ae.gain.linearRampToValueAtTime(de,c.currentTime+.3);let V;le.connect(ae),w.position?(v==="3d"?(V=c.createPanner(),V.panningModel="HRTF",V.distanceModel="inverse",V.refDistance=2,V.maxDistance=80,V.rolloffFactor=1.3,[V.positionX.value,V.positionY.value,V.positionZ.value]=w.position):V=c.createStereoPanner(),ae.connect(V).connect(L[te])):ae.connect(L[te]),te==="effects"&&(V||ae).connect(q);let xe={id:t,source:le,gain:ae,pan:V,loop:U,bus:te,volume:de,position:w.position?.slice(),priority:w.priority??1,start:c.currentTime};return v==="2d"&&V&&(ae.gain.cancelScheduledValues(c.currentTime),B(xe)),m.add(xe),le.onended=()=>G(xe),le.start(),xe}async function g(t,y={}){if(!l.has(t))return u("audio: unknown imported sound "+t),null;if(!c||C||N||Q||c.state!=="running")return null;if(y.position&&(!Array.isArray(y.position)||y.position.length!==3||y.position.some(T=>!Number.isFinite(T))))throw Error("audio: invalid position");for(let T of["rate","gain","cooldown","priority"])if(y[T]!==void 0&&!Number.isFinite(y[T]))throw Error("audio: invalid "+T);let w=F,O=P.get(t),U=c.currentTime;if(U-(A.get(t)??-99)<(y.cooldown??.045))return null;A.set(t,U);let te=y.loop??l.get(t).loop;try{let T=await J(t);return w!==F||O!==P.get(t)||!te&&c.currentTime-U>.2||y.ambient&&!n.includes(t)?null:i(t,T,y)}catch(T){return!C&&T.name!=="AbortError"&&!X.has(t)&&(X.add(t),u(String(T))),null}}let p=new Map;function S(t){let y={hit:[160,80],damage:[120,60],death:[180,90,45],respawn:[330,440,660],pickup:[660,990],win:[440,554,660,880],lose:[220,164,110]};if(!Object.hasOwn(y,t)||!c||C||N||Q||c.state!=="running")return null;let w="cue:"+t,O=c.currentTime;if(O-(A.get(w)??-99)<.12)return null;if(A.set(w,O),!p.has(t)){let U=y[t],te=U.length*.085,T=c.createBuffer(1,Math.ceil(te*c.sampleRate),c.sampleRate),le=T.getChannelData(0),ae=0,de=1973;for(let V=0;V<le.length;V++){let xe=V/c.sampleRate,he=Math.min(U.length-1,Math.floor(xe/.085)),ye=xe%.085;ae+=2*Math.PI*U[he]/c.sampleRate,de=Math.imul(de,1664525)+1013904223>>>0;let Me=["hit","damage","death"].includes(t)?(de/2147483648-1)*.25:0,Pe=Math.min(1,ye/.004)*Math.max(0,1-ye/.085)**2;le[V]=(Math.sin(ae)*.35+Math.sin(ae*2)*.06+Me)*Pe}p.set(t,T)}return i(w,p.get(t),{gain:.5,rate:1,priority:2})}let n=[];async function z(t){n=[...new Set(t)].slice(0,4);let y=new Set(n);for(let w of m)w.bus==="ambience"&&!y.has(w.id)&&ie(w);for(let w of n)!ee.has(w)&&![...m].some(O=>O.loop&&O.id===w&&!O.fading)&&(ee.add(w),g(w,{loop:!0,ambient:!0,bus:"ambience"}).finally(()=>ee.delete(w)))}async function j(t){if(!(!t.isTrusted||C||N)){pe();try{await c.resume(),await Promise.all([...l.keys()].map(y=>J(y).catch(w=>{!X.has(y)&&w.name!=="AbortError"&&(X.add(y),u("audio: "+w.message))}))),C||await z(n)}catch(y){C||u("audio: "+y.message)}}}return a.addEventListener("pointerdown",j,{signal:H.signal}),document.addEventListener("keydown",j,{signal:H.signal}),{play:g,cue:S,ambience:z,setRoom:Y,unlock:j,stop(t){if(l.has(t)){P.set(t,(P.get(t)||0)+1);for(let y of[...m])y.id===t&&G(y)}},setVolume(t){M=Math.max(0,Math.min(1,Number.isFinite(t)?t:.55)),Qe.set(a,{muted:Q,volume:M}),fe()},setMuted(t){if(Q=!!t,Qe.set(a,{muted:Q,volume:M}),F++,Q)for(let y of[...m])G(y);fe(),Q||z(n)},setPaused(t){if(N!==!!t){if(N=!!t,F++,N){for(let y of[...m])G(y);c?.suspend().catch(()=>{})}else c&&c.resume().then(()=>z(n)).catch(()=>{});fe()}},listener(t,y=[0,0,-1],w=[0,1,0]){if(oe.splice(0,3,...t),!c)return;if(v==="2d")for(let U of m)B(U);let O=c.listener;for(let[U,te]of[["position",t],["forward",y],["up",w]])for(let T=0;T<3;T++)O[U+"XYZ"[T]]&&(O[U+"XYZ"[T]].value=te[T])},connectMusic(t){if(!c)throw Error("audio: unlock with player interaction first");if(t.context!==c)throw Error("audio: music must use the game AudioContext");return t.connect(L.music),()=>t.disconnect(L.music)},get context(){return c},get preferences(){return{muted:Q,volume:M}},reset(){F++;for(let t of[...m])G(t);A.clear(),z(n)},stats(){return{voices:m.size,loops:[...m].filter(t=>t.loop).length,buffers:d.size,cue_buffers:p.size,state:c?.state||"locked"}},dispose(){if(!C){C=!0,H.abort();for(let t of[...m])G(t);d.clear(),p.clear(),E.clear(),q?.disconnect(),$?.disconnect(),se?.disconnect(),Object.values(L).forEach(t=>t.disconnect()),W?.disconnect(),I?.disconnect(),c?.close().catch(()=>{})}}}}var ot=(x,e,a)=>Math.max(e,Math.min(a,x));function It({adapter:x,config:e,root:a,report:u=console.warn,controls:v=!0}){e||={effects:[],sounds:[],bindings:[],quality:"auto"};let l=new Map((e.effects||[]).map(i=>[i.id,i])),d=new Map,E=new AbortController,m=it({sounds:e.sounds,base:e.base,root:a,report:u,dimension:x.dimension}),A={events:{},effects:{},sounds:{}},P=(i,g)=>{(Object.keys(i).length<64||Object.hasOwn(i,g))&&(i[g]=Math.min(1e6,(i[g]||0)+1))},H=new Map;for(let i of e.bindings||[])H.set(i.event,[...H.get(i.event)||[],i.sound]);let c=e.quality||"auto",W=2,I=0,se=!1,q=document.hidden,$=!1,C=0,N=0,Q=!0,M=0,Z=matchMedia("(prefers-reduced-motion: reduce)"),L=Z.matches;function oe(){x.quality(W,L)}function ee(i,g={}){let p=l.get(i);if(!p)throw Error("presentation: unknown imported effect "+i);let S={...p.defaults};for(let[n,z]of Object.entries(g))if(["position","normal"].includes(n)){if(!Array.isArray(z)||z.length!==3||z.some(j=>!Number.isFinite(j)||Math.abs(j)>1e6))throw Error("presentation: invalid "+n);S[n]=z.slice()}else if(n==="color"){if(!/^#[a-f0-9]{6}$/i.test(z))throw Error("presentation: invalid color");S[n]=z}else if(n==="fixed"){if(typeof z!="boolean")throw Error("presentation: invalid fixed");S[n]=z}else if(["intensity","scale","lifetime","cycle","hour","width","depth","y","speed","density","radius"].includes(n)){if(!Number.isFinite(z))throw Error("presentation: invalid "+n);S[n]=ot(z,n==="y"?-1e4:0,n==="cycle"?86400:n==="width"||n==="depth"?1e3:n==="hour"?24:n==="lifetime"?120:100)}else throw Error("presentation: unknown parameter "+n);return S}function X(i,g={}){let p=ee(i,g);return d.set(i,p),x.set(i,p),ie}function ne(i,g={}){if(se||q||$)return;let p=ee(i,g);if(i.startsWith("blood")&&!Q){l.has("metal-sparks")&&x.emit("metal-sparks",p);return}x.emit(i,p),P(A.effects,i)}let F=new Map,pe={shot:["muzzle-flash"],hit:["blood-spray","blood-pool","blood-decal","metal-sparks","stone-debris","hit-flash"],pickup:["pickup-glow"],splash:["water-splash","water-ripple"],win:["magic"]};function fe(i,g=[0,0,0],p=[0,1,0],S="flesh"){if(se||q||$)return;P(A.events,i);for(let j of F.has(i)?F.get(i).effects:pe[i]||[])l.has(j)&&(!j.startsWith("blood")||S==="flesh")&&(j!=="metal-sparks"||S==="metal")&&(j!=="stone-debris"||S==="stone")&&ne(j,{position:g,normal:p});let n=F.has(i)?F.get(i).sounds:H.get(i),z=n?.[Math.floor(Math.random()*n.length)];z?(P(A.sounds,z),m.play(z,{position:x.dimension==="2d"?[g[0],g[1],0]:g})):e.feedback===!0&&!F.has(i)&&m.cue(i)}function Y(){m.setPaused(se||q)}E.signal.addEventListener("abort",()=>m.dispose(),{once:!0}),document.addEventListener("visibilitychange",()=>{q=document.hidden,Y()},{signal:E.signal}),Z.addEventListener("change",i=>{L=i.matches,oe()},{signal:E.signal});let G=(l.get(e.environment)?.sounds||[]).filter(i=>e.sounds?.find(g=>g.id===i)?.loop);H.has("ambient")&&G.push(...H.get("ambient"));let ie={audio:m,set:X,emit:ne,event:fe,bindEvent(i,{effect:g,sound:p}={}){if(typeof i!="string"||!i||i.length>64)throw Error("presentation: invalid event name");if(g&&!l.has(g))throw Error("presentation: event effect was not imported "+g);if(p&&!e.sounds?.some(S=>S.id===p))throw Error("presentation: event sound was not imported "+p);return F.set(i,{effects:g?[g]:[],sounds:p?[p]:[]}),ie},setEnvironment(i){let g=l.get(i);if(g?.category!=="environment")throw Error("presentation: environment was not imported");d.clear(),x.clear?.();for(let p of g.effects)X(p);return G=g.sounds.filter(p=>e.sounds?.find(S=>S.id===p)?.loop),m.ambience(G),ie},setPaused(i){se=!!i,Y()},setActive(i){q=!i||document.hidden,Y()},setBlood(i){Q=!!i,x.blood?.(Q)},setQuality(i){if(!["auto","low","medium","high"].includes(i))throw Error("presentation: unknown quality");c=i,W=i==="low"?0:i==="medium"?1:2,C=N=0,oe()},registerSurface(...i){return x.registerSurface(...i)},applyObject(i,g,p){return x.applyObject(i,g,ee(g,p))},update(i){if(se||q||$)return;let g=ot(i,0,.1);I+=g,c==="auto"&&(i>.025?(C+=g,N=0):(N+=g,C=Math.max(0,C-g)),C>2&&W>0&&(W--,C=0,oe()),N>12&&W<2&&(W++,N=0,oe())),x.update(g,I)?.thunder&&(M=I+1.2),M&&I>=M&&(M=0,e.sounds?.some(S=>S.id==="thunder")&&m.play("thunder",{gain:.4}));let p=d.get("day-night");if(p&&G.includes("forest-day")&&G.includes("forest-night")){let S=p.fixed?p.hour:(p.hour+I*24/Math.max(1,p.cycle))%24;m.ambience([S>6&&S<19?"forest-day":"forest-night",...G.filter(n=>!n.startsWith("forest-"))])}else m.ambience(G)},render(){x.render?.()},resize(){x.resize?.()},reset(){I=M=0,x.reset(),m.reset(),C=N=0;for(let i of Object.values(A))for(let g of Object.keys(i))delete i[g]},audit(){return{events:{...A.events},effects:{...A.effects},sounds:{...A.sounds}}},stats(){return{...x.stats(),...m.stats(),quality:["low","medium","high"][W],time:I}},dispose(){$||($=!0,E.abort(),B?.remove(),x.dispose())}},B;if(v&&(l.size||e.sounds?.length||e.feedback===!0)){B=document.createElement("div"),B.className="aurago-game-presentation",B.style.cssText="position:absolute;right:12px;top:12px;z-index:1100;display:flex;gap:8px;align-items:center;padding:7px 10px;border:1px solid #ffffff30;border-radius:12px;background:#101822d9;color:#fff;font:12px system-ui;max-width:90%";let i=document.createElement("button");i.textContent=m.preferences.muted?"\u266B \xD7":"\u266A",i.title="Sound",i.setAttribute("aria-label","Mute sound"),i.setAttribute("aria-pressed",String(m.preferences.muted));let g=m.preferences.muted;i.onclick=()=>{g=!g,i.textContent=g?"\u266B \xD7":"\u266A",i.setAttribute("aria-pressed",String(g)),m.setMuted(g)};let p=document.createElement("input");p.type="range",p.min="0",p.max="100",p.value=String(Math.round(m.preferences.volume*100)),p.style.width="68px",p.setAttribute("aria-label","Volume"),p.oninput=()=>m.setVolume(+p.value/100);let S=document.createElement("select");S.setAttribute("aria-label","Effects quality");for(let n of["auto","low","medium","high"])S.add(new Option(n,n));if(S.value=c,S.onchange=()=>ie.setQuality(S.value),B.append(i,p,S),[...l.keys()].some(n=>n.startsWith("blood"))){let n=document.createElement("button");n.textContent="\u25CF",n.title="Blood effects",n.setAttribute("aria-label","Blood effects"),n.setAttribute("aria-pressed","true"),n.onclick=()=>{ie.setBlood(!Q),n.setAttribute("aria-pressed",String(Q))},B.append(n)}B.style.colorScheme="dark",B.style.accentColor="#71dfce",B.style.flexWrap="wrap";for(let n of B.querySelectorAll("button,select"))n.style.cssText="font:inherit;color:inherit;border:1px solid #ffffff28;border-radius:7px;background:#243443;padding:6px;min-height:"+(matchMedia("(pointer: coarse)").matches?44:32)+"px";a.append(B)}for(let i of l.values())i.category!=="environment"&&X(i.id);return ie.setQuality(c),m.ambience(G),Y(),ie}var Gt=`attribute float size;attribute float alpha;attribute float kind;varying vec3 tint;varying float opacity;varying float shape;
void main(){tint=color;opacity=alpha;shape=kind;vec4 p=modelViewMatrix*vec4(position,1.);gl_Position=projectionMatrix*p;gl_PointSize=clamp(size*650./max(1.,-p.z),1.,kind<.5?34.:160.);}`,jt=`varying vec3 tint;varying float opacity;varying float shape;void main(){vec2 p=gl_PointCoord*2.-1.;float a;
if(shape<.5){p.x*=6.;a=(1.-smoothstep(.35,1.,length(p)))*.5;}
else if(shape<1.5){a=(1.-smoothstep(.05,1.,length(p)))*.5;}
else if(shape<2.5){a=1.-smoothstep(.65,1.,length(p));}
else if(shape<3.5){p.y*=1.8;a=1.-smoothstep(.5,1.,length(p));}
else {p.y*=3.;a=(1.-smoothstep(0.,1.,length(p)))*.16;}
if(a*opacity<.015)discard;gl_FragColor=vec4(tint,a*opacity);}`,Qt={uniforms:{tDiffuse:{value:null},time:{value:0},vignette:{value:0},grain:{value:0},heat:{value:0},underwater:{value:0},grade:{value:0}},vertexShader:"varying vec2 uv0;void main(){uv0=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`uniform sampler2D tDiffuse;uniform float time,vignette,grain,heat,underwater,grade;varying vec2 uv0;
void main(){vec2 p=uv0;p.x+=(sin(p.y*32.+time*2.)*.002+sin(p.y*77.-time)*.001)*(heat+underwater);vec3 c=texture2D(tDiffuse,p).rgb;
c=mix(c,c*vec3(.55,.86,1.04),underwater*.6);c=mix(c,pow(max(c,vec3(0.)),vec3(.97))*vec3(1.035,1.01,.96),grade*.4);
c*=1.-vignette*.6*smoothstep(.15,.8,length(uv0-.5));c+=(fract(sin(dot(uv0+time,vec2(12.9898,78.233)))*43758.5453)-.5)*grain*.035;gl_FragColor=vec4(c,1.);}`};function Fi({scene:x,camera:e,renderer:a,sun:u,ambient:v}){let l=new h.Group;l.name="AuraGo presentation",x.add(l);let d=new Map,E=new Map,m=[],A=[],P=[],H=[],c=new Map,W=2,I=!1,se=!0,q=0,$=!1,C=71237,N=0,Q=6,M=()=>(C^=C<<13,C^=C>>>17,C^=C<<5,(C>>>0)/4294967296),Z=new h.Vector3,L=new h.Vector3,oe=new h.Raycaster,ee=new h.Vector3,X={background:x.background,fog:x.fog,sun:u&&{color:u.color.clone(),intensity:u.intensity,position:u.position.clone()},ambient:v?.intensity},ne=4e3,F=new Float32Array(ne*3),pe=new Float32Array(ne*3),fe=new Float32Array(ne),Y=new Float32Array(ne),J=new Float32Array(ne),G=new h.BufferGeometry;for(let[r,f,D]of[["position",F,3],["color",pe,3],["size",fe,1],["alpha",Y,1],["kind",J,1]])G.setAttribute(r,new h.BufferAttribute(f,D).setUsage(h.DynamicDrawUsage));G.setDrawRange(0,0);let ie=new h.ShaderMaterial({vertexShader:Gt,fragmentShader:jt,vertexColors:!0,transparent:!0,depthWrite:!1}),B=new h.Points(G,ie);B.frustumCulled=!1,l.add(B),m.push(G,ie);let i,g,p,S,n,z,j,t,y,w,O=new h.Color,U=new h.Color;function te(){if(i)return;i=new Ae,i.scale.setScalar(210),i.material.uniforms.cloudCoverage.value=0,i.material.uniforms.auraSkyColor={value:new h.Color(.42,.62,.78)},i.material.fragmentShader=`uniform vec3 auraSkyColor;
`+i.material.fragmentShader.replace("gl_FragColor = vec4( texColor, 1.0 );","vec3 d=normalize(vWorldPosition-cameraPosition);vec3 atmosphere=mix(auraSkyColor,auraSkyColor*vec3(.1,.28,.52),pow(max(0.,d.y),.45));gl_FragColor = vec4(mix(atmosphere,clamp(texColor*.035,0.,1.),.3),1.0);"),l.add(i),m.push(i.geometry,i.material);let r=new h.BufferGeometry,f=new Float32Array(1200*3);for(let R=0;R<1200;R++)L.set(M()-.5,M()-.5,M()-.5).normalize().multiplyScalar(190),f.set(L.toArray(),R*3);r.setAttribute("position",new h.BufferAttribute(f,3));let D=new h.PointsMaterial({color:13164287,size:.65,transparent:!0,depthWrite:!1,fog:!1});g=new h.Points(r,D),l.add(g),m.push(r,D);let b=new h.SphereGeometry(4,24,16),s=new h.MeshBasicMaterial({color:14147815,fog:!1});S=new h.Mesh(b,s),S.position.set(-80,100,-120),l.add(S),m.push(b,s);let o=new h.SphereGeometry(205,32,16),_=new h.ShaderMaterial({side:h.BackSide,transparent:!0,depthWrite:!1,uniforms:{time:{value:0},coverage:{value:.5},night:{value:0}},vertexShader:"varying vec3 p;void main(){p=position;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`varying vec3 p;uniform float time,coverage,night;
float hash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}float noise(vec2 x){vec2 i=floor(x),f=fract(x);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1,0)),f.x),mix(hash(i+vec2(0,1)),hash(i+1.),f.x),f.y);}
void main(){vec3 d=normalize(p);vec2 uv=night>1.1?vec2(atan(d.z,d.x),asin(d.y))*3.:d.xz/max(.12,d.y)*1.7+vec2(time*.015,0);float n=0.,a=.55;for(int i=0;i<5;i++){n+=a*noise(uv);uv=uv*2.03+.71;a*=.5;}float c=smoothstep(1.-coverage,1.15-coverage,n)*(night>1.1?1.:smoothstep(0.,.2,d.y));gl_FragColor=night>1.1?vec4(mix(vec3(.05,.12,.3),vec3(.3,.07,.4),n),c*.45):vec4(mix(vec3(.78,.87,.95),vec3(.09,.12,.18),night),c*.42);}`});p=new h.Mesh(o,_),l.add(p),m.push(o,_)}function T(){j||(j=new Ue(a),j.addPass(new Ne(x,e)),t=new _e(new h.Vector2(512,512),.3,.35,.8),j.addPass(t),y=new Ce(Qt),j.addPass(y),w=new Oe,j.addPass(w),Me())}function le(r,f){n&&(l.remove(n),n.geometry.dispose(),n.material.dispose(),n.dispose(),z.dispose());let D=new Uint8Array(4096*4);for(let o=0;o<4096;o++)D[o*4]=128+Math.sin(o*.37)*32,D[o*4+1]=128+Math.cos(o*.51)*32,D[o*4+2]=245,D[o*4+3]=255;z=new h.DataTexture(D,64,64),z.wrapS=z.wrapT=h.RepeatWrapping,z.magFilter=z.minFilter=h.LinearFilter,z.needsUpdate=!0;let b=new h.PlaneGeometry(f.width||80,f.depth||80,32,32);n=new Fe(b,{textureWidth:512,textureHeight:512,waterNormals:z,sunDirection:new h.Vector3(1,1,1),sunColor:16772305,waterColor:r==="water-river"?2387301:1204096,distortionScale:r==="water-ocean"?3:1.2,fog:!!x.fog}),n.material.fragmentShader=n.material.fragmentShader.replace("vec3 outgoingLight = albedo;","vec3 outgoingLight = mix(albedo,waterColor,.4);").replace("gl_FragColor = vec4( outgoingLight, alpha );",`
            float wave=sin(worldPosition.x*.32+time)*cos(worldPosition.z*.24-time*.6);
            float foam=smoothstep(.94,.995,wave)*(.3+.7*sin(worldPosition.x*8.+worldPosition.z*9.)*sin(worldPosition.x*8.+worldPosition.z*9.));
            outgoingLight=mix(outgoingLight,vec3(.65,.87,.9),foam*.25);
            gl_FragColor = vec4(outgoingLight, alpha);`),n.rotation.x=-Math.PI/2,n.position.fromArray(f.position||[0,f.y??-.05,20]),n.userData.base=b.attributes.position.array.slice(),n.userData.id=r,n.userData.speed=f.speed||.6;let s=n.onBeforeRender;n.onBeforeRender=function(...o){W===2&&s.apply(this,o)},l.add(n)}function ae(r,f){if(d.set(r,f),r.startsWith("sky-")||r==="day-night"){for(let D of d.keys())r.startsWith("sky-")&&D.startsWith("sky-")&&D!==r&&d.delete(D);te()}["water-lake","water-river","water-ocean"].includes(r)&&le(r,f),["bloom","color-grade","vignette","film-grain","heat-haze","underwater"].includes(r)&&T(),r==="fog-distance"&&(x.fog=new h.FogExp2(10268850,f.density??.018))}function de(r,f){return E.size?(oe.set(L.set(r,200,f),ee.set(0,-1,0)),oe.intersectObjects([...E.keys()],!0)[0]?.point.y??0):0}function V(r,f,D){let b=[500,1500,4e3][W];if(A.length>=b)return;let s=r.startsWith("rain"),o=r==="snow",_=r==="smoke"||r.startsWith("fog"),R=r==="wind-leaves",k=r.startsWith("blood"),re=r==="fire",ue=r==="engine-trail",ge=r==="water-splash",ce=r==="stone-debris",rt=k?f.color||"#9f162a":_?"#9caebb":s||ge?"#b3d7ed":ce?"#a69b89":o?"#f5faff":R?"#9eae55":ue||r==="magic"||r==="teleport"||r==="pickup-glow"?"#76ddff":re?"#ff792e":"#ffbd57";O.set(f.color&&f.color!=="#ffffff"?f.color:rt);let Ee=D||f.position||[0,1,0],Te=f.scale||1;A.push({id:r,x:Ee[0],y:Ee[1],z:Ee[2],vx:(M()-.5)*(s?2:6)*Te,vy:s?-26:o?-1.5:_?.5+M():(M()*5+1)*Te,vz:(M()-.5)*(s?1:6)*Te,age:0,life:s?2.5:o?12:_?5:f.lifetime||1.5,size:(s?.25:o?.09:_?1.8:k?.11:.18)*Te,c:O.toArray(),kind:s?0:r.startsWith("fog")?4:_?1:R?3:2,floor:s||o?de(Ee[0],Ee[2]):0});let K=A[A.length-1];if((re||ue)&&(K.kind=1,K.size=(re?.5:.25)*Te,K.life=re?1:.5,K.vx*=.1,K.vz*=.1,K.vy=re?2:0,ue)){let We=f.normal||[0,0,-1];K.vx=We[0]*5,K.vy=We[1]*5,K.vz=We[2]*5}(r==="muzzle-flash"||r==="hit-flash")&&(K.life=.12,K.size=.6*Te,K.kind=1,K.vx=K.vy=K.vz=0),f.normal&&(k||r==="metal-sparks"||ce||ge)&&(K.vx+=f.normal[0]*3,K.vy+=f.normal[1]*3,K.vz+=f.normal[2]*3),(r==="wind-dust"||R)&&(K.vx=3,K.vy=-.15,K.life=8,K.size=R?.16:.5,K.kind=R?3:1)}function xe(r,f){if(!se)return;for(r==="blood-pool"&&(f={...f,position:[f.position?.[0]||0,de(f.position?.[0]||0,f.position?.[2]||0),f.position?.[2]||0],normal:[0,1,0]});P.length>=[24,48,96][W];){let k=P.shift();k.mesh.removeFromParent(),k.mesh.geometry.dispose(),k.mesh.material.dispose(),k.tex?.dispose()}let D=document.createElement("canvas");D.width=D.height=96;let b=D.getContext("2d");b.fillStyle=f.color||"#8c1625",b.beginPath();for(let k=0;k<=24;k++){let re=k/24*Math.PI*2,ue=25+M()*19,ge=48+Math.cos(re)*ue,ce=48+Math.sin(re)*ue;k?b.lineTo(ge,ce):b.moveTo(ge,ce)}b.fill();for(let k=0;k<14;k++)b.beginPath(),b.arc(M()*96,M()*96,1+M()*3,0,7),b.fill();let s=new h.CanvasTexture(D),o=new h.PlaneGeometry((r==="blood-pool"?1.2:.5)*(f.scale||1),(r==="blood-pool"?1:.6)*(f.scale||1)),_=new h.MeshBasicMaterial({map:s,transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,side:h.DoubleSide}),R=new h.Mesh(o,_);ee.fromArray(f.normal||[0,1,0]),ee.lengthSq()<.001&&ee.set(0,1,0),ee.normalize(),R.quaternion.setFromUnitVectors(new h.Vector3(0,0,1),ee),R.position.fromArray(f.position||[0,0,0]).addScaledVector(ee,.006),l.add(R),P.push({mesh:R,tex:s,age:0,life:f.lifetime||30})}function he(r,f){if(!["blood-pool","blood-decal","blood-spray","hit-flash","metal-sparks","stone-debris","fire","smoke","embers","explosion","muzzle-flash","engine-trail","magic","teleport","pickup-glow","water-splash","water-ripple"].includes(r))return;if(r==="blood-pool"||r==="blood-decal"){xe(r,f);return}if(I&&(r==="hit-flash"||r==="muzzle-flash"))return;if(r==="hit-flash"){for(let b of H)b.id==="hit-flash"&&(b.age=0);V(r,{...f,color:"#efffff",lifetime:.08,scale:2});return}let D=Math.round((r==="explosion"?90:r==="muzzle-flash"?8:30)*Math.min(2,f.intensity??1));if(r!=="water-ripple")for(let b=0;b<D;b++)V(r,f);if(r==="explosion"){for(let b=0;b<18;b++)V("smoke",{...f,color:"#515760",scale:1.2,lifetime:3});for(let b of A.slice(-D-18,-18))b.vx*=2,b.vz*=2,b.vy*=1.5}if(r==="water-ripple"){for(;P.length>=[24,48,96][W];){let _=P.shift();_.mesh.removeFromParent(),_.mesh.geometry.dispose(),_.mesh.material.dispose(),_.tex?.dispose()}let b=new h.RingGeometry(.09,.1,32),s=new h.MeshBasicMaterial({color:11789555,transparent:!0,opacity:.65,side:h.DoubleSide,depthWrite:!1}),o=new h.Mesh(b,s);o.rotation.x=-Math.PI/2,o.position.fromArray(f.position||[0,.02,0]),l.add(o),P.push({mesh:o,age:0,life:f.lifetime||1.4,ripple:!0,scale:f.scale||1})}}function ye(r,f){let D=!1;if(q=f,e.getWorldPosition(Z),i&&!d.has("day-night")&&![...d.keys()].some(s=>s.startsWith("sky-"))&&(i.visible=g.visible=p.visible=S.visible=!1),i&&(d.has("day-night")||[...d.keys()].some(s=>s.startsWith("sky-")))){let s=d.get("day-night"),o=[...d.keys()].find(ue=>ue.startsWith("sky-"))||"sky-clear",R=((s?s.fixed?s.hour:(s.hour+f*24/Math.max(1,s.cycle))%24:o==="sky-night"||o==="sky-space"?0:o==="sky-sunset"?17.4:11)-6)/24*Math.PI*2,k=Math.max(0,Math.sin(R)),re=1-Math.min(1,k*3);i.position.copy(Z),p.position.copy(Z),g.position.copy(Z),S.position.copy(Z).add(L.set(-80,100,-120)),i.material.uniforms.sunPosition.value.set(Math.cos(R)*100,Math.sin(R)*100,-30),i.material.uniforms.turbidity.value=o==="sky-storm"?18:5,i.material.uniforms.rayleigh.value=2,i.material.uniforms.auraSkyColor.value.set(o==="sky-storm"?6714244:k<.3?14189931:8371940),i.visible=re<.95,g.visible=re>.15,g.material.opacity=re,S.visible=o!=="sky-space"&&re>.4,re>.7&&(x.background=U.set(o==="sky-space"?396065:1121334)),u&&(u.position.set(Math.cos(R)*35,Math.sin(R)*40,15),u.intensity=.12+k*2.7,u.color.setRGB(1,.65+k*.28,.48+k*.4)),v&&(v.intensity=.3+k*1.3),p.material.uniforms.time.value=I?0:q,p.material.uniforms.coverage.value=o==="sky-cloudy"?.65:o==="sky-storm"?.85:.38,p.material.uniforms.night.value=re,p.visible=!0,p.material.uniforms.night.value=o==="sky-space"?1.5:re,x.fog&&x.fog.color.setRGB(.12+k*.45,.16+k*.48,.24+k*.45)}let b=[...d.keys()].find(s=>["rain-light","rain-heavy","snow","wind-dust","wind-leaves"].includes(s));if(b){N+=r*(b==="rain-heavy"?650:b==="rain-light"?220:b==="snow"?75:25)*[.3,.65,1][W]*Math.min(3,d.get(b).intensity??1);let s=Math.min(32,Math.floor(N));for(N=Math.min(1,N-s);s-- >0;)V(b,d.get(b),[Z.x+(M()-.5)*34,Z.y+8+M()*7,Z.z+(M()-.5)*34])}for(let s of["fire","smoke","embers","engine-trail","fog-ground","fog-zone"])if(d.has(s)&&(s.startsWith("fog")||d.get(s).position)&&M()<r*18*Math.min(3,d.get(s).intensity??1)){let o=d.get(s),_=s.startsWith("fog"),R=o.position||[Z.x,.5,Z.z],k=s==="fog-zone"?o.radius||12:18;V(s,{...o,scale:_?5*(o.scale||1):o.scale,lifetime:_?8:o.lifetime||2},_?[R[0]+(M()-.5)*k*2,R[1],R[2]+(M()-.5)*k*2]:R)}d.has("thunderstorm")&&q>Q&&(D=!0,u&&!I&&(u.intensity=5),Q=q+5+M()*12);for(let s=A.length-1;s>=0;s--){let o=A[s];if(o.age+=r,o.age>o.life){A.splice(s,1);continue}if(o.x+=o.vx*r,o.y+=o.vy*r,o.z+=o.vz*r,!o.id.startsWith("rain")&&o.id!=="snow"&&o.kind!==1&&o.kind!==4&&(o.vy-=6*r),o.y<o.floor){o.id.startsWith("rain")&&W>0&&M()<.08&&he("water-ripple",{position:[o.x,o.floor+.02,o.z],lifetime:.6,scale:.25}),A.splice(s,1);continue}}for(let s=0;s<A.length;s++){let o=A[s];F.set([o.x,o.y,o.z],s*3),pe.set(o.c,s*3),fe[s]=o.size*(o.kind===1?1+o.age*.2:1),Y[s]=(o.id.startsWith("rain")?Math.min(1,o.age*8):1)*(1-o.age/o.life),J[s]=o.kind}G.setDrawRange(0,A.length);for(let s of Object.values(G.attributes))s.needsUpdate=!0;for(let s=P.length-1;s>=0;s--){let o=P[s];o.age+=r,o.mesh.material.opacity=Math.min(1,(o.life-o.age)/Math.min(5,o.life)),o.ripple&&o.mesh.scale.setScalar((1+o.age*7)*o.scale),o.age>o.life&&(o.mesh.removeFromParent(),o.mesh.geometry.dispose(),o.mesh.material.dispose(),o.tex?.dispose(),P.splice(s,1))}for(let s of H)s.age+=r,s.clock.value=I?0:q,s.progress.value=s.id==="dissolve"?Math.min(1,s.age/s.life):s.id==="hit-flash"?Math.max(0,1-s.age*8):0;if(n){n.material.uniforms.time.value=q*n.userData.speed;let s=n.geometry.attributes.position,o=n.userData.base;for(let _=0;_<s.count;_++)s.array[_*3+2]=Math.sin(o[_*3]*.3+q)*Math.cos(o[_*3+1]*.25-q*.6)*(n.userData.id==="water-ocean"?.25:.035);s.needsUpdate=!0,n.geometry.computeVertexNormals(),n.material.uniforms.sunDirection.value.copy(u?.position||L.set(1,1,1)).normalize()}if(j){t.enabled=W>0&&d.has("bloom"),t.strength=.3*(d.get("bloom")?.intensity??1);let s=y.uniforms;s.time.value=q;for(let[o,_]of[["vignette","vignette"],["grain","film-grain"],["heat","heat-haze"],["underwater","underwater"],["grade","color-grade"]])s[o].value=d.has(_)?I&&["heat","underwater"].includes(o)?0:Math.min(2,d.get(_).intensity??1):0}return{thunder:D}}function Me(){let r=a.getSize(new h.Vector2);j?.setSize(r.x,r.y)}function Pe(){A.length=0;for(let r of P)r.mesh.removeFromParent(),r.mesh.geometry.dispose(),r.mesh.material.dispose(),r.tex?.dispose();P.length=0,G.setDrawRange(0,0),N=0,Q=6,C=71237;for(let r of H)r.age=0,r.progress.value=0,r.clock.value=0}return{dimension:"3d",set:ae,emit:he,update:ye,resize:Me,reset:Pe,clear(){d.clear(),Pe(),x.fog=X.fog,i&&(i.visible=g.visible=p.visible=S.visible=!1),n&&(n.removeFromParent(),n.geometry.dispose(),n.material.dispose(),n.dispose(),z.dispose(),n=null)},quality(r,f){W=r,I=f,j&&j.setPixelRatio(Math.min(a.getPixelRatio(),r===0?.75:r===1?1:1.5))},blood(r){if(se=r,!se)for(let f of P)f.ripple||(f.life=0)},render(){j?j.render(0):a.render(x,e)},registerSurface(r,f="ground"){return E.set(r,f),()=>E.delete(r)},applyObject(r,f,D={}){if(!["hologram","dissolve","hit-flash"].includes(f))throw Error("presentation: unsupported object effect");if(I&&f==="hit-flash")return()=>{};c.get(r)?.(),c.size>=128&&c.values().next().value();let b=[];r.traverse(_=>{if(!_.isMesh)return;let R=_.material,k=(Array.isArray(R)?R:[R]).map(re=>{let ue=re.clone(),ge={material:ue,id:f,age:0,life:D.lifetime||2,progress:{value:0},clock:{value:0}};return ue.transparent=!0,ue.onBeforeCompile=ce=>{ce.uniforms.auraProgress=ge.progress,ce.uniforms.auraTime=ge.clock,ce.vertexShader=`varying vec3 auraPosition;
`+ce.vertexShader.replace("#include <begin_vertex>",`#include <begin_vertex>
auraPosition=position;`),ce.fragmentShader=`uniform float auraProgress,auraTime;varying vec3 auraPosition;
`+ce.fragmentShader.replace("#include <dithering_fragment>",f==="dissolve"?"float n=fract(sin(dot(floor(auraPosition*35.),vec3(12.9898,78.233,37.719)))*43758.5453);if(n<auraProgress)discard;gl_FragColor.rgb+=vec3(1.,.3,.04)*(1.-smoothstep(0.,.07,n-auraProgress));":f==="hologram"?"float scan=.6+.4*sin(auraPosition.y*90.-auraTime*5.);gl_FragColor=vec4(vec3(.13,.72,1.)*scan,.38+scan*.3);":"gl_FragColor.rgb=mix(gl_FragColor.rgb,vec3(1.),auraProgress);")},ue.customProgramCacheKey=()=>"aurago-"+f,H.push(ge),b.push(()=>{ue.dispose();let ce=H.indexOf(ge);ce>=0&&H.splice(ce,1)}),ue});_.material=Array.isArray(R)?k:k[0],b.push(()=>{_.material=R})});let s=!1,o=()=>{s||(s=!0,b.forEach(_=>_()),c.delete(r))};return c.set(r,o),o},stats(){return{particles:A.length,decals:P.length,surfaces:E.size,objects:H.length}},dispose(){if(!$){$=!0,Pe(),l.removeFromParent();for(let r of c.values())r();m.forEach(r=>r.dispose()),n&&(n.geometry.dispose(),n.material.dispose(),n.dispose(),z.dispose()),j?.dispose(),t?.dispose(),y?.dispose(),w?.dispose(),E.clear(),x.background=X.background,x.fog=X.fog,u&&X.sun&&(u.color.copy(X.sun.color),u.intensity=X.sun.intensity,u.position.copy(X.sun.position)),v&&(v.intensity=X.ambient)}}}}export{It as createPresentation,Fi as createThreeAdapter};
