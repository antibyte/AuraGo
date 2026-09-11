import*as c from"./three-0.185.1.module.min.js";import{BackSide as rt,BoxGeometry as at,Mesh as st,ShaderMaterial as nt,UniformsUtils as lt,Vector3 as Qe}from"./three-0.185.1.module.min.js";var Se=class x extends st{constructor(){let e=x.SkyShader,r=new nt({name:e.name,uniforms:lt.clone(e.uniforms),vertexShader:e.vertexShader,fragmentShader:e.fragmentShader,side:rt,depthWrite:!1});super(new at(1,1,1),r),this.isSky=!0}};Se.SkyShader={name:"SkyShader",uniforms:{turbidity:{value:2},rayleigh:{value:1},mieCoefficient:{value:.005},mieDirectionalG:{value:.8},sunPosition:{value:new Qe},up:{value:new Qe(0,1,0)},cloudScale:{value:2e-4},cloudSpeed:{value:1e-4},cloudCoverage:{value:.4},cloudDensity:{value:.4},cloudElevation:{value:.5},showSunDisc:{value:1},time:{value:0}},vertexShader:`
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

		}`};import{Color as Ae,FrontSide as ut,HalfFloatType as ct,Matrix4 as Le,Mesh as ft,PerspectiveCamera as ht,Plane as mt,ShaderMaterial as dt,UniformsLib as He,UniformsUtils as je,Vector3 as de,Vector4 as qe,WebGLRenderTarget as pt}from"./three-0.185.1.module.min.js";var ze=class extends ft{constructor(e,r={}){super(e),this.isWater=!0;let n=this,p=r.textureWidth!==void 0?r.textureWidth:512,s=r.textureHeight!==void 0?r.textureHeight:512,h=r.clipBias!==void 0?r.clipBias:0,P=r.alpha!==void 0?r.alpha:1,m=r.time!==void 0?r.time:0,B=r.waterNormals!==void 0?r.waterNormals:null,_=r.sunDirection!==void 0?r.sunDirection:new de(.70707,.70707,0),L=new Ae(r.sunColor!==void 0?r.sunColor:16777215),f=new Ae(r.waterColor!==void 0?r.waterColor:8355711),V=r.eye!==void 0?r.eye:new de(0,0,0),O=r.distortionScale!==void 0?r.distortionScale:20,oe=r.side!==void 0?r.side:ut,I=r.fog!==void 0?r.fog:!1,X=new mt,E=new de,W=new de,ie=new de,A=new Le,D=new de(0,0,-1),G=new qe,re=new de,K=new de,Y=new qe,J=new Le,H=new ht,ce=new pt(p,s,{type:ct}),$={name:"MirrorShader",uniforms:je.merge([He.fog,He.lights,{normalSampler:{value:null},mirrorSampler:{value:null},alpha:{value:1},time:{value:0},size:{value:1},distortionScale:{value:20},textureMatrix:{value:new Le},sunColor:{value:new Ae(8355711)},sunDirection:{value:new de(.70707,.70707,0)},eye:{value:new de},waterColor:{value:new Ae(5592405)}}]),vertexShader:`
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
				}`},Q=new dt({name:$.name,uniforms:je.clone($.uniforms),vertexShader:$.vertexShader,fragmentShader:$.fragmentShader,lights:!0,side:oe,fog:I});Q.uniforms.mirrorSampler.value=ce.texture,this.dispose=()=>ce.dispose(),Q.uniforms.textureMatrix.value=J,Q.uniforms.alpha.value=P,Q.uniforms.time.value=m,Q.uniforms.normalSampler.value=B,Q.uniforms.sunColor.value=L,Q.uniforms.waterColor.value=f,Q.uniforms.sunDirection.value=_,Q.uniforms.distortionScale.value=O,Q.uniforms.eye.value=V,n.material=Q,n.onBeforeRender=function(T,l,b){if(W.setFromMatrixPosition(n.matrixWorld),ie.setFromMatrixPosition(b.matrixWorld),A.extractRotation(n.matrixWorld),E.set(0,0,1),E.applyMatrix4(A),re.subVectors(W,ie),re.dot(E)>0)return;re.reflect(E).negate(),re.add(W),A.extractRotation(b.matrixWorld),D.set(0,0,-1),D.applyMatrix4(A),D.add(ie),K.subVectors(W,D),K.reflect(E).negate(),K.add(W),H.position.copy(re),H.up.set(0,1,0),H.up.applyMatrix4(A),H.up.reflect(E),H.lookAt(K),H.far=b.far,H.updateMatrixWorld(),H.projectionMatrix.copy(b.projectionMatrix),J.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),J.multiply(H.projectionMatrix),J.multiply(H.matrixWorldInverse),X.setFromNormalAndCoplanarPoint(E,W),X.applyMatrix4(H.matrixWorldInverse),G.set(X.normal.x,X.normal.y,X.normal.z,X.constant);let w=H.projectionMatrix;Y.x=(Math.sign(G.x)+w.elements[8])/w.elements[0],Y.y=(Math.sign(G.y)+w.elements[9])/w.elements[5],Y.z=-1,Y.w=(1+w.elements[10])/w.elements[14],G.multiplyScalar(2/G.dot(Y)),w.elements[2]=G.x,w.elements[6]=G.y,w.elements[10]=G.z+1-h,w.elements[14]=G.w,V.setFromMatrixPosition(b.matrixWorld);let v=T.getRenderTarget(),d=T.xr.enabled,z=T.shadowMap.autoUpdate;n.visible=!1,T.xr.enabled=!1,T.shadowMap.autoUpdate=!1,T.setRenderTarget(ce),T.state.buffers.depth.setMask(!0),T.autoClear===!1&&T.clear(),T.render(l,H),n.visible=!0,T.xr.enabled=d,T.shadowMap.autoUpdate=z,T.setRenderTarget(v);let j=b.viewport;j!==void 0&&T.state.viewport(j)}}};import{HalfFloatType as Mt,NoBlending as Tt,Timer as St,Vector2 as Ze,WebGLRenderTarget as Ct}from"./three-0.185.1.module.min.js";var we={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};import{ShaderMaterial as Ke,UniformsUtils as bt}from"./three-0.185.1.module.min.js";import{BufferGeometry as gt,Float32BufferAttribute as Xe,OrthographicCamera as vt,Mesh as xt}from"./three-0.185.1.module.min.js";var ue=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},yt=new vt(-1,1,1,-1,0,1),We=class extends gt{constructor(){super(),this.setAttribute("position",new Xe([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new Xe([0,2,0,0,2,0],2))}},wt=new We,pe=class{constructor(e){this._mesh=new xt(wt,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,yt)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var be=class extends ue{constructor(e,r="tDiffuse"){super(),this.textureID=r,this.uniforms=null,this.material=null,e instanceof Ke?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=bt.clone(e.uniforms),this.material=new Ke({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new pe(this.material)}render(e,r,n){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=n.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(r),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ce=class extends ue{constructor(e,r){super(),this.scene=e,this.camera=r,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,r,n){let p=e.getContext(),s=e.state;s.buffers.color.setMask(!1),s.buffers.depth.setMask(!1),s.buffers.color.setLocked(!0),s.buffers.depth.setLocked(!0);let h,P;this.inverse?(h=0,P=1):(h=1,P=0),s.buffers.stencil.setTest(!0),s.buffers.stencil.setOp(p.REPLACE,p.REPLACE,p.REPLACE),s.buffers.stencil.setFunc(p.ALWAYS,h,4294967295),s.buffers.stencil.setClear(P),s.buffers.stencil.setLocked(!0),e.setRenderTarget(n),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(r),this.clear&&e.clear(),e.render(this.scene,this.camera),s.buffers.color.setLocked(!1),s.buffers.depth.setLocked(!1),s.buffers.color.setMask(!0),s.buffers.depth.setMask(!0),s.buffers.stencil.setLocked(!1),s.buffers.stencil.setFunc(p.EQUAL,1,4294967295),s.buffers.stencil.setOp(p.KEEP,p.KEEP,p.KEEP),s.buffers.stencil.setLocked(!0)}},ke=class extends ue{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Re=class{constructor(e,r){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),r===void 0){let n=e.getSize(new Ze);this._width=n.width,this._height=n.height,r=new Ct(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Mt}),r.texture.name="EffectComposer.rt1"}else this._width=r.width,this._height=r.height;this.renderTarget1=r,this.renderTarget2=r.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new be(we),this.copyPass.material.blending=Tt,this.timer=new St}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,r){this.passes.splice(r,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let r=this.passes.indexOf(e);r!==-1&&this.passes.splice(r,1)}isLastEnabledPass(e){for(let r=e+1;r<this.passes.length;r++)if(this.passes[r].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let r=this.renderer.getRenderTarget(),n=!1;for(let p=0,s=this.passes.length;p<s;p++){let h=this.passes[p];if(h.enabled!==!1){if(h.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(p),h.render(this.renderer,this.writeBuffer,this.readBuffer,e,n),h.needsSwap){if(n){let P=this.renderer.getContext(),m=this.renderer.state.buffers.stencil;m.setFunc(P.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),m.setFunc(P.EQUAL,1,4294967295)}this.swapBuffers()}Ce!==void 0&&(h instanceof Ce?n=!0:h instanceof ke&&(n=!1))}}this.renderer.setRenderTarget(r)}reset(e){if(e===void 0){let r=this.renderer.getSize(new Ze);this._pixelRatio=this.renderer.getPixelRatio(),this._width=r.width,this._height=r.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,r){this._width=e,this._height=r;let n=this._width*this._pixelRatio,p=this._height*this._pixelRatio;this.renderTarget1.setSize(n,p),this.renderTarget2.setSize(n,p);for(let s=0;s<this.passes.length;s++)this.passes[s].setSize(n,p)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};import{Color as _t}from"./three-0.185.1.module.min.js";var De=class extends ue{constructor(e,r,n=null,p=null,s=null){super(),this.scene=e,this.camera=r,this.overrideMaterial=n,this.clearColor=p,this.clearAlpha=s,this.clear=!0,this.clearDepth=!1,this.needsSwap=!1,this.isRenderPass=!0,this._oldClearColor=new _t}render(e,r,n){let p=e.autoClear;e.autoClear=!1;let s,h;this.overrideMaterial!==null&&(h=this.scene.overrideMaterial,this.scene.overrideMaterial=this.overrideMaterial),this.clearColor!==null&&(e.getClearColor(this._oldClearColor),e.setClearColor(this.clearColor,e.getClearAlpha())),this.clearAlpha!==null&&(s=e.getClearAlpha(),e.setClearAlpha(this.clearAlpha)),this.clearDepth==!0&&e.clearDepth(),e.setRenderTarget(this.renderToScreen?null:n),this.clear===!0&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),e.render(this.scene,this.camera),this.clearColor!==null&&e.setClearColor(this._oldClearColor),this.clearAlpha!==null&&e.setClearAlpha(s),this.overrideMaterial!==null&&(this.scene.overrideMaterial=h),e.autoClear=p}};import{AdditiveBlending as Et,Color as $e,HalfFloatType as Ve,MeshBasicMaterial as At,ShaderMaterial as Fe,UniformsUtils as Je,Vector2 as ge,Vector3 as _e,WebGLRenderTarget as Oe}from"./three-0.185.1.module.min.js";import{Color as Pt}from"./three-0.185.1.module.min.js";var Ye={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Pt(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var Me=class x extends ue{constructor(e,r=1,n,p){super(),this.strength=r,this.radius=n,this.threshold=p,this.resolution=e!==void 0?new ge(e.x,e.y):new ge(256,256),this.clearColor=new $e(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let s=Math.round(this.resolution.x/2),h=Math.round(this.resolution.y/2);this.renderTargetBright=new Oe(s,h,{type:Ve}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let _=0;_<this.nMips;_++){let L=new Oe(s,h,{type:Ve});L.texture.name="UnrealBloomPass.h"+_,L.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(L);let f=new Oe(s,h,{type:Ve});f.texture.name="UnrealBloomPass.v"+_,f.texture.generateMipmaps=!1,this.renderTargetsVertical.push(f),s=Math.round(s/2),h=Math.round(h/2)}let P=Ye;this.highPassUniforms=Je.clone(P.uniforms),this.highPassUniforms.luminosityThreshold.value=p,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new Fe({uniforms:this.highPassUniforms,vertexShader:P.vertexShader,fragmentShader:P.fragmentShader}),this.separableBlurMaterials=[];let m=[6,10,14,18,22];s=Math.round(this.resolution.x/2),h=Math.round(this.resolution.y/2);for(let _=0;_<this.nMips;_++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(m[_])),this.separableBlurMaterials[_].uniforms.invSize.value=new ge(1/s,1/h),s=Math.round(s/2),h=Math.round(h/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=r,this.compositeMaterial.uniforms.bloomRadius.value=.1;let B=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=B,this.bloomTintColors=[new _e(1,1,1),new _e(1,1,1),new _e(1,1,1),new _e(1,1,1),new _e(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=Je.clone(we.uniforms),this.blendMaterial=new Fe({uniforms:this.copyUniforms,vertexShader:we.vertexShader,fragmentShader:we.fragmentShader,premultipliedAlpha:!0,blending:Et,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new $e,this._oldClearAlpha=1,this._basic=new At,this._fsQuad=new pe(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,r){let n=Math.round(e/2),p=Math.round(r/2);this.renderTargetBright.setSize(n,p);for(let s=0;s<this.nMips;s++)this.renderTargetsHorizontal[s].setSize(n,p),this.renderTargetsVertical[s].setSize(n,p),this.separableBlurMaterials[s].uniforms.invSize.value=new ge(1/n,1/p),n=Math.round(n/2),p=Math.round(p/2)}render(e,r,n,p,s){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let h=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),s&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=n.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=n.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let P=this.renderTargetBright;for(let m=0;m<this.nMips;m++)this._fsQuad.material=this.separableBlurMaterials[m],this.separableBlurMaterials[m].uniforms.colorTexture.value=P.texture,this.separableBlurMaterials[m].uniforms.direction.value=x.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[m]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[m].uniforms.colorTexture.value=this.renderTargetsHorizontal[m].texture,this.separableBlurMaterials[m].uniforms.direction.value=x.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[m]),e.clear(),this._fsQuad.render(e),P=this.renderTargetsVertical[m];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,s&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(n),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=h}_getSeparableBlurMaterial(e){let r=[],n=e/3;for(let p=0;p<e;p++)r.push(.39894*Math.exp(-.5*p*p/(n*n))/n);return new Fe({defines:{KERNEL_RADIUS:e},uniforms:{colorTexture:{value:null},invSize:{value:new ge(.5,.5)},direction:{value:new ge(.5,.5)},gaussianCoefficients:{value:r}},vertexShader:`

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

				}`})}};Me.BlurDirectionX=new ge(1,0);Me.BlurDirectionY=new ge(0,1);import{ColorManagement as zt,RawShaderMaterial as kt,UniformsUtils as Rt,LinearToneMapping as Dt,ReinhardToneMapping as Ft,CineonToneMapping as Bt,AgXToneMapping as Ut,ACESFilmicToneMapping as Nt,NeutralToneMapping as Lt,CustomToneMapping as Wt,SRGBTransfer as Vt}from"./three-0.185.1.module.min.js";var Pe={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var Be=class extends ue{constructor(){super(),this.isOutputPass=!0,this.uniforms=Rt.clone(Pe.uniforms),this.material=new kt({name:Pe.name,uniforms:this.uniforms,vertexShader:Pe.vertexShader,fragmentShader:Pe.fragmentShader}),this._fsQuad=new pe(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,r,n){this.uniforms.tDiffuse.value=n.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},zt.getTransfer(this._outputColorSpace)===Vt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Dt?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Ft?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Bt?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Nt?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Ut?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Lt?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Wt&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(r),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};function et({sounds:x=[],base:e,root:r,report:n=console.warn,dimension:p="3d"}){let s=new Map(x.map(t=>[t.id,t])),h=new Map,P=new Map,m=new Set,B=new Map,_=new Map,L=new AbortController,f,V,O,oe,I,X,E=!1,W=!1,ie=!1,A=.55,D={},G=[0,0,0],re=new Set,K=new Set,Y="outside",J=0;function H(){if(!(f||E)){f=new AudioContext({sampleRate:48e3}),V=f.createGain(),O=f.createDynamicsCompressor(),O.threshold.value=-12,O.knee.value=12,O.ratio.value=8,O.attack.value=.003,O.release.value=.2,V.connect(O).connect(f.destination);for(let t of["effects","ambience","music"])D[t]=f.createGain(),D[t].gain.value=t==="ambience"?.45:1,D[t].connect(V);I=f.createConvolver(),oe=f.createGain(),X=f.createBiquadFilter(),X.type="lowpass",X.frequency.value=8e3,I.connect(X).connect(oe).connect(V),$(Y),ce()}}function ce(){V&&V.gain.setTargetAtTime(ie||W?0:A*A,f.currentTime,.03)}function $(t){if(!["outside","small-room","hall"].includes(t))throw Error("audio: unknown room");if(Y=t,!f)return;oe.gain.value=t==="outside"?0:t==="hall"?.2:.1;let g=Math.floor(f.sampleRate*(t==="hall"?1.7:.35)),y=f.createBuffer(2,g,f.sampleRate),U=1973;for(let N=0;N<2;N++){let Z=y.getChannelData(N);for(let S=0;S<g;S++)U=Math.imul(U,1664525)+1013904223>>>0,Z[S]=(U/2147483648-1)*Math.pow(1-S/g,3)}I.buffer=y}async function Q(t){if(h.has(t))return h.get(t);if(P.has(t))return P.get(t);let g=s.get(t);if(!g)throw Error("audio: unknown imported sound "+t);let y=(async()=>{let U=g.files?.[0];if(!U||!U.file.startsWith("sounds/")||!/^sounds\/[a-z0-9-]+\.wav$/.test(U.file))throw Error("audio: invalid sample path");let N=new URL(e,document.baseURI),Z=new URL(U.file,N);if(Z.origin!==new URL(document.baseURI).origin||!Z.pathname.startsWith(N.pathname))throw Error("audio: sample outside game");let S=await fetch(Z,{signal:L.signal});if(!S.ok)throw Error("audio: "+t+" HTTP "+S.status);let ne=await S.arrayBuffer();if(ne.byteLength!==U.bytes)throw Error("audio: size mismatch "+t);if(crypto.subtle&&[...new Uint8Array(await crypto.subtle.digest("SHA-256",ne))].map(ee=>ee.toString(16).padStart(2,"0")).join("")!==U.sha256)throw Error("audio: checksum mismatch "+t);let le=await f.decodeAudioData(ne);return E?null:(h.set(t,le),le)})();P.set(t,y);try{return await y}finally{P.delete(t)}}function T(t){if(m.has(t)){m.delete(t);try{t.source.stop()}catch{}t.source.disconnect(),t.gain.disconnect(),t.pan?.disconnect()}}function l(t,g=.18){if(!(!m.has(t)||t.fading)){t.fading=!0,t.gain.gain.setTargetAtTime(0,f.currentTime,g/3);try{t.source.stop(f.currentTime+g)}catch{}}}function b(t){!t.position||!t.pan?.pan||t.fading||(t.pan.pan.value=Math.max(-1,Math.min(1,(t.position[0]-G[0])/450)),t.gain.gain.setTargetAtTime(t.volume/(1+Math.hypot(t.position[0]-G[0],t.position[1]-G[1])/500),f.currentTime,.03))}function w(t,g,y={}){if(!g||E||W||ie||f.state!=="running")return null;let U=s.get(t),N=y.loop??U?.loop??!1,Z=y.bus||(N&&t!=="engine"?"ambience":"effects");if(!D[Z])throw Error("audio: unknown bus");let S=N&&[...m].find(fe=>fe.loop&&fe.id===t&&!fe.fading);if(S)return y.position&&(S.position=y.position.slice(),S.pan?.positionX?[S.pan.positionX.value,S.pan.positionY.value,S.pan.positionZ.value]=y.position:b(S)),S;if(Z==="ambience"&&[...m].filter(fe=>fe.bus==="ambience").length>=4)return null;if(m.size>=32){let fe=[...m].filter(he=>!he.loop).sort((he,Ee)=>he.priority-Ee.priority||he.start-Ee.start)[0];if(!fe||fe.priority>(y.priority??1))return null;T(fe)}let ne=f.createBufferSource(),le=f.createGain();ne.buffer=g,ne.loop=N,ne.playbackRate.value=y.rate??(N?1:1+(Math.random()-.5)*.06);let ve=Math.min(1,Math.max(0,y.gain??U?.gain??.6));le.gain.setValueAtTime(N?0:ve,f.currentTime),N&&le.gain.linearRampToValueAtTime(ve,f.currentTime+.3);let ee;ne.connect(le),y.position?(p==="3d"?(ee=f.createPanner(),ee.panningModel="HRTF",ee.distanceModel="inverse",ee.refDistance=2,ee.maxDistance=80,ee.rolloffFactor=1.3,[ee.positionX.value,ee.positionY.value,ee.positionZ.value]=y.position):ee=f.createStereoPanner(),le.connect(ee).connect(D[Z])):le.connect(D[Z]),Z==="effects"&&(ee||le).connect(I);let xe={id:t,source:ne,gain:le,pan:ee,loop:N,bus:Z,volume:ve,position:y.position?.slice(),priority:y.priority??1,start:f.currentTime};return p==="2d"&&ee&&(le.gain.cancelScheduledValues(f.currentTime),b(xe)),m.add(xe),ne.onended=()=>T(xe),ne.start(),xe}async function v(t,g={}){if(!s.has(t))return n("audio: unknown imported sound "+t),null;if(!f||E||W||ie||f.state!=="running")return null;if(g.position&&(!Array.isArray(g.position)||g.position.length!==3||g.position.some(S=>!Number.isFinite(S))))throw Error("audio: invalid position");for(let S of["rate","gain","cooldown","priority"])if(g[S]!==void 0&&!Number.isFinite(g[S]))throw Error("audio: invalid "+S);let y=J,U=_.get(t),N=f.currentTime;if(N-(B.get(t)??-99)<(g.cooldown??.045))return null;B.set(t,N);let Z=g.loop??s.get(t).loop;try{let S=await Q(t);return y!==J||U!==_.get(t)||!Z&&f.currentTime-N>.2||g.ambient&&!d.includes(t)?null:w(t,S,g)}catch(S){return!E&&S.name!=="AbortError"&&!K.has(t)&&(K.add(t),n(String(S))),null}}let d=[];async function z(t){d=[...new Set(t)].slice(0,4);let g=new Set(d);for(let y of m)y.bus==="ambience"&&!g.has(y.id)&&l(y);for(let y of d)!re.has(y)&&![...m].some(U=>U.loop&&U.id===y&&!U.fading)&&(re.add(y),v(y,{loop:!0,ambient:!0,bus:"ambience"}).finally(()=>re.delete(y)))}async function j(t){if(!(!t.isTrusted||E||W)){H();try{await f.resume(),await Promise.all([...s.keys()].map(g=>Q(g).catch(y=>{!K.has(g)&&y.name!=="AbortError"&&(K.add(g),n("audio: "+y.message))}))),E||await z(d)}catch(g){E||n("audio: "+g.message)}}}return r.addEventListener("pointerdown",j,{signal:L.signal}),r.addEventListener("keydown",j,{signal:L.signal}),{play:v,ambience:z,setRoom:$,unlock:j,stop(t){if(s.has(t)){_.set(t,(_.get(t)||0)+1);for(let g of[...m])g.id===t&&T(g)}},setVolume(t){A=Math.max(0,Math.min(1,Number.isFinite(t)?t:.55)),ce()},setMuted(t){if(ie=!!t,J++,ie)for(let g of[...m])T(g);ce(),ie||z(d)},setPaused(t){if(W!==!!t){if(W=!!t,J++,W){for(let g of[...m])T(g);f?.suspend().catch(()=>{})}else f&&f.resume().then(()=>z(d)).catch(()=>{});ce()}},listener(t,g=[0,0,-1],y=[0,1,0]){if(G.splice(0,3,...t),!f)return;if(p==="2d")for(let N of m)b(N);let U=f.listener;for(let[N,Z]of[["position",t],["forward",g],["up",y]])for(let S=0;S<3;S++)U[N+"XYZ"[S]]&&(U[N+"XYZ"[S]].value=Z[S])},connectMusic(t){if(!f)throw Error("audio: unlock with player interaction first");if(t.context!==f)throw Error("audio: music must use the game AudioContext");return t.connect(D.music),()=>t.disconnect(D.music)},get context(){return f},reset(){J++;for(let t of[...m])T(t);B.clear(),z(d)},stats(){return{voices:m.size,loops:[...m].filter(t=>t.loop).length,buffers:h.size,state:f?.state||"locked"}},dispose(){if(!E){E=!0,L.abort();for(let t of[...m])T(t);h.clear(),P.clear(),I?.disconnect(),X?.disconnect(),oe?.disconnect(),Object.values(D).forEach(t=>t.disconnect()),V?.disconnect(),O?.disconnect(),f?.close().catch(()=>{})}}}}var tt=(x,e,r)=>Math.max(e,Math.min(r,x));function Ot({adapter:x,config:e,root:r,report:n=console.warn,controls:p=!0}){e||={effects:[],sounds:[],bindings:[],quality:"auto"};let s=new Map((e.effects||[]).map(l=>[l.id,l])),h=new Map,P=new AbortController,m=et({sounds:e.sounds,base:e.base,root:r,report:n,dimension:x.dimension}),B=new Map;for(let l of e.bindings||[])B.set(l.event,[...B.get(l.event)||[],l.sound]);let _=e.quality||"auto",L=2,f=0,V=!1,O=document.hidden,oe=!1,I=0,X=0,E=!0,W=0,ie=matchMedia("(prefers-reduced-motion: reduce)"),A=ie.matches;function D(){x.quality(L,A)}function G(l,b={}){let w=s.get(l);if(!w)throw Error("presentation: unknown imported effect "+l);let v={...w.defaults};for(let[d,z]of Object.entries(b))if(["position","normal"].includes(d)){if(!Array.isArray(z)||z.length!==3||z.some(j=>!Number.isFinite(j)||Math.abs(j)>1e6))throw Error("presentation: invalid "+d);v[d]=z.slice()}else if(d==="color"){if(!/^#[a-f0-9]{6}$/i.test(z))throw Error("presentation: invalid color");v[d]=z}else if(d==="fixed"){if(typeof z!="boolean")throw Error("presentation: invalid fixed");v[d]=z}else if(["intensity","scale","lifetime","cycle","hour","width","depth","y","speed","density","radius"].includes(d)){if(!Number.isFinite(z))throw Error("presentation: invalid "+d);v[d]=tt(z,d==="y"?-1e4:0,d==="cycle"?86400:d==="width"||d==="depth"?1e3:d==="hour"?24:d==="lifetime"?120:100)}else throw Error("presentation: unknown parameter "+d);return v}function re(l,b={}){let w=G(l,b);return h.set(l,w),x.set(l,w),Q}function K(l,b={}){if(V||O||oe)return;let w=G(l,b);if(l.startsWith("blood")&&!E){s.has("metal-sparks")&&x.emit("metal-sparks",w);return}x.emit(l,w)}let Y={shot:["muzzle-flash"],hit:["blood-spray","blood-pool","blood-decal","metal-sparks","stone-debris","hit-flash"],pickup:["pickup-glow"],splash:["water-splash","water-ripple"],win:["magic"]};function J(l,b=[0,0,0],w=[0,1,0],v="flesh"){if(V||O||oe)return;for(let j of Y[l]||[])s.has(j)&&(!j.startsWith("blood")||v==="flesh")&&(j!=="metal-sparks"||v==="metal")&&(j!=="stone-debris"||v==="stone")&&K(j,{position:b,normal:w});let d=B.get(l),z=d?.[Math.floor(Math.random()*d.length)];z&&m.play(z,{position:x.dimension==="2d"?[b[0],b[1],0]:b})}function H(){m.setPaused(V||O)}P.signal.addEventListener("abort",()=>m.dispose(),{once:!0}),document.addEventListener("visibilitychange",()=>{O=document.hidden,H()},{signal:P.signal}),ie.addEventListener("change",l=>{A=l.matches,D()},{signal:P.signal});let $=(s.get(e.environment)?.sounds||[]).filter(l=>e.sounds?.find(b=>b.id===l)?.loop);B.has("ambient")&&$.push(...B.get("ambient"));let Q={audio:m,set:re,emit:K,event:J,setEnvironment(l){let b=s.get(l);if(b?.category!=="environment")throw Error("presentation: environment was not imported");h.clear(),x.clear?.();for(let w of b.effects)re(w);return $=b.sounds.filter(w=>e.sounds?.find(v=>v.id===w)?.loop),m.ambience($),Q},setPaused(l){V=!!l,H()},setActive(l){O=!l||document.hidden,H()},setBlood(l){E=!!l,x.blood?.(E)},setQuality(l){if(!["auto","low","medium","high"].includes(l))throw Error("presentation: unknown quality");_=l,L=l==="low"?0:l==="medium"?1:2,I=X=0,D()},registerSurface(...l){return x.registerSurface(...l)},applyObject(l,b,w){return x.applyObject(l,b,G(b,w))},update(l){if(V||O||oe)return;let b=tt(l,0,.1);f+=b,_==="auto"&&(l>.025?(I+=b,X=0):(X+=b,I=Math.max(0,I-b)),I>2&&L>0&&(L--,I=0,D()),X>12&&L<2&&(L++,X=0,D())),x.update(b,f)?.thunder&&(W=f+1.2),W&&f>=W&&(W=0,e.sounds?.some(v=>v.id==="thunder")&&m.play("thunder",{gain:.4}));let w=h.get("day-night");if(w&&$.includes("forest-day")&&$.includes("forest-night")){let v=w.fixed?w.hour:(w.hour+f*24/Math.max(1,w.cycle))%24;m.ambience([v>6&&v<19?"forest-day":"forest-night",...$.filter(d=>!d.startsWith("forest-"))])}else m.ambience($)},render(){x.render?.()},resize(){x.resize?.()},reset(){f=W=0,x.reset(),m.reset(),I=X=0},stats(){return{...x.stats(),...m.stats(),quality:["low","medium","high"][L],time:f}},dispose(){oe||(oe=!0,P.abort(),T?.remove(),x.dispose())}},T;if(p&&(s.size||e.sounds?.length)){T=document.createElement("div"),T.className="aurago-game-presentation",T.style.cssText="position:absolute;right:12px;top:12px;z-index:1100;display:flex;gap:8px;align-items:center;padding:7px 10px;border:1px solid #ffffff30;border-radius:12px;background:#101822d9;color:#fff;font:12px system-ui;max-width:90%";let l=document.createElement("button");l.textContent="\u266A",l.title="Sound",l.setAttribute("aria-label","Mute sound"),l.setAttribute("aria-pressed","false");let b=!1;l.onclick=()=>{b=!b,l.textContent=b?"\u266B \xD7":"\u266A",l.setAttribute("aria-pressed",String(b)),m.setMuted(b)};let w=document.createElement("input");w.type="range",w.min="0",w.max="100",w.value="55",w.style.width="68px",w.setAttribute("aria-label","Volume"),w.oninput=()=>m.setVolume(+w.value/100);let v=document.createElement("select");v.setAttribute("aria-label","Effects quality");for(let d of["auto","low","medium","high"])v.add(new Option(d,d));if(v.value=_,v.onchange=()=>Q.setQuality(v.value),T.append(l,w,v),[...s.keys()].some(d=>d.startsWith("blood"))){let d=document.createElement("button");d.textContent="\u25CF",d.title="Blood effects",d.setAttribute("aria-label","Blood effects"),d.setAttribute("aria-pressed","true"),d.onclick=()=>{Q.setBlood(!E),d.setAttribute("aria-pressed",String(E))},T.append(d)}T.style.colorScheme="dark",T.style.accentColor="#71dfce",T.style.flexWrap="wrap";for(let d of T.querySelectorAll("button,select"))d.style.cssText="font:inherit;color:inherit;border:1px solid #ffffff28;border-radius:7px;background:#243443;padding:6px;min-height:"+(matchMedia("(pointer: coarse)").matches?44:32)+"px";r.append(T)}for(let l of s.values())l.category!=="environment"&&re(l.id);return Q.setQuality(_),m.ambience($),H(),Q}var It=`attribute float size;attribute float alpha;attribute float kind;varying vec3 tint;varying float opacity;varying float shape;
void main(){tint=color;opacity=alpha;shape=kind;vec4 p=modelViewMatrix*vec4(position,1.);gl_Position=projectionMatrix*p;gl_PointSize=clamp(size*650./max(1.,-p.z),1.,kind<.5?34.:160.);}`,Gt=`varying vec3 tint;varying float opacity;varying float shape;void main(){vec2 p=gl_PointCoord*2.-1.;float a;
if(shape<.5){p.x*=6.;a=(1.-smoothstep(.35,1.,length(p)))*.5;}
else if(shape<1.5){a=(1.-smoothstep(.05,1.,length(p)))*.5;}
else if(shape<2.5){a=1.-smoothstep(.65,1.,length(p));}
else if(shape<3.5){p.y*=1.8;a=1.-smoothstep(.5,1.,length(p));}
else {p.y*=3.;a=(1.-smoothstep(0.,1.,length(p)))*.16;}
if(a*opacity<.015)discard;gl_FragColor=vec4(tint,a*opacity);}`,Qt={uniforms:{tDiffuse:{value:null},time:{value:0},vignette:{value:0},grain:{value:0},heat:{value:0},underwater:{value:0},grade:{value:0}},vertexShader:"varying vec2 uv0;void main(){uv0=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`uniform sampler2D tDiffuse;uniform float time,vignette,grain,heat,underwater,grade;varying vec2 uv0;
void main(){vec2 p=uv0;p.x+=(sin(p.y*32.+time*2.)*.002+sin(p.y*77.-time)*.001)*(heat+underwater);vec3 c=texture2D(tDiffuse,p).rgb;
c=mix(c,c*vec3(.55,.86,1.04),underwater*.6);c=mix(c,pow(max(c,vec3(0.)),vec3(.97))*vec3(1.035,1.01,.96),grade*.4);
c*=1.-vignette*.6*smoothstep(.15,.8,length(uv0-.5));c+=(fract(sin(dot(uv0+time,vec2(12.9898,78.233)))*43758.5453)-.5)*grain*.035;gl_FragColor=vec4(c,1.);}`};function Di({scene:x,camera:e,renderer:r,sun:n,ambient:p}){let s=new c.Group;s.name="AuraGo presentation",x.add(s);let h=new Map,P=new Map,m=[],B=[],_=[],L=[],f=new Map,V=2,O=!1,oe=!0,I=0,X=!1,E=71237,W=0,ie=6,A=()=>(E^=E<<13,E^=E>>>17,E^=E<<5,(E>>>0)/4294967296),D=new c.Vector3,G=new c.Vector3,re=new c.Raycaster,K=new c.Vector3,Y={background:x.background,fog:x.fog,sun:n&&{color:n.color.clone(),intensity:n.intensity,position:n.position.clone()},ambient:p?.intensity},J=4e3,H=new Float32Array(J*3),ce=new Float32Array(J*3),$=new Float32Array(J),Q=new Float32Array(J),T=new Float32Array(J),l=new c.BufferGeometry;for(let[o,u,F]of[["position",H,3],["color",ce,3],["size",$,1],["alpha",Q,1],["kind",T,1]])l.setAttribute(o,new c.BufferAttribute(u,F).setUsage(c.DynamicDrawUsage));l.setDrawRange(0,0);let b=new c.ShaderMaterial({vertexShader:It,fragmentShader:Gt,vertexColors:!0,transparent:!0,depthWrite:!1}),w=new c.Points(l,b);w.frustumCulled=!1,s.add(w),m.push(l,b);let v,d,z,j,t,g,y,U,N,Z,S=new c.Color,ne=new c.Color;function le(){if(v)return;v=new Se,v.scale.setScalar(210),v.material.uniforms.cloudCoverage.value=0,v.material.uniforms.auraSkyColor={value:new c.Color(.42,.62,.78)},v.material.fragmentShader=`uniform vec3 auraSkyColor;
`+v.material.fragmentShader.replace("gl_FragColor = vec4( texColor, 1.0 );","vec3 d=normalize(vWorldPosition-cameraPosition);vec3 atmosphere=mix(auraSkyColor,auraSkyColor*vec3(.1,.28,.52),pow(max(0.,d.y),.45));gl_FragColor = vec4(mix(atmosphere,clamp(texColor*.035,0.,1.),.3),1.0);"),s.add(v),m.push(v.geometry,v.material);let o=new c.BufferGeometry,u=new Float32Array(1200*3);for(let R=0;R<1200;R++)G.set(A()-.5,A()-.5,A()-.5).normalize().multiplyScalar(190),u.set(G.toArray(),R*3);o.setAttribute("position",new c.BufferAttribute(u,3));let F=new c.PointsMaterial({color:13164287,size:.65,transparent:!0,depthWrite:!1,fog:!1});d=new c.Points(o,F),s.add(d),m.push(o,F);let M=new c.SphereGeometry(4,24,16),a=new c.MeshBasicMaterial({color:14147815,fog:!1});j=new c.Mesh(M,a),j.position.set(-80,100,-120),s.add(j),m.push(M,a);let i=new c.SphereGeometry(205,32,16),C=new c.ShaderMaterial({side:c.BackSide,transparent:!0,depthWrite:!1,uniforms:{time:{value:0},coverage:{value:.5},night:{value:0}},vertexShader:"varying vec3 p;void main(){p=position;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`varying vec3 p;uniform float time,coverage,night;
float hash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}float noise(vec2 x){vec2 i=floor(x),f=fract(x);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1,0)),f.x),mix(hash(i+vec2(0,1)),hash(i+1.),f.x),f.y);}
void main(){vec3 d=normalize(p);vec2 uv=night>1.1?vec2(atan(d.z,d.x),asin(d.y))*3.:d.xz/max(.12,d.y)*1.7+vec2(time*.015,0);float n=0.,a=.55;for(int i=0;i<5;i++){n+=a*noise(uv);uv=uv*2.03+.71;a*=.5;}float c=smoothstep(1.-coverage,1.15-coverage,n)*(night>1.1?1.:smoothstep(0.,.2,d.y));gl_FragColor=night>1.1?vec4(mix(vec3(.05,.12,.3),vec3(.3,.07,.4),n),c*.45):vec4(mix(vec3(.78,.87,.95),vec3(.09,.12,.18),night),c*.42);}`});z=new c.Mesh(i,C),s.add(z),m.push(i,C)}function ve(){y||(y=new Re(r),y.addPass(new De(x,e)),U=new Me(new c.Vector2(512,512),.3,.35,.8),y.addPass(U),N=new be(Qt),y.addPass(N),Z=new Be,y.addPass(Z),Ge())}function ee(o,u){t&&(s.remove(t),t.geometry.dispose(),t.material.dispose(),t.dispose(),g.dispose());let F=new Uint8Array(4096*4);for(let i=0;i<4096;i++)F[i*4]=128+Math.sin(i*.37)*32,F[i*4+1]=128+Math.cos(i*.51)*32,F[i*4+2]=245,F[i*4+3]=255;g=new c.DataTexture(F,64,64),g.wrapS=g.wrapT=c.RepeatWrapping,g.magFilter=g.minFilter=c.LinearFilter,g.needsUpdate=!0;let M=new c.PlaneGeometry(u.width||80,u.depth||80,32,32);t=new ze(M,{textureWidth:512,textureHeight:512,waterNormals:g,sunDirection:new c.Vector3(1,1,1),sunColor:16772305,waterColor:o==="water-river"?2387301:1204096,distortionScale:o==="water-ocean"?3:1.2,fog:!!x.fog}),t.material.fragmentShader=t.material.fragmentShader.replace("vec3 outgoingLight = albedo;","vec3 outgoingLight = mix(albedo,waterColor,.4);").replace("gl_FragColor = vec4( outgoingLight, alpha );",`
            float wave=sin(worldPosition.x*.32+time)*cos(worldPosition.z*.24-time*.6);
            float foam=smoothstep(.94,.995,wave)*(.3+.7*sin(worldPosition.x*8.+worldPosition.z*9.)*sin(worldPosition.x*8.+worldPosition.z*9.));
            outgoingLight=mix(outgoingLight,vec3(.65,.87,.9),foam*.25);
            gl_FragColor = vec4(outgoingLight, alpha);`),t.rotation.x=-Math.PI/2,t.position.fromArray(u.position||[0,u.y??-.05,20]),t.userData.base=M.attributes.position.array.slice(),t.userData.id=o,t.userData.speed=u.speed||.6;let a=t.onBeforeRender;t.onBeforeRender=function(...i){V===2&&a.apply(this,i)},s.add(t)}function xe(o,u){if(h.set(o,u),o.startsWith("sky-")||o==="day-night"){for(let F of h.keys())o.startsWith("sky-")&&F.startsWith("sky-")&&F!==o&&h.delete(F);le()}["water-lake","water-river","water-ocean"].includes(o)&&ee(o,u),["bloom","color-grade","vignette","film-grain","heat-haze","underwater"].includes(o)&&ve(),o==="fog-distance"&&(x.fog=new c.FogExp2(10268850,u.density??.018))}function fe(o,u){return P.size?(re.set(G.set(o,200,u),K.set(0,-1,0)),re.intersectObjects([...P.keys()],!0)[0]?.point.y??0):0}function he(o,u,F){let M=[500,1500,4e3][V];if(B.length>=M)return;let a=o.startsWith("rain"),i=o==="snow",C=o==="smoke"||o.startsWith("fog"),R=o==="wind-leaves",k=o.startsWith("blood"),te=o==="fire",ae=o==="engine-trail",me=o==="water-splash",se=o==="stone-debris",ot=k?u.color||"#9f162a":C?"#9caebb":a||me?"#b3d7ed":se?"#a69b89":i?"#f5faff":R?"#9eae55":ae||o==="magic"||o==="teleport"||o==="pickup-glow"?"#76ddff":te?"#ff792e":"#ffbd57";S.set(u.color&&u.color!=="#ffffff"?u.color:ot);let Te=F||u.position||[0,1,0],ye=u.scale||1;B.push({id:o,x:Te[0],y:Te[1],z:Te[2],vx:(A()-.5)*(a?2:6)*ye,vy:a?-26:i?-1.5:C?.5+A():(A()*5+1)*ye,vz:(A()-.5)*(a?1:6)*ye,age:0,life:a?2.5:i?12:C?5:u.lifetime||1.5,size:(a?.25:i?.09:C?1.8:k?.11:.18)*ye,c:S.toArray(),kind:a?0:o.startsWith("fog")?4:C?1:R?3:2,floor:a||i?fe(Te[0],Te[2]):0});let q=B[B.length-1];if((te||ae)&&(q.kind=1,q.size=(te?.5:.25)*ye,q.life=te?1:.5,q.vx*=.1,q.vz*=.1,q.vy=te?2:0,ae)){let Ne=u.normal||[0,0,-1];q.vx=Ne[0]*5,q.vy=Ne[1]*5,q.vz=Ne[2]*5}(o==="muzzle-flash"||o==="hit-flash")&&(q.life=.12,q.size=.6*ye,q.kind=1,q.vx=q.vy=q.vz=0),u.normal&&(k||o==="metal-sparks"||se||me)&&(q.vx+=u.normal[0]*3,q.vy+=u.normal[1]*3,q.vz+=u.normal[2]*3),(o==="wind-dust"||R)&&(q.vx=3,q.vy=-.15,q.life=8,q.size=R?.16:.5,q.kind=R?3:1)}function Ee(o,u){if(!oe)return;for(o==="blood-pool"&&(u={...u,position:[u.position?.[0]||0,fe(u.position?.[0]||0,u.position?.[2]||0),u.position?.[2]||0],normal:[0,1,0]});_.length>=[24,48,96][V];){let k=_.shift();k.mesh.removeFromParent(),k.mesh.geometry.dispose(),k.mesh.material.dispose(),k.tex?.dispose()}let F=document.createElement("canvas");F.width=F.height=96;let M=F.getContext("2d");M.fillStyle=u.color||"#8c1625",M.beginPath();for(let k=0;k<=24;k++){let te=k/24*Math.PI*2,ae=25+A()*19,me=48+Math.cos(te)*ae,se=48+Math.sin(te)*ae;k?M.lineTo(me,se):M.moveTo(me,se)}M.fill();for(let k=0;k<14;k++)M.beginPath(),M.arc(A()*96,A()*96,1+A()*3,0,7),M.fill();let a=new c.CanvasTexture(F),i=new c.PlaneGeometry((o==="blood-pool"?1.2:.5)*(u.scale||1),(o==="blood-pool"?1:.6)*(u.scale||1)),C=new c.MeshBasicMaterial({map:a,transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,side:c.DoubleSide}),R=new c.Mesh(i,C);K.fromArray(u.normal||[0,1,0]),K.lengthSq()<.001&&K.set(0,1,0),K.normalize(),R.quaternion.setFromUnitVectors(new c.Vector3(0,0,1),K),R.position.fromArray(u.position||[0,0,0]).addScaledVector(K,.006),s.add(R),_.push({mesh:R,tex:a,age:0,life:u.lifetime||30})}function Ie(o,u){if(!["blood-pool","blood-decal","blood-spray","hit-flash","metal-sparks","stone-debris","fire","smoke","embers","explosion","muzzle-flash","engine-trail","magic","teleport","pickup-glow","water-splash","water-ripple"].includes(o))return;if(o==="blood-pool"||o==="blood-decal"){Ee(o,u);return}if(O&&(o==="hit-flash"||o==="muzzle-flash"))return;if(o==="hit-flash"){for(let M of L)M.id==="hit-flash"&&(M.age=0);he(o,{...u,color:"#efffff",lifetime:.08,scale:2});return}let F=Math.round((o==="explosion"?90:o==="muzzle-flash"?8:30)*Math.min(2,u.intensity??1));if(o!=="water-ripple")for(let M=0;M<F;M++)he(o,u);if(o==="explosion"){for(let M=0;M<18;M++)he("smoke",{...u,color:"#515760",scale:1.2,lifetime:3});for(let M of B.slice(-F-18,-18))M.vx*=2,M.vz*=2,M.vy*=1.5}if(o==="water-ripple"){for(;_.length>=[24,48,96][V];){let C=_.shift();C.mesh.removeFromParent(),C.mesh.geometry.dispose(),C.mesh.material.dispose(),C.tex?.dispose()}let M=new c.RingGeometry(.09,.1,32),a=new c.MeshBasicMaterial({color:11789555,transparent:!0,opacity:.65,side:c.DoubleSide,depthWrite:!1}),i=new c.Mesh(M,a);i.rotation.x=-Math.PI/2,i.position.fromArray(u.position||[0,.02,0]),s.add(i),_.push({mesh:i,age:0,life:u.lifetime||1.4,ripple:!0,scale:u.scale||1})}}function it(o,u){let F=!1;if(I=u,e.getWorldPosition(D),v&&!h.has("day-night")&&![...h.keys()].some(a=>a.startsWith("sky-"))&&(v.visible=d.visible=z.visible=j.visible=!1),v&&(h.has("day-night")||[...h.keys()].some(a=>a.startsWith("sky-")))){let a=h.get("day-night"),i=[...h.keys()].find(ae=>ae.startsWith("sky-"))||"sky-clear",R=((a?a.fixed?a.hour:(a.hour+u*24/Math.max(1,a.cycle))%24:i==="sky-night"||i==="sky-space"?0:i==="sky-sunset"?17.4:11)-6)/24*Math.PI*2,k=Math.max(0,Math.sin(R)),te=1-Math.min(1,k*3);v.position.copy(D),z.position.copy(D),d.position.copy(D),j.position.copy(D).add(G.set(-80,100,-120)),v.material.uniforms.sunPosition.value.set(Math.cos(R)*100,Math.sin(R)*100,-30),v.material.uniforms.turbidity.value=i==="sky-storm"?18:5,v.material.uniforms.rayleigh.value=2,v.material.uniforms.auraSkyColor.value.set(i==="sky-storm"?6714244:k<.3?14189931:8371940),v.visible=te<.95,d.visible=te>.15,d.material.opacity=te,j.visible=i!=="sky-space"&&te>.4,te>.7&&(x.background=ne.set(i==="sky-space"?396065:1121334)),n&&(n.position.set(Math.cos(R)*35,Math.sin(R)*40,15),n.intensity=.12+k*2.7,n.color.setRGB(1,.65+k*.28,.48+k*.4)),p&&(p.intensity=.3+k*1.3),z.material.uniforms.time.value=O?0:I,z.material.uniforms.coverage.value=i==="sky-cloudy"?.65:i==="sky-storm"?.85:.38,z.material.uniforms.night.value=te,z.visible=!0,z.material.uniforms.night.value=i==="sky-space"?1.5:te,x.fog&&x.fog.color.setRGB(.12+k*.45,.16+k*.48,.24+k*.45)}let M=[...h.keys()].find(a=>["rain-light","rain-heavy","snow","wind-dust","wind-leaves"].includes(a));if(M){W+=o*(M==="rain-heavy"?650:M==="rain-light"?220:M==="snow"?75:25)*[.3,.65,1][V]*Math.min(3,h.get(M).intensity??1);let a=Math.min(32,Math.floor(W));for(W=Math.min(1,W-a);a-- >0;)he(M,h.get(M),[D.x+(A()-.5)*34,D.y+8+A()*7,D.z+(A()-.5)*34])}for(let a of["fire","smoke","embers","engine-trail","fog-ground","fog-zone"])if(h.has(a)&&(a.startsWith("fog")||h.get(a).position)&&A()<o*18*Math.min(3,h.get(a).intensity??1)){let i=h.get(a),C=a.startsWith("fog"),R=i.position||[D.x,.5,D.z],k=a==="fog-zone"?i.radius||12:18;he(a,{...i,scale:C?5*(i.scale||1):i.scale,lifetime:C?8:i.lifetime||2},C?[R[0]+(A()-.5)*k*2,R[1],R[2]+(A()-.5)*k*2]:R)}h.has("thunderstorm")&&I>ie&&(F=!0,n&&!O&&(n.intensity=5),ie=I+5+A()*12);for(let a=B.length-1;a>=0;a--){let i=B[a];if(i.age+=o,i.age>i.life){B.splice(a,1);continue}if(i.x+=i.vx*o,i.y+=i.vy*o,i.z+=i.vz*o,!i.id.startsWith("rain")&&i.id!=="snow"&&i.kind!==1&&i.kind!==4&&(i.vy-=6*o),i.y<i.floor){i.id.startsWith("rain")&&V>0&&A()<.08&&Ie("water-ripple",{position:[i.x,i.floor+.02,i.z],lifetime:.6,scale:.25}),B.splice(a,1);continue}}for(let a=0;a<B.length;a++){let i=B[a];H.set([i.x,i.y,i.z],a*3),ce.set(i.c,a*3),$[a]=i.size*(i.kind===1?1+i.age*.2:1),Q[a]=(i.id.startsWith("rain")?Math.min(1,i.age*8):1)*(1-i.age/i.life),T[a]=i.kind}l.setDrawRange(0,B.length);for(let a of Object.values(l.attributes))a.needsUpdate=!0;for(let a=_.length-1;a>=0;a--){let i=_[a];i.age+=o,i.mesh.material.opacity=Math.min(1,(i.life-i.age)/Math.min(5,i.life)),i.ripple&&i.mesh.scale.setScalar((1+i.age*7)*i.scale),i.age>i.life&&(i.mesh.removeFromParent(),i.mesh.geometry.dispose(),i.mesh.material.dispose(),i.tex?.dispose(),_.splice(a,1))}for(let a of L)a.age+=o,a.clock.value=O?0:I,a.progress.value=a.id==="dissolve"?Math.min(1,a.age/a.life):a.id==="hit-flash"?Math.max(0,1-a.age*8):0;if(t){t.material.uniforms.time.value=I*t.userData.speed;let a=t.geometry.attributes.position,i=t.userData.base;for(let C=0;C<a.count;C++)a.array[C*3+2]=Math.sin(i[C*3]*.3+I)*Math.cos(i[C*3+1]*.25-I*.6)*(t.userData.id==="water-ocean"?.25:.035);a.needsUpdate=!0,t.geometry.computeVertexNormals(),t.material.uniforms.sunDirection.value.copy(n?.position||G.set(1,1,1)).normalize()}if(y){U.enabled=V>0&&h.has("bloom"),U.strength=.3*(h.get("bloom")?.intensity??1);let a=N.uniforms;a.time.value=I;for(let[i,C]of[["vignette","vignette"],["grain","film-grain"],["heat","heat-haze"],["underwater","underwater"],["grade","color-grade"]])a[i].value=h.has(C)?O&&["heat","underwater"].includes(i)?0:Math.min(2,h.get(C).intensity??1):0}return{thunder:F}}function Ge(){let o=r.getSize(new c.Vector2);y?.setSize(o.x,o.y)}function Ue(){B.length=0;for(let o of _)o.mesh.removeFromParent(),o.mesh.geometry.dispose(),o.mesh.material.dispose(),o.tex?.dispose();_.length=0,l.setDrawRange(0,0),W=0,ie=6,E=71237;for(let o of L)o.age=0,o.progress.value=0,o.clock.value=0}return{dimension:"3d",set:xe,emit:Ie,update:it,resize:Ge,reset:Ue,clear(){h.clear(),Ue(),x.fog=Y.fog,v&&(v.visible=d.visible=z.visible=j.visible=!1),t&&(t.removeFromParent(),t.geometry.dispose(),t.material.dispose(),t.dispose(),g.dispose(),t=null)},quality(o,u){V=o,O=u,y&&y.setPixelRatio(Math.min(r.getPixelRatio(),o===0?.75:o===1?1:1.5))},blood(o){if(oe=o,!oe)for(let u of _)u.ripple||(u.life=0)},render(){y?y.render(0):r.render(x,e)},registerSurface(o,u="ground"){return P.set(o,u),()=>P.delete(o)},applyObject(o,u,F={}){if(!["hologram","dissolve","hit-flash"].includes(u))throw Error("presentation: unsupported object effect");if(O&&u==="hit-flash")return()=>{};f.get(o)?.(),f.size>=128&&f.values().next().value();let M=[];o.traverse(C=>{if(!C.isMesh)return;let R=C.material,k=(Array.isArray(R)?R:[R]).map(te=>{let ae=te.clone(),me={material:ae,id:u,age:0,life:F.lifetime||2,progress:{value:0},clock:{value:0}};return ae.transparent=!0,ae.onBeforeCompile=se=>{se.uniforms.auraProgress=me.progress,se.uniforms.auraTime=me.clock,se.vertexShader=`varying vec3 auraPosition;
`+se.vertexShader.replace("#include <begin_vertex>",`#include <begin_vertex>
auraPosition=position;`),se.fragmentShader=`uniform float auraProgress,auraTime;varying vec3 auraPosition;
`+se.fragmentShader.replace("#include <dithering_fragment>",u==="dissolve"?"float n=fract(sin(dot(floor(auraPosition*35.),vec3(12.9898,78.233,37.719)))*43758.5453);if(n<auraProgress)discard;gl_FragColor.rgb+=vec3(1.,.3,.04)*(1.-smoothstep(0.,.07,n-auraProgress));":u==="hologram"?"float scan=.6+.4*sin(auraPosition.y*90.-auraTime*5.);gl_FragColor=vec4(vec3(.13,.72,1.)*scan,.38+scan*.3);":"gl_FragColor.rgb=mix(gl_FragColor.rgb,vec3(1.),auraProgress);")},ae.customProgramCacheKey=()=>"aurago-"+u,L.push(me),M.push(()=>{ae.dispose();let se=L.indexOf(me);se>=0&&L.splice(se,1)}),ae});C.material=Array.isArray(R)?k:k[0],M.push(()=>{C.material=R})});let a=!1,i=()=>{a||(a=!0,M.forEach(C=>C()),f.delete(o))};return f.set(o,i),i},stats(){return{particles:B.length,decals:_.length,surfaces:P.size,objects:L.length}},dispose(){if(!X){X=!0,Ue(),s.removeFromParent();for(let o of f.values())o();m.forEach(o=>o.dispose()),t&&(t.geometry.dispose(),t.material.dispose(),t.dispose(),g.dispose()),y?.dispose(),U?.dispose(),N?.dispose(),Z?.dispose(),P.clear(),x.background=Y.background,x.fog=Y.fog,n&&Y.sun&&(n.color.copy(Y.sun.color),n.intensity=Y.sun.intensity,n.position.copy(Y.sun.position)),p&&(p.intensity=Y.ambient)}}}}export{Ot as createPresentation,Di as createThreeAdapter};
