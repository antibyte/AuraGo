import*as f from"./three-0.185.1.module.min.js";import{BackSide as rt,BoxGeometry as at,Mesh as st,ShaderMaterial as nt,UniformsUtils as lt,Vector3 as je}from"./three-0.185.1.module.min.js";var Se=class y extends st{constructor(){let e=y.SkyShader,a=new nt({name:e.name,uniforms:lt.clone(e.uniforms),vertexShader:e.vertexShader,fragmentShader:e.fragmentShader,side:rt,depthWrite:!1});super(new at(1,1,1),a),this.isSky=!0}};Se.SkyShader={name:"SkyShader",uniforms:{turbidity:{value:2},rayleigh:{value:1},mieCoefficient:{value:.005},mieDirectionalG:{value:.8},sunPosition:{value:new je},up:{value:new je(0,1,0)},cloudScale:{value:2e-4},cloudSpeed:{value:1e-4},cloudCoverage:{value:.4},cloudDensity:{value:.4},cloudElevation:{value:.5},showSunDisc:{value:1},time:{value:0}},vertexShader:`
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

		}`};import{Color as Ae,FrontSide as ct,HalfFloatType as ut,Matrix4 as Le,Mesh as ft,PerspectiveCamera as ht,Plane as mt,ShaderMaterial as dt,UniformsLib as Qe,UniformsUtils as He,Vector3 as de,Vector4 as qe,WebGLRenderTarget as pt}from"./three-0.185.1.module.min.js";var ke=class extends ft{constructor(e,a={}){super(e),this.isWater=!0;let l=this,x=a.textureWidth!==void 0?a.textureWidth:512,n=a.textureHeight!==void 0?a.textureHeight:512,m=a.clipBias!==void 0?a.clipBias:0,P=a.alpha!==void 0?a.alpha:1,p=a.time!==void 0?a.time:0,k=a.waterNormals!==void 0?a.waterNormals:null,_=a.sunDirection!==void 0?a.sunDirection:new de(.70707,.70707,0),O=new Ae(a.sunColor!==void 0?a.sunColor:16777215),d=new Ae(a.waterColor!==void 0?a.waterColor:8355711),B=a.eye!==void 0?a.eye:new de(0,0,0),L=a.distortionScale!==void 0?a.distortionScale:20,oe=a.side!==void 0?a.side:ct,V=a.fog!==void 0?a.fog:!1,H=new mt,S=new de,U=new de,Z=new de,C=new Le,R=new de(0,0,-1),G=new qe,ie=new de,q=new de,X=new qe,J=new Le,I=new ht,se=new pt(x,n,{type:ut}),ue={name:"MirrorShader",uniforms:He.merge([Qe.fog,Qe.lights,{normalSampler:{value:null},mirrorSampler:{value:null},alpha:{value:1},time:{value:0},size:{value:1},distortionScale:{value:20},textureMatrix:{value:new Le},sunColor:{value:new Ae(8355711)},sunDirection:{value:new de(.70707,.70707,0)},eye:{value:new de},waterColor:{value:new Ae(5592405)}}]),vertexShader:`
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
				}`},Q=new dt({name:ue.name,uniforms:He.clone(ue.uniforms),vertexShader:ue.vertexShader,fragmentShader:ue.fragmentShader,lights:!0,side:oe,fog:V});Q.uniforms.mirrorSampler.value=se.texture,this.dispose=()=>se.dispose(),Q.uniforms.textureMatrix.value=J,Q.uniforms.alpha.value=P,Q.uniforms.time.value=p,Q.uniforms.normalSampler.value=k,Q.uniforms.sunColor.value=O,Q.uniforms.waterColor.value=d,Q.uniforms.sunDirection.value=_,Q.uniforms.distortionScale.value=L,Q.uniforms.eye.value=B,l.material=Q,l.onBeforeRender=function(W,Y,K){if(U.setFromMatrixPosition(l.matrixWorld),Z.setFromMatrixPosition(K.matrixWorld),C.extractRotation(l.matrixWorld),S.set(0,0,1),S.applyMatrix4(C),ie.subVectors(U,Z),ie.dot(S)>0)return;ie.reflect(S).negate(),ie.add(U),C.extractRotation(K.matrixWorld),R.set(0,0,-1),R.applyMatrix4(C),R.add(Z),q.subVectors(U,R),q.reflect(S).negate(),q.add(U),I.position.copy(ie),I.up.set(0,1,0),I.up.applyMatrix4(C),I.up.reflect(S),I.lookAt(q),I.far=K.far,I.updateMatrixWorld(),I.projectionMatrix.copy(K.projectionMatrix),J.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),J.multiply(I.projectionMatrix),J.multiply(I.matrixWorldInverse),H.setFromNormalAndCoplanarPoint(S,U),H.applyMatrix4(I.matrixWorldInverse),G.set(H.normal.x,H.normal.y,H.normal.z,H.constant);let N=I.projectionMatrix;X.x=(Math.sign(G.x)+N.elements[8])/N.elements[0],X.y=(Math.sign(G.y)+N.elements[9])/N.elements[5],X.z=-1,X.w=(1+N.elements[10])/N.elements[14],G.multiplyScalar(2/G.dot(X)),N.elements[2]=G.x,N.elements[6]=G.y,N.elements[10]=G.z+1-m,N.elements[14]=G.w,B.setFromMatrixPosition(K.matrixWorld);let o=W.getRenderTarget(),h=W.xr.enabled,v=W.shadowMap.autoUpdate;l.visible=!1,W.xr.enabled=!1,W.shadowMap.autoUpdate=!1,W.setRenderTarget(se),W.state.buffers.depth.setMask(!0),W.autoClear===!1&&W.clear(),W.render(Y,I),l.visible=!0,W.xr.enabled=h,W.shadowMap.autoUpdate=v,W.setRenderTarget(o);let b=K.viewport;b!==void 0&&W.state.viewport(b)}}};import{HalfFloatType as Mt,NoBlending as Tt,Timer as St,Vector2 as Ze,WebGLRenderTarget as Ct}from"./three-0.185.1.module.min.js";var we={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};import{ShaderMaterial as Ke,UniformsUtils as bt}from"./three-0.185.1.module.min.js";import{BufferGeometry as gt,Float32BufferAttribute as Xe,OrthographicCamera as vt,Mesh as xt}from"./three-0.185.1.module.min.js";var ce=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},yt=new vt(-1,1,1,-1,0,1),We=class extends gt{constructor(){super(),this.setAttribute("position",new Xe([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new Xe([0,2,0,0,2,0],2))}},wt=new We,pe=class{constructor(e){this._mesh=new xt(wt,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,yt)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var be=class extends ce{constructor(e,a="tDiffuse"){super(),this.textureID=a,this.uniforms=null,this.material=null,e instanceof Ke?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=bt.clone(e.uniforms),this.material=new Ke({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new pe(this.material)}render(e,a,l){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=l.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(a),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ce=class extends ce{constructor(e,a){super(),this.scene=e,this.camera=a,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,a,l){let x=e.getContext(),n=e.state;n.buffers.color.setMask(!1),n.buffers.depth.setMask(!1),n.buffers.color.setLocked(!0),n.buffers.depth.setLocked(!0);let m,P;this.inverse?(m=0,P=1):(m=1,P=0),n.buffers.stencil.setTest(!0),n.buffers.stencil.setOp(x.REPLACE,x.REPLACE,x.REPLACE),n.buffers.stencil.setFunc(x.ALWAYS,m,4294967295),n.buffers.stencil.setClear(P),n.buffers.stencil.setLocked(!0),e.setRenderTarget(l),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(a),this.clear&&e.clear(),e.render(this.scene,this.camera),n.buffers.color.setLocked(!1),n.buffers.depth.setLocked(!1),n.buffers.color.setMask(!0),n.buffers.depth.setMask(!0),n.buffers.stencil.setLocked(!1),n.buffers.stencil.setFunc(x.EQUAL,1,4294967295),n.buffers.stencil.setOp(x.KEEP,x.KEEP,x.KEEP),n.buffers.stencil.setLocked(!0)}},ze=class extends ce{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Re=class{constructor(e,a){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),a===void 0){let l=e.getSize(new Ze);this._width=l.width,this._height=l.height,a=new Ct(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Mt}),a.texture.name="EffectComposer.rt1"}else this._width=a.width,this._height=a.height;this.renderTarget1=a,this.renderTarget2=a.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new be(we),this.copyPass.material.blending=Tt,this.timer=new St}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,a){this.passes.splice(a,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let a=this.passes.indexOf(e);a!==-1&&this.passes.splice(a,1)}isLastEnabledPass(e){for(let a=e+1;a<this.passes.length;a++)if(this.passes[a].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let a=this.renderer.getRenderTarget(),l=!1;for(let x=0,n=this.passes.length;x<n;x++){let m=this.passes[x];if(m.enabled!==!1){if(m.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(x),m.render(this.renderer,this.writeBuffer,this.readBuffer,e,l),m.needsSwap){if(l){let P=this.renderer.getContext(),p=this.renderer.state.buffers.stencil;p.setFunc(P.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),p.setFunc(P.EQUAL,1,4294967295)}this.swapBuffers()}Ce!==void 0&&(m instanceof Ce?l=!0:m instanceof ze&&(l=!1))}}this.renderer.setRenderTarget(a)}reset(e){if(e===void 0){let a=this.renderer.getSize(new Ze);this._pixelRatio=this.renderer.getPixelRatio(),this._width=a.width,this._height=a.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,a){this._width=e,this._height=a;let l=this._width*this._pixelRatio,x=this._height*this._pixelRatio;this.renderTarget1.setSize(l,x),this.renderTarget2.setSize(l,x);for(let n=0;n<this.passes.length;n++)this.passes[n].setSize(l,x)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};import{Color as _t}from"./three-0.185.1.module.min.js";var De=class extends ce{constructor(e,a,l=null,x=null,n=null){super(),this.scene=e,this.camera=a,this.overrideMaterial=l,this.clearColor=x,this.clearAlpha=n,this.clear=!0,this.clearDepth=!1,this.needsSwap=!1,this.isRenderPass=!0,this._oldClearColor=new _t}render(e,a,l){let x=e.autoClear;e.autoClear=!1;let n,m;this.overrideMaterial!==null&&(m=this.scene.overrideMaterial,this.scene.overrideMaterial=this.overrideMaterial),this.clearColor!==null&&(e.getClearColor(this._oldClearColor),e.setClearColor(this.clearColor,e.getClearAlpha())),this.clearAlpha!==null&&(n=e.getClearAlpha(),e.setClearAlpha(this.clearAlpha)),this.clearDepth==!0&&e.clearDepth(),e.setRenderTarget(this.renderToScreen?null:l),this.clear===!0&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),e.render(this.scene,this.camera),this.clearColor!==null&&e.setClearColor(this._oldClearColor),this.clearAlpha!==null&&e.setClearAlpha(n),this.overrideMaterial!==null&&(this.scene.overrideMaterial=m),e.autoClear=x}};import{AdditiveBlending as Et,Color as $e,HalfFloatType as Oe,MeshBasicMaterial as At,ShaderMaterial as Fe,UniformsUtils as Je,Vector2 as ge,Vector3 as _e,WebGLRenderTarget as Ve}from"./three-0.185.1.module.min.js";import{Color as Pt}from"./three-0.185.1.module.min.js";var Ye={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Pt(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Me=class y extends ce{constructor(e,a=1,l,x){super(),this.strength=a,this.radius=l,this.threshold=x,this.resolution=e!==void 0?new ge(e.x,e.y):new ge(256,256),this.clearColor=new $e(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let n=Math.round(this.resolution.x/2),m=Math.round(this.resolution.y/2);this.renderTargetBright=new Ve(n,m,{type:Oe}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let _=0;_<this.nMips;_++){let O=new Ve(n,m,{type:Oe});O.texture.name="UnrealBloomPass.h"+_,O.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(O);let d=new Ve(n,m,{type:Oe});d.texture.name="UnrealBloomPass.v"+_,d.texture.generateMipmaps=!1,this.renderTargetsVertical.push(d),n=Math.round(n/2),m=Math.round(m/2)}let P=Ye;this.highPassUniforms=Je.clone(P.uniforms),this.highPassUniforms.luminosityThreshold.value=x,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new Fe({uniforms:this.highPassUniforms,vertexShader:P.vertexShader,fragmentShader:P.fragmentShader}),this.separableBlurMaterials=[];let p=[6,10,14,18,22];n=Math.round(this.resolution.x/2),m=Math.round(this.resolution.y/2);for(let _=0;_<this.nMips;_++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(p[_])),this.separableBlurMaterials[_].uniforms.invSize.value=new ge(1/n,1/m),n=Math.round(n/2),m=Math.round(m/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=a,this.compositeMaterial.uniforms.bloomRadius.value=.1;let k=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=k,this.bloomTintColors=[new _e(1,1,1),new _e(1,1,1),new _e(1,1,1),new _e(1,1,1),new _e(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=Je.clone(we.uniforms),this.blendMaterial=new Fe({uniforms:this.copyUniforms,vertexShader:we.vertexShader,fragmentShader:we.fragmentShader,premultipliedAlpha:!0,blending:Et,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new $e,this._oldClearAlpha=1,this._basic=new At,this._fsQuad=new pe(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,a){let l=Math.round(e/2),x=Math.round(a/2);this.renderTargetBright.setSize(l,x);for(let n=0;n<this.nMips;n++)this.renderTargetsHorizontal[n].setSize(l,x),this.renderTargetsVertical[n].setSize(l,x),this.separableBlurMaterials[n].uniforms.invSize.value=new ge(1/l,1/x),l=Math.round(l/2),x=Math.round(x/2)}render(e,a,l,x,n){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let m=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),n&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=l.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=l.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let P=this.renderTargetBright;for(let p=0;p<this.nMips;p++)this._fsQuad.material=this.separableBlurMaterials[p],this.separableBlurMaterials[p].uniforms.colorTexture.value=P.texture,this.separableBlurMaterials[p].uniforms.direction.value=y.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[p]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[p].uniforms.colorTexture.value=this.renderTargetsHorizontal[p].texture,this.separableBlurMaterials[p].uniforms.direction.value=y.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[p]),e.clear(),this._fsQuad.render(e),P=this.renderTargetsVertical[p];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,n&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(l),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=m}_getSeparableBlurMaterial(e){let a=[],l=e/3;for(let x=0;x<e;x++)a.push(.39894*Math.exp(-.5*x*x/(l*l))/l);return new Fe({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new ge(.5,.5)},direction:{value:new ge(.5,.5)},gaussianCoefficients:{value:a}},vertexShader:`

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

				}`})}_getCompositeMaterial(e){return new Fe({defines:{NUM_MIPS:e},uniforms:{blurTexture1:{value:null},blurTexture2:{value:null},blurTexture3:{value:null},blurTexture4:{value:null},blurTexture5:{value:null},bloomStrength:{value:1},bloomFactors:{value:null},bloomTintColors:{value:null},bloomRadius:{value:0}},vertexShader:`

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

				}`})}};Me.BlurDirectionX=new ge(1,0);Me.BlurDirectionY=new ge(0,1);import{ColorManagement as kt,RawShaderMaterial as zt,UniformsUtils as Rt,LinearToneMapping as Dt,ReinhardToneMapping as Ft,CineonToneMapping as Bt,AgXToneMapping as Ut,ACESFilmicToneMapping as Nt,NeutralToneMapping as Lt,CustomToneMapping as Wt,SRGBTransfer as Ot}from"./three-0.185.1.module.min.js";var Pe={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var Be=class extends ce{constructor(){super(),this.isOutputPass=!0,this.uniforms=Rt.clone(Pe.uniforms),this.material=new zt({name:Pe.name,uniforms:this.uniforms,vertexShader:Pe.vertexShader,fragmentShader:Pe.fragmentShader}),this._fsQuad=new pe(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,a,l){this.uniforms.tDiffuse.value=l.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},kt.getTransfer(this._outputColorSpace)===Ot&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Dt?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Ft?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Bt?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Nt?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Ut?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Lt?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Wt&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(a),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};function et({sounds:y=[],base:e,root:a,report:l=console.warn,dimension:x="3d"}){let n=new Map(y.map(t=>[t.id,t])),m=new Map,P=new Map,p=new Set,k=new Map,_=new Map,O=new AbortController,d,B,L,oe,V,H,S=!1,U=!1,Z=!1,C=.55,R={},G=[0,0,0],ie=new Set,q=new Set,X="outside",J=0;function I(){if(!(d||S)){d=new AudioContext({sampleRate:48e3}),B=d.createGain(),L=d.createDynamicsCompressor(),L.threshold.value=-12,L.knee.value=12,L.ratio.value=8,L.attack.value=.003,L.release.value=.2,B.connect(L).connect(d.destination);for(let t of["effects","ambience","music"])R[t]=d.createGain(),R[t].gain.value=t==="ambience"?.45:1,R[t].connect(B);V=d.createConvolver(),oe=d.createGain(),H=d.createBiquadFilter(),H.type="lowpass",H.frequency.value=8e3,V.connect(H).connect(oe).connect(B),ue(X),se()}}function se(){B&&B.gain.setTargetAtTime(Z||U?0:C*C,d.currentTime,.03)}function ue(t){if(!["outside","small-room","hall"].includes(t))throw Error("audio: unknown room");if(X=t,!d)return;oe.gain.value=t==="outside"?0:t==="hall"?.2:.1;let u=Math.floor(d.sampleRate*(t==="hall"?1.7:.35)),g=d.createBuffer(2,u,d.sampleRate),D=1973;for(let F=0;F<2;F++){let $=g.getChannelData(F);for(let M=0;M<u;M++)D=Math.imul(D,1664525)+1013904223>>>0,$[M]=(D/2147483648-1)*Math.pow(1-M/u,3)}V.buffer=g}async function Q(t){if(m.has(t))return m.get(t);if(P.has(t))return P.get(t);let u=n.get(t);if(!u)throw Error("audio: unknown imported sound "+t);let g=(async()=>{let D=u.files?.[0];if(!D||!D.file.startsWith("sounds/")||!/^sounds\/[a-z0-9-]+\.wav$/.test(D.file))throw Error("audio: invalid sample path");let F=new URL(e,document.baseURI),$=new URL(D.file,F);if($.origin!==new URL(document.baseURI).origin||!$.pathname.startsWith(F.pathname))throw Error("audio: sample outside game");let M=await fetch($,{signal:O.signal});if(!M.ok)throw Error("audio: "+t+" HTTP "+M.status);let ne=await M.arrayBuffer();if(ne.byteLength!==D.bytes)throw Error("audio: size mismatch "+t);if(crypto.subtle&&[...new Uint8Array(await crypto.subtle.digest("SHA-256",ne))].map(ee=>ee.toString(16).padStart(2,"0")).join("")!==D.sha256)throw Error("audio: checksum mismatch "+t);let le=await d.decodeAudioData(ne);return S?null:(m.set(t,le),le)})();P.set(t,g);try{return await g}finally{P.delete(t)}}function W(t){if(p.has(t)){p.delete(t);try{t.source.stop()}catch{}t.source.disconnect(),t.gain.disconnect(),t.pan?.disconnect()}}function Y(t,u=.18){if(!(!p.has(t)||t.fading)){t.fading=!0,t.gain.gain.setTargetAtTime(0,d.currentTime,u/3);try{t.source.stop(d.currentTime+u)}catch{}}}function K(t){!t.position||!t.pan?.pan||t.fading||(t.pan.pan.value=Math.max(-1,Math.min(1,(t.position[0]-G[0])/450)),t.gain.gain.setTargetAtTime(t.volume/(1+Math.hypot(t.position[0]-G[0],t.position[1]-G[1])/500),d.currentTime,.03))}function N(t,u,g={}){if(!u||S||U||Z||d.state!=="running")return null;let D=n.get(t),F=g.loop??D?.loop??!1,$=g.bus||(F&&t!=="engine"?"ambience":"effects");if(!R[$])throw Error("audio: unknown bus");let M=F&&[...p].find(fe=>fe.loop&&fe.id===t&&!fe.fading);if(M)return g.position&&(M.position=g.position.slice(),M.pan?.positionX?[M.pan.positionX.value,M.pan.positionY.value,M.pan.positionZ.value]=g.position:K(M)),M;if($==="ambience"&&[...p].filter(fe=>fe.bus==="ambience").length>=4)return null;if(p.size>=32){let fe=[...p].filter(he=>!he.loop).sort((he,Ee)=>he.priority-Ee.priority||he.start-Ee.start)[0];if(!fe||fe.priority>(g.priority??1))return null;W(fe)}let ne=d.createBufferSource(),le=d.createGain();ne.buffer=u,ne.loop=F,ne.playbackRate.value=g.rate??(F?1:1+(Math.random()-.5)*.06);let ve=Math.min(1,Math.max(0,g.gain??D?.gain??.6));le.gain.setValueAtTime(F?0:ve,d.currentTime),F&&le.gain.linearRampToValueAtTime(ve,d.currentTime+.3);let ee;ne.connect(le),g.position?(x==="3d"?(ee=d.createPanner(),ee.panningModel="HRTF",ee.distanceModel="inverse",ee.refDistance=2,ee.maxDistance=80,ee.rolloffFactor=1.3,[ee.positionX.value,ee.positionY.value,ee.positionZ.value]=g.position):ee=d.createStereoPanner(),le.connect(ee).connect(R[$])):le.connect(R[$]),$==="effects"&&(ee||le).connect(V);let xe={id:t,source:ne,gain:le,pan:ee,loop:F,bus:$,volume:ve,position:g.position?.slice(),priority:g.priority??1,start:d.currentTime};return x==="2d"&&ee&&(le.gain.cancelScheduledValues(d.currentTime),K(xe)),p.add(xe),ne.onended=()=>W(xe),ne.start(),xe}async function o(t,u={}){if(!n.has(t))return l("audio: unknown imported sound "+t),null;if(!d||S||U||Z||d.state!=="running")return null;if(u.position&&(!Array.isArray(u.position)||u.position.length!==3||u.position.some(M=>!Number.isFinite(M))))throw Error("audio: invalid position");for(let M of["rate","gain","cooldown","priority"])if(u[M]!==void 0&&!Number.isFinite(u[M]))throw Error("audio: invalid "+M);let g=J,D=_.get(t),F=d.currentTime;if(F-(k.get(t)??-99)<(u.cooldown??.045))return null;k.set(t,F);let $=u.loop??n.get(t).loop;try{let M=await Q(t);return g!==J||D!==_.get(t)||!$&&d.currentTime-F>.2||u.ambient&&!h.includes(t)?null:N(t,M,u)}catch(M){return!S&&M.name!=="AbortError"&&!q.has(t)&&(q.add(t),l(String(M))),null}}let h=[];async function v(t){h=[...new Set(t)].slice(0,4);let u=new Set(h);for(let g of p)g.bus==="ambience"&&!u.has(g.id)&&Y(g);for(let g of h)!ie.has(g)&&![...p].some(D=>D.loop&&D.id===g&&!D.fading)&&(ie.add(g),o(g,{loop:!0,ambient:!0,bus:"ambience"}).finally(()=>ie.delete(g)))}async function b(t){if(!(!t.isTrusted||S||U)){I();try{await d.resume(),await Promise.all([...n.keys()].map(u=>Q(u).catch(g=>{!q.has(u)&&g.name!=="AbortError"&&(q.add(u),l("audio: "+g.message))}))),S||await v(h)}catch(u){S||l("audio: "+u.message)}}}return a.addEventListener("pointerdown",b,{signal:O.signal}),a.addEventListener("keydown",b,{signal:O.signal}),{play:o,ambience:v,setRoom:ue,unlock:b,stop(t){if(n.has(t)){_.set(t,(_.get(t)||0)+1);for(let u of[...p])u.id===t&&W(u)}},setVolume(t){C=Math.max(0,Math.min(1,Number.isFinite(t)?t:.55)),se()},setMuted(t){if(Z=!!t,J++,Z)for(let u of[...p])W(u);se(),Z||v(h)},setPaused(t){if(U!==!!t){if(U=!!t,J++,U){for(let u of[...p])W(u);d?.suspend().catch(()=>{})}else d&&d.resume().then(()=>v(h)).catch(()=>{});se()}},listener(t,u=[0,0,-1],g=[0,1,0]){if(G.splice(0,3,...t),!d)return;if(x==="2d")for(let F of p)K(F);let D=d.listener;for(let[F,$]of[["position",t],["forward",u],["up",g]])for(let M=0;M<3;M++)D[F+"XYZ"[M]]&&(D[F+"XYZ"[M]].value=$[M])},connectMusic(t){if(!d)throw Error("audio: unlock with player interaction first");if(t.context!==d)throw Error("audio: music must use the game AudioContext");return t.connect(R.music),()=>t.disconnect(R.music)},get context(){return d},reset(){J++;for(let t of[...p])W(t);k.clear(),v(h)},stats(){return{voices:p.size,loops:[...p].filter(t=>t.loop).length,buffers:m.size,state:d?.state||"locked"}},dispose(){if(!S){S=!0,O.abort();for(let t of[...p])W(t);m.clear(),P.clear(),V?.disconnect(),H?.disconnect(),oe?.disconnect(),Object.values(R).forEach(t=>t.disconnect()),B?.disconnect(),L?.disconnect(),d?.close().catch(()=>{})}}}}var tt=(y,e,a)=>Math.max(e,Math.min(a,y));function Vt({adapter:y,config:e,root:a,report:l=console.warn,controls:x=!0}){e||={effects:[],sounds:[],bindings:[],quality:"auto"};let n=new Map((e.effects||[]).map(o=>[o.id,o])),m=new Map,P=new AbortController,p=et({sounds:e.sounds,base:e.base,root:a,report:l,dimension:y.dimension}),k={events:{},effects:{},sounds:{}},_=(o,h)=>{(Object.keys(o).length<64||Object.hasOwn(o,h))&&(o[h]=Math.min(1e6,(o[h]||0)+1))},O=new Map;for(let o of e.bindings||[])O.set(o.event,[...O.get(o.event)||[],o.sound]);let d=e.quality||"auto",B=2,L=0,oe=!1,V=document.hidden,H=!1,S=0,U=0,Z=!0,C=0,R=matchMedia("(prefers-reduced-motion: reduce)"),G=R.matches;function ie(){y.quality(B,G)}function q(o,h={}){let v=n.get(o);if(!v)throw Error("presentation: unknown imported effect "+o);let b={...v.defaults};for(let[t,u]of Object.entries(h))if(["position","normal"].includes(t)){if(!Array.isArray(u)||u.length!==3||u.some(g=>!Number.isFinite(g)||Math.abs(g)>1e6))throw Error("presentation: invalid "+t);b[t]=u.slice()}else if(t==="color"){if(!/^#[a-f0-9]{6}$/i.test(u))throw Error("presentation: invalid color");b[t]=u}else if(t==="fixed"){if(typeof u!="boolean")throw Error("presentation: invalid fixed");b[t]=u}else if(["intensity","scale","lifetime","cycle","hour","width","depth","y","speed","density","radius"].includes(t)){if(!Number.isFinite(u))throw Error("presentation: invalid "+t);b[t]=tt(u,t==="y"?-1e4:0,t==="cycle"?86400:t==="width"||t==="depth"?1e3:t==="hour"?24:t==="lifetime"?120:100)}else throw Error("presentation: unknown parameter "+t);return b}function X(o,h={}){let v=q(o,h);return m.set(o,v),y.set(o,v),K}function J(o,h={}){if(oe||V||H)return;let v=q(o,h);if(o.startsWith("blood")&&!Z){n.has("metal-sparks")&&y.emit("metal-sparks",v);return}y.emit(o,v),_(k.effects,o)}let I=new Map,se={shot:["muzzle-flash"],hit:["blood-spray","blood-pool","blood-decal","metal-sparks","stone-debris","hit-flash"],pickup:["pickup-glow"],splash:["water-splash","water-ripple"],win:["magic"]};function ue(o,h=[0,0,0],v=[0,1,0],b="flesh"){if(oe||V||H)return;_(k.events,o);for(let g of I.has(o)?I.get(o).effects:se[o]||[])n.has(g)&&(!g.startsWith("blood")||b==="flesh")&&(g!=="metal-sparks"||b==="metal")&&(g!=="stone-debris"||b==="stone")&&J(g,{position:h,normal:v});let t=I.has(o)?I.get(o).sounds:O.get(o),u=t?.[Math.floor(Math.random()*t.length)];u&&(_(k.sounds,u),p.play(u,{position:y.dimension==="2d"?[h[0],h[1],0]:h}))}function Q(){p.setPaused(oe||V)}P.signal.addEventListener("abort",()=>p.dispose(),{once:!0}),document.addEventListener("visibilitychange",()=>{V=document.hidden,Q()},{signal:P.signal}),R.addEventListener("change",o=>{G=o.matches,ie()},{signal:P.signal});let Y=(n.get(e.environment)?.sounds||[]).filter(o=>e.sounds?.find(h=>h.id===o)?.loop);O.has("ambient")&&Y.push(...O.get("ambient"));let K={audio:p,set:X,emit:J,event:ue,bindEvent(o,{effect:h,sound:v}={}){if(typeof o!="string"||!o||o.length>64)throw Error("presentation: invalid event name");if(h&&!n.has(h))throw Error("presentation: event effect was not imported "+h);if(v&&!e.sounds?.some(b=>b.id===v))throw Error("presentation: event sound was not imported "+v);return I.set(o,{effects:h?[h]:[],sounds:v?[v]:[]}),K},setEnvironment(o){let h=n.get(o);if(h?.category!=="environment")throw Error("presentation: environment was not imported");m.clear(),y.clear?.();for(let v of h.effects)X(v);return Y=h.sounds.filter(v=>e.sounds?.find(b=>b.id===v)?.loop),p.ambience(Y),K},setPaused(o){oe=!!o,Q()},setActive(o){V=!o||document.hidden,Q()},setBlood(o){Z=!!o,y.blood?.(Z)},setQuality(o){if(!["auto","low","medium","high"].includes(o))throw Error("presentation: unknown quality");d=o,B=o==="low"?0:o==="medium"?1:2,S=U=0,ie()},registerSurface(...o){return y.registerSurface(...o)},applyObject(o,h,v){return y.applyObject(o,h,q(h,v))},update(o){if(oe||V||H)return;let h=tt(o,0,.1);L+=h,d==="auto"&&(o>.025?(S+=h,U=0):(U+=h,S=Math.max(0,S-h)),S>2&&B>0&&(B--,S=0,ie()),U>12&&B<2&&(B++,U=0,ie())),y.update(h,L)?.thunder&&(C=L+1.2),C&&L>=C&&(C=0,e.sounds?.some(b=>b.id==="thunder")&&p.play("thunder",{gain:.4}));let v=m.get("day-night");if(v&&Y.includes("forest-day")&&Y.includes("forest-night")){let b=v.fixed?v.hour:(v.hour+L*24/Math.max(1,v.cycle))%24;p.ambience([b>6&&b<19?"forest-day":"forest-night",...Y.filter(t=>!t.startsWith("forest-"))])}else p.ambience(Y)},render(){y.render?.()},resize(){y.resize?.()},reset(){L=C=0,y.reset(),p.reset(),S=U=0;for(let o of Object.values(k))for(let h of Object.keys(o))delete o[h]},audit(){return{events:{...k.events},effects:{...k.effects},sounds:{...k.sounds}}},stats(){return{...y.stats(),...p.stats(),quality:["low","medium","high"][B],time:L}},dispose(){H||(H=!0,P.abort(),N?.remove(),y.dispose())}},N;if(x&&(n.size||e.sounds?.length)){N=document.createElement("div"),N.className="aurago-game-presentation",N.style.cssText="position:absolute;right:12px;top:12px;z-index:1100;display:flex;gap:8px;align-items:center;padding:7px 10px;border:1px solid #ffffff30;border-radius:12px;background:#101822d9;color:#fff;font:12px system-ui;max-width:90%";let o=document.createElement("button");o.textContent="\u266A",o.title="Sound",o.setAttribute("aria-label","Mute sound"),o.setAttribute("aria-pressed","false");let h=!1;o.onclick=()=>{h=!h,o.textContent=h?"\u266B \xD7":"\u266A",o.setAttribute("aria-pressed",String(h)),p.setMuted(h)};let v=document.createElement("input");v.type="range",v.min="0",v.max="100",v.value="55",v.style.width="68px",v.setAttribute("aria-label","Volume"),v.oninput=()=>p.setVolume(+v.value/100);let b=document.createElement("select");b.setAttribute("aria-label","Effects quality");for(let t of["auto","low","medium","high"])b.add(new Option(t,t));if(b.value=d,b.onchange=()=>K.setQuality(b.value),N.append(o,v,b),[...n.keys()].some(t=>t.startsWith("blood"))){let t=document.createElement("button");t.textContent="\u25CF",t.title="Blood effects",t.setAttribute("aria-label","Blood effects"),t.setAttribute("aria-pressed","true"),t.onclick=()=>{K.setBlood(!Z),t.setAttribute("aria-pressed",String(Z))},N.append(t)}N.style.colorScheme="dark",N.style.accentColor="#71dfce",N.style.flexWrap="wrap";for(let t of N.querySelectorAll("button,select"))t.style.cssText="font:inherit;color:inherit;border:1px solid #ffffff28;border-radius:7px;background:#243443;padding:6px;min-height:"+(matchMedia("(pointer: coarse)").matches?44:32)+"px";a.append(N)}for(let o of n.values())o.category!=="environment"&&X(o.id);return K.setQuality(d),p.ambience(Y),Q(),K}var It=`attribute float size;attribute float alpha;attribute float kind;varying vec3 tint;varying float opacity;varying float shape;
void main(){tint=color;opacity=alpha;shape=kind;vec4 p=modelViewMatrix*vec4(position,1.);gl_Position=projectionMatrix*p;gl_PointSize=clamp(size*650./max(1.,-p.z),1.,kind<.5?34.:160.);}`,Gt=`varying vec3 tint;varying float opacity;varying float shape;void main(){vec2 p=gl_PointCoord*2.-1.;float a;
if(shape<.5){p.x*=6.;a=(1.-smoothstep(.35,1.,length(p)))*.5;}
else if(shape<1.5){a=(1.-smoothstep(.05,1.,length(p)))*.5;}
else if(shape<2.5){a=1.-smoothstep(.65,1.,length(p));}
else if(shape<3.5){p.y*=1.8;a=1.-smoothstep(.5,1.,length(p));}
else {p.y*=3.;a=(1.-smoothstep(0.,1.,length(p)))*.16;}
if(a*opacity<.015)discard;gl_FragColor=vec4(tint,a*opacity);}`,jt={uniforms:{tDiffuse:{value:null},time:{value:0},vignette:{value:0},grain:{value:0},heat:{value:0},underwater:{value:0},grade:{value:0}},vertexShader:"varying vec2 uv0;void main(){uv0=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`uniform sampler2D tDiffuse;uniform float time,vignette,grain,heat,underwater,grade;varying vec2 uv0;
void main(){vec2 p=uv0;p.x+=(sin(p.y*32.+time*2.)*.002+sin(p.y*77.-time)*.001)*(heat+underwater);vec3 c=texture2D(tDiffuse,p).rgb;
c=mix(c,c*vec3(.55,.86,1.04),underwater*.6);c=mix(c,pow(max(c,vec3(0.)),vec3(.97))*vec3(1.035,1.01,.96),grade*.4);
c*=1.-vignette*.6*smoothstep(.15,.8,length(uv0-.5));c+=(fract(sin(dot(uv0+time,vec2(12.9898,78.233)))*43758.5453)-.5)*grain*.035;gl_FragColor=vec4(c,1.);}`};function Di({scene:y,camera:e,renderer:a,sun:l,ambient:x}){let n=new f.Group;n.name="AuraGo presentation",y.add(n);let m=new Map,P=new Map,p=[],k=[],_=[],O=[],d=new Map,B=2,L=!1,oe=!0,V=0,H=!1,S=71237,U=0,Z=6,C=()=>(S^=S<<13,S^=S>>>17,S^=S<<5,(S>>>0)/4294967296),R=new f.Vector3,G=new f.Vector3,ie=new f.Raycaster,q=new f.Vector3,X={background:y.background,fog:y.fog,sun:l&&{color:l.color.clone(),intensity:l.intensity,position:l.position.clone()},ambient:x?.intensity},J=4e3,I=new Float32Array(J*3),se=new Float32Array(J*3),ue=new Float32Array(J),Q=new Float32Array(J),W=new Float32Array(J),Y=new f.BufferGeometry;for(let[r,c,z]of[["position",I,3],["color",se,3],["size",ue,1],["alpha",Q,1],["kind",W,1]])Y.setAttribute(r,new f.BufferAttribute(c,z).setUsage(f.DynamicDrawUsage));Y.setDrawRange(0,0);let K=new f.ShaderMaterial({vertexShader:It,fragmentShader:Gt,vertexColors:!0,transparent:!0,depthWrite:!1}),N=new f.Points(Y,K);N.frustumCulled=!1,n.add(N),p.push(Y,K);let o,h,v,b,t,u,g,D,F,$,M=new f.Color,ne=new f.Color;function le(){if(o)return;o=new Se,o.scale.setScalar(210),o.material.uniforms.cloudCoverage.value=0,o.material.uniforms.auraSkyColor={value:new f.Color(.42,.62,.78)},o.material.fragmentShader=`uniform vec3 auraSkyColor;
`+o.material.fragmentShader.replace("gl_FragColor = vec4( texColor, 1.0 );","vec3 d=normalize(vWorldPosition-cameraPosition);vec3 atmosphere=mix(auraSkyColor,auraSkyColor*vec3(.1,.28,.52),pow(max(0.,d.y),.45));gl_FragColor = vec4(mix(atmosphere,clamp(texColor*.035,0.,1.),.3),1.0);"),n.add(o),p.push(o.geometry,o.material);let r=new f.BufferGeometry,c=new Float32Array(1200*3);for(let A=0;A<1200;A++)G.set(C()-.5,C()-.5,C()-.5).normalize().multiplyScalar(190),c.set(G.toArray(),A*3);r.setAttribute("position",new f.BufferAttribute(c,3));let z=new f.PointsMaterial({color:13164287,size:.65,transparent:!0,depthWrite:!1,fog:!1});h=new f.Points(r,z),n.add(h),p.push(r,z);let w=new f.SphereGeometry(4,24,16),s=new f.MeshBasicMaterial({color:14147815,fog:!1});b=new f.Mesh(w,s),b.position.set(-80,100,-120),n.add(b),p.push(w,s);let i=new f.SphereGeometry(205,32,16),T=new f.ShaderMaterial({side:f.BackSide,transparent:!0,depthWrite:!1,uniforms:{time:{value:0},coverage:{value:.5},night:{value:0}},vertexShader:"varying vec3 p;void main(){p=position;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`varying vec3 p;uniform float time,coverage,night;
float hash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}float noise(vec2 x){vec2 i=floor(x),f=fract(x);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1,0)),f.x),mix(hash(i+vec2(0,1)),hash(i+1.),f.x),f.y);}
void main(){vec3 d=normalize(p);vec2 uv=night>1.1?vec2(atan(d.z,d.x),asin(d.y))*3.:d.xz/max(.12,d.y)*1.7+vec2(time*.015,0);float n=0.,a=.55;for(int i=0;i<5;i++){n+=a*noise(uv);uv=uv*2.03+.71;a*=.5;}float c=smoothstep(1.-coverage,1.15-coverage,n)*(night>1.1?1.:smoothstep(0.,.2,d.y));gl_FragColor=night>1.1?vec4(mix(vec3(.05,.12,.3),vec3(.3,.07,.4),n),c*.45):vec4(mix(vec3(.78,.87,.95),vec3(.09,.12,.18),night),c*.42);}`});v=new f.Mesh(i,T),n.add(v),p.push(i,T)}function ve(){g||(g=new Re(a),g.addPass(new De(y,e)),D=new Me(new f.Vector2(512,512),.3,.35,.8),g.addPass(D),F=new be(jt),g.addPass(F),$=new Be,g.addPass($),Ge())}function ee(r,c){t&&(n.remove(t),t.geometry.dispose(),t.material.dispose(),t.dispose(),u.dispose());let z=new Uint8Array(4096*4);for(let i=0;i<4096;i++)z[i*4]=128+Math.sin(i*.37)*32,z[i*4+1]=128+Math.cos(i*.51)*32,z[i*4+2]=245,z[i*4+3]=255;u=new f.DataTexture(z,64,64),u.wrapS=u.wrapT=f.RepeatWrapping,u.magFilter=u.minFilter=f.LinearFilter,u.needsUpdate=!0;let w=new f.PlaneGeometry(c.width||80,c.depth||80,32,32);t=new ke(w,{textureWidth:512,textureHeight:512,waterNormals:u,sunDirection:new f.Vector3(1,1,1),sunColor:16772305,waterColor:r==="water-river"?2387301:1204096,distortionScale:r==="water-ocean"?3:1.2,fog:!!y.fog}),t.material.fragmentShader=t.material.fragmentShader.replace("vec3 outgoingLight = albedo;","vec3 outgoingLight = mix(albedo,waterColor,.4);").replace("gl_FragColor = vec4( outgoingLight, alpha );",`
            float wave=sin(worldPosition.x*.32+time)*cos(worldPosition.z*.24-time*.6);
            float foam=smoothstep(.94,.995,wave)*(.3+.7*sin(worldPosition.x*8.+worldPosition.z*9.)*sin(worldPosition.x*8.+worldPosition.z*9.));
            outgoingLight=mix(outgoingLight,vec3(.65,.87,.9),foam*.25);
            gl_FragColor = vec4(outgoingLight, alpha);`),t.rotation.x=-Math.PI/2,t.position.fromArray(c.position||[0,c.y??-.05,20]),t.userData.base=w.attributes.position.array.slice(),t.userData.id=r,t.userData.speed=c.speed||.6;let s=t.onBeforeRender;t.onBeforeRender=function(...i){B===2&&s.apply(this,i)},n.add(t)}function xe(r,c){if(m.set(r,c),r.startsWith("sky-")||r==="day-night"){for(let z of m.keys())r.startsWith("sky-")&&z.startsWith("sky-")&&z!==r&&m.delete(z);le()}["water-lake","water-river","water-ocean"].includes(r)&&ee(r,c),["bloom","color-grade","vignette","film-grain","heat-haze","underwater"].includes(r)&&ve(),r==="fog-distance"&&(y.fog=new f.FogExp2(10268850,c.density??.018))}function fe(r,c){return P.size?(ie.set(G.set(r,200,c),q.set(0,-1,0)),ie.intersectObjects([...P.keys()],!0)[0]?.point.y??0):0}function he(r,c,z){let w=[500,1500,4e3][B];if(k.length>=w)return;let s=r.startsWith("rain"),i=r==="snow",T=r==="smoke"||r.startsWith("fog"),A=r==="wind-leaves",E=r.startsWith("blood"),te=r==="fire",re=r==="engine-trail",me=r==="water-splash",ae=r==="stone-debris",ot=E?c.color||"#9f162a":T?"#9caebb":s||me?"#b3d7ed":ae?"#a69b89":i?"#f5faff":A?"#9eae55":re||r==="magic"||r==="teleport"||r==="pickup-glow"?"#76ddff":te?"#ff792e":"#ffbd57";M.set(c.color&&c.color!=="#ffffff"?c.color:ot);let Te=z||c.position||[0,1,0],ye=c.scale||1;k.push({id:r,x:Te[0],y:Te[1],z:Te[2],vx:(C()-.5)*(s?2:6)*ye,vy:s?-26:i?-1.5:T?.5+C():(C()*5+1)*ye,vz:(C()-.5)*(s?1:6)*ye,age:0,life:s?2.5:i?12:T?5:c.lifetime||1.5,size:(s?.25:i?.09:T?1.8:E?.11:.18)*ye,c:M.toArray(),kind:s?0:r.startsWith("fog")?4:T?1:A?3:2,floor:s||i?fe(Te[0],Te[2]):0});let j=k[k.length-1];if((te||re)&&(j.kind=1,j.size=(te?.5:.25)*ye,j.life=te?1:.5,j.vx*=.1,j.vz*=.1,j.vy=te?2:0,re)){let Ne=c.normal||[0,0,-1];j.vx=Ne[0]*5,j.vy=Ne[1]*5,j.vz=Ne[2]*5}(r==="muzzle-flash"||r==="hit-flash")&&(j.life=.12,j.size=.6*ye,j.kind=1,j.vx=j.vy=j.vz=0),c.normal&&(E||r==="metal-sparks"||ae||me)&&(j.vx+=c.normal[0]*3,j.vy+=c.normal[1]*3,j.vz+=c.normal[2]*3),(r==="wind-dust"||A)&&(j.vx=3,j.vy=-.15,j.life=8,j.size=A?.16:.5,j.kind=A?3:1)}function Ee(r,c){if(!oe)return;for(r==="blood-pool"&&(c={...c,position:[c.position?.[0]||0,fe(c.position?.[0]||0,c.position?.[2]||0),c.position?.[2]||0],normal:[0,1,0]});_.length>=[24,48,96][B];){let E=_.shift();E.mesh.removeFromParent(),E.mesh.geometry.dispose(),E.mesh.material.dispose(),E.tex?.dispose()}let z=document.createElement("canvas");z.width=z.height=96;let w=z.getContext("2d");w.fillStyle=c.color||"#8c1625",w.beginPath();for(let E=0;E<=24;E++){let te=E/24*Math.PI*2,re=25+C()*19,me=48+Math.cos(te)*re,ae=48+Math.sin(te)*re;E?w.lineTo(me,ae):w.moveTo(me,ae)}w.fill();for(let E=0;E<14;E++)w.beginPath(),w.arc(C()*96,C()*96,1+C()*3,0,7),w.fill();let s=new f.CanvasTexture(z),i=new f.PlaneGeometry((r==="blood-pool"?1.2:.5)*(c.scale||1),(r==="blood-pool"?1:.6)*(c.scale||1)),T=new f.MeshBasicMaterial({map:s,transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,side:f.DoubleSide}),A=new f.Mesh(i,T);q.fromArray(c.normal||[0,1,0]),q.lengthSq()<.001&&q.set(0,1,0),q.normalize(),A.quaternion.setFromUnitVectors(new f.Vector3(0,0,1),q),A.position.fromArray(c.position||[0,0,0]).addScaledVector(q,.006),n.add(A),_.push({mesh:A,tex:s,age:0,life:c.lifetime||30})}function Ie(r,c){if(!["blood-pool","blood-decal","blood-spray","hit-flash","metal-sparks","stone-debris","fire","smoke","embers","explosion","muzzle-flash","engine-trail","magic","teleport","pickup-glow","water-splash","water-ripple"].includes(r))return;if(r==="blood-pool"||r==="blood-decal"){Ee(r,c);return}if(L&&(r==="hit-flash"||r==="muzzle-flash"))return;if(r==="hit-flash"){for(let w of O)w.id==="hit-flash"&&(w.age=0);he(r,{...c,color:"#efffff",lifetime:.08,scale:2});return}let z=Math.round((r==="explosion"?90:r==="muzzle-flash"?8:30)*Math.min(2,c.intensity??1));if(r!=="water-ripple")for(let w=0;w<z;w++)he(r,c);if(r==="explosion"){for(let w=0;w<18;w++)he("smoke",{...c,color:"#515760",scale:1.2,lifetime:3});for(let w of k.slice(-z-18,-18))w.vx*=2,w.vz*=2,w.vy*=1.5}if(r==="water-ripple"){for(;_.length>=[24,48,96][B];){let T=_.shift();T.mesh.removeFromParent(),T.mesh.geometry.dispose(),T.mesh.material.dispose(),T.tex?.dispose()}let w=new f.RingGeometry(.09,.1,32),s=new f.MeshBasicMaterial({color:11789555,transparent:!0,opacity:.65,side:f.DoubleSide,depthWrite:!1}),i=new f.Mesh(w,s);i.rotation.x=-Math.PI/2,i.position.fromArray(c.position||[0,.02,0]),n.add(i),_.push({mesh:i,age:0,life:c.lifetime||1.4,ripple:!0,scale:c.scale||1})}}function it(r,c){let z=!1;if(V=c,e.getWorldPosition(R),o&&!m.has("day-night")&&![...m.keys()].some(s=>s.startsWith("sky-"))&&(o.visible=h.visible=v.visible=b.visible=!1),o&&(m.has("day-night")||[...m.keys()].some(s=>s.startsWith("sky-")))){let s=m.get("day-night"),i=[...m.keys()].find(re=>re.startsWith("sky-"))||"sky-clear",A=((s?s.fixed?s.hour:(s.hour+c*24/Math.max(1,s.cycle))%24:i==="sky-night"||i==="sky-space"?0:i==="sky-sunset"?17.4:11)-6)/24*Math.PI*2,E=Math.max(0,Math.sin(A)),te=1-Math.min(1,E*3);o.position.copy(R),v.position.copy(R),h.position.copy(R),b.position.copy(R).add(G.set(-80,100,-120)),o.material.uniforms.sunPosition.value.set(Math.cos(A)*100,Math.sin(A)*100,-30),o.material.uniforms.turbidity.value=i==="sky-storm"?18:5,o.material.uniforms.rayleigh.value=2,o.material.uniforms.auraSkyColor.value.set(i==="sky-storm"?6714244:E<.3?14189931:8371940),o.visible=te<.95,h.visible=te>.15,h.material.opacity=te,b.visible=i!=="sky-space"&&te>.4,te>.7&&(y.background=ne.set(i==="sky-space"?396065:1121334)),l&&(l.position.set(Math.cos(A)*35,Math.sin(A)*40,15),l.intensity=.12+E*2.7,l.color.setRGB(1,.65+E*.28,.48+E*.4)),x&&(x.intensity=.3+E*1.3),v.material.uniforms.time.value=L?0:V,v.material.uniforms.coverage.value=i==="sky-cloudy"?.65:i==="sky-storm"?.85:.38,v.material.uniforms.night.value=te,v.visible=!0,v.material.uniforms.night.value=i==="sky-space"?1.5:te,y.fog&&y.fog.color.setRGB(.12+E*.45,.16+E*.48,.24+E*.45)}let w=[...m.keys()].find(s=>["rain-light","rain-heavy","snow","wind-dust","wind-leaves"].includes(s));if(w){U+=r*(w==="rain-heavy"?650:w==="rain-light"?220:w==="snow"?75:25)*[.3,.65,1][B]*Math.min(3,m.get(w).intensity??1);let s=Math.min(32,Math.floor(U));for(U=Math.min(1,U-s);s-- >0;)he(w,m.get(w),[R.x+(C()-.5)*34,R.y+8+C()*7,R.z+(C()-.5)*34])}for(let s of["fire","smoke","embers","engine-trail","fog-ground","fog-zone"])if(m.has(s)&&(s.startsWith("fog")||m.get(s).position)&&C()<r*18*Math.min(3,m.get(s).intensity??1)){let i=m.get(s),T=s.startsWith("fog"),A=i.position||[R.x,.5,R.z],E=s==="fog-zone"?i.radius||12:18;he(s,{...i,scale:T?5*(i.scale||1):i.scale,lifetime:T?8:i.lifetime||2},T?[A[0]+(C()-.5)*E*2,A[1],A[2]+(C()-.5)*E*2]:A)}m.has("thunderstorm")&&V>Z&&(z=!0,l&&!L&&(l.intensity=5),Z=V+5+C()*12);for(let s=k.length-1;s>=0;s--){let i=k[s];if(i.age+=r,i.age>i.life){k.splice(s,1);continue}if(i.x+=i.vx*r,i.y+=i.vy*r,i.z+=i.vz*r,!i.id.startsWith("rain")&&i.id!=="snow"&&i.kind!==1&&i.kind!==4&&(i.vy-=6*r),i.y<i.floor){i.id.startsWith("rain")&&B>0&&C()<.08&&Ie("water-ripple",{position:[i.x,i.floor+.02,i.z],lifetime:.6,scale:.25}),k.splice(s,1);continue}}for(let s=0;s<k.length;s++){let i=k[s];I.set([i.x,i.y,i.z],s*3),se.set(i.c,s*3),ue[s]=i.size*(i.kind===1?1+i.age*.2:1),Q[s]=(i.id.startsWith("rain")?Math.min(1,i.age*8):1)*(1-i.age/i.life),W[s]=i.kind}Y.setDrawRange(0,k.length);for(let s of Object.values(Y.attributes))s.needsUpdate=!0;for(let s=_.length-1;s>=0;s--){let i=_[s];i.age+=r,i.mesh.material.opacity=Math.min(1,(i.life-i.age)/Math.min(5,i.life)),i.ripple&&i.mesh.scale.setScalar((1+i.age*7)*i.scale),i.age>i.life&&(i.mesh.removeFromParent(),i.mesh.geometry.dispose(),i.mesh.material.dispose(),i.tex?.dispose(),_.splice(s,1))}for(let s of O)s.age+=r,s.clock.value=L?0:V,s.progress.value=s.id==="dissolve"?Math.min(1,s.age/s.life):s.id==="hit-flash"?Math.max(0,1-s.age*8):0;if(t){t.material.uniforms.time.value=V*t.userData.speed;let s=t.geometry.attributes.position,i=t.userData.base;for(let T=0;T<s.count;T++)s.array[T*3+2]=Math.sin(i[T*3]*.3+V)*Math.cos(i[T*3+1]*.25-V*.6)*(t.userData.id==="water-ocean"?.25:.035);s.needsUpdate=!0,t.geometry.computeVertexNormals(),t.material.uniforms.sunDirection.value.copy(l?.position||G.set(1,1,1)).normalize()}if(g){D.enabled=B>0&&m.has("bloom"),D.strength=.3*(m.get("bloom")?.intensity??1);let s=F.uniforms;s.time.value=V;for(let[i,T]of[["vignette","vignette"],["grain","film-grain"],["heat","heat-haze"],["underwater","underwater"],["grade","color-grade"]])s[i].value=m.has(T)?L&&["heat","underwater"].includes(i)?0:Math.min(2,m.get(T).intensity??1):0}return{thunder:z}}function Ge(){let r=a.getSize(new f.Vector2);g?.setSize(r.x,r.y)}function Ue(){k.length=0;for(let r of _)r.mesh.removeFromParent(),r.mesh.geometry.dispose(),r.mesh.material.dispose(),r.tex?.dispose();_.length=0,Y.setDrawRange(0,0),U=0,Z=6,S=71237;for(let r of O)r.age=0,r.progress.value=0,r.clock.value=0}return{dimension:"3d",set:xe,emit:Ie,update:it,resize:Ge,reset:Ue,clear(){m.clear(),Ue(),y.fog=X.fog,o&&(o.visible=h.visible=v.visible=b.visible=!1),t&&(t.removeFromParent(),t.geometry.dispose(),t.material.dispose(),t.dispose(),u.dispose(),t=null)},quality(r,c){B=r,L=c,g&&g.setPixelRatio(Math.min(a.getPixelRatio(),r===0?.75:r===1?1:1.5))},blood(r){if(oe=r,!oe)for(let c of _)c.ripple||(c.life=0)},render(){g?g.render(0):a.render(y,e)},registerSurface(r,c="ground"){return P.set(r,c),()=>P.delete(r)},applyObject(r,c,z={}){if(!["hologram","dissolve","hit-flash"].includes(c))throw Error("presentation: unsupported object effect");if(L&&c==="hit-flash")return()=>{};d.get(r)?.(),d.size>=128&&d.values().next().value();let w=[];r.traverse(T=>{if(!T.isMesh)return;let A=T.material,E=(Array.isArray(A)?A:[A]).map(te=>{let re=te.clone(),me={material:re,id:c,age:0,life:z.lifetime||2,progress:{value:0},clock:{value:0}};return re.transparent=!0,re.onBeforeCompile=ae=>{ae.uniforms.auraProgress=me.progress,ae.uniforms.auraTime=me.clock,ae.vertexShader=`varying vec3 auraPosition;
`+ae.vertexShader.replace("#include <begin_vertex>",`#include <begin_vertex>
auraPosition=position;`),ae.fragmentShader=`uniform float auraProgress,auraTime;varying vec3 auraPosition;
`+ae.fragmentShader.replace("#include <dithering_fragment>",c==="dissolve"?"float n=fract(sin(dot(floor(auraPosition*35.),vec3(12.9898,78.233,37.719)))*43758.5453);if(n<auraProgress)discard;gl_FragColor.rgb+=vec3(1.,.3,.04)*(1.-smoothstep(0.,.07,n-auraProgress));":c==="hologram"?"float scan=.6+.4*sin(auraPosition.y*90.-auraTime*5.);gl_FragColor=vec4(vec3(.13,.72,1.)*scan,.38+scan*.3);":"gl_FragColor.rgb=mix(gl_FragColor.rgb,vec3(1.),auraProgress);")},re.customProgramCacheKey=()=>"aurago-"+c,O.push(me),w.push(()=>{re.dispose();let ae=O.indexOf(me);ae>=0&&O.splice(ae,1)}),re});T.material=Array.isArray(A)?E:E[0],w.push(()=>{T.material=A})});let s=!1,i=()=>{s||(s=!0,w.forEach(T=>T()),d.delete(r))};return d.set(r,i),i},stats(){return{particles:k.length,decals:_.length,surfaces:P.size,objects:O.length}},dispose(){if(!H){H=!0,Ue(),n.removeFromParent();for(let r of d.values())r();p.forEach(r=>r.dispose()),t&&(t.geometry.dispose(),t.material.dispose(),t.dispose(),u.dispose()),g?.dispose(),D?.dispose(),F?.dispose(),$?.dispose(),P.clear(),y.background=X.background,y.fog=X.fog,l&&X.sun&&(l.color.copy(X.sun.color),l.intensity=X.sun.intensity,l.position.copy(X.sun.position)),x&&(x.intensity=X.ambient)}}}}export{Vt as createPresentation,Di as createThreeAdapter};
