import*as m from"./three-0.186.1.module.min.js";import{BackSide as rt,BoxGeometry as at,Mesh as st,ShaderMaterial as nt,UniformsUtils as lt,Vector3 as ct}from"./three-0.186.1.module.min.js";var ke=class x extends st{constructor(){let e=x.SkyShader,a=new nt({name:e.name,uniforms:lt.clone(e.uniforms),vertexShader:e.vertexShader,fragmentShader:e.fragmentShader,side:rt,depthWrite:!1});super(new at(1,1,1),a),this.isSky=!0}};ke.SkyShader={name:"SkyShader",uniforms:{turbidity:{value:2},rayleigh:{value:1},mieCoefficient:{value:.005},mieDirectionalG:{value:.8},sunPosition:{value:new ct},cloudScale:{value:2e-4},cloudSpeed:{value:2e-5},cloudCoverage:{value:.4},cloudDensity:{value:.4},cloudElevation:{value:.5},showSunDisc:{value:1},time:{value:0}},vertexShader:`
		uniform vec3 sunPosition;
		uniform float rayleigh;
		uniform float turbidity;
		uniform float mieCoefficient;

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

			vSunE = sunIntensity( vSunDirection.y );

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
		uniform float cloudScale;
		uniform float cloudSpeed;
		uniform float cloudCoverage;
		uniform float cloudDensity;
		uniform float cloudElevation;
		uniform float showSunDisc;
		uniform float time;

		// gradient at a lattice corner; sinless hash so every GPU produces the same clouds
		vec2 gradient( vec2 i ) {
			vec3 p = fract( i.xyx * vec3( 0.1031, 0.1030, 0.0973 ) );
			p += dot( p, p.yzx + 33.33 );
			return fract( ( p.xx + p.yz ) * p.zy ) * 2.0 - 1.0;
		}

		// 2D gradient noise: isotropic lobes like Perlin at value-noise cost
		float noise( vec2 p ) {
			vec2 i = floor( p );
			vec2 f = fract( p );
			vec2 u = f * f * f * ( f * ( f * 6.0 - 15.0 ) + 10.0 ); // quintic fade
			float a = dot( gradient( i ), f );
			float b = dot( gradient( i + vec2( 1.0, 0.0 ) ), f - vec2( 1.0, 0.0 ) );
			float c = dot( gradient( i + vec2( 0.0, 1.0 ) ), f - vec2( 0.0, 1.0 ) );
			float d = dot( gradient( i + vec2( 1.0, 1.0 ) ), f - vec2( 1.0, 1.0 ) );
			return mix( mix( a, b, u.x ), mix( c, d, u.x ), u.y ) * 1.6; // ~[-1,1]
		}

		// fbm; per-octave drift makes clouds billow instead of scrolling as a rigid stamp
		float fbm( vec2 p, float drift ) {
			float result = 0.0;
			float amplitude = 1.0;
			for ( int i = 0; i < 4; i ++ ) {
				result += amplitude * noise( p );
				amplitude *= 0.5;
				p = p * 2.0 + drift;
			}
			return result;
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
			float zenithAngle = acos( max( 0.0, direction.y ) );
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
			Lin *= mix( vec3( 1.0 ), pow( vSunE * ( ( betaRTheta + betaMTheta ) / ( vBetaR + vBetaM ) ) * Fex, vec3( 1.0 / 2.0 ) ), clamp( pow( 1.0 - vSunDirection.y, 5.0 ), 0.0, 1.0 ) );

			// nightsky
			float theta = acos( direction.y ); // elevation --> y-axis, [-pi/2, pi/2]
			float phi = atan( direction.z, direction.x ); // azimuth --> x-axis [-pi/2, pi/2]
			vec2 uv = vec2( phi, theta ) / vec2( 2.0 * pi, pi ) + vec2( 0.5, 0.0 );
			vec3 L0 = vec3( 0.1 ) * Fex;

			// composition + solar disc
			float sundisc = clamp( ( cosTheta - sunAngularDiameterCos ) * 50000.0, 0.0, 1.0 ) * showSunDisc;
			vec3 sundiscColor = ( 760.0 * sundisc ) * min( vSunE * Fex, 80.0 );

			vec3 texColor = ( Lin + L0 ) * 0.04 + sundiscColor + vec3( 0.0, 0.0003, 0.00075 );

			// Clouds
			if ( direction.y > 0.0 && cloudCoverage > 0.0 ) {

				// Project to cloud plane (higher elevation = clouds appear lower/closer)
				float elevation = mix( 1.0, 0.1, cloudElevation );
				vec2 cloudUV = direction.xz / ( direction.y * elevation );
				cloudUV *= cloudScale;
				cloudUV += time * cloudSpeed;

				// Cloud density field
				float evolve = time * cloudSpeed * 300.0;
				float cloudNoise = clamp( fbm( cloudUV * 1000.0, evolve ) * 0.7 + 0.5, 0.0, 1.0 );

				// Large-scale coverage variation: clear gaps next to dense banks
				float region = noise( cloudUV * 300.0 ) * 0.37 + 0.5;
				float cov = clamp( cloudCoverage + ( region - 0.5 ) * 0.6, 0.0, 1.0 );

				// Carve clouds where noise rises above the coverage level
				float threshold = 1.0 - cov;
				float cloudMask = smoothstep( threshold, threshold + 0.3, cloudNoise );

				// Fade clouds near horizon (adjusted by elevation)
				float horizonFade = smoothstep( 0.0, 0.03 + 0.06 * cloudElevation, direction.y );
				cloudMask *= horizonFade;

				// Cloud lighting from the sky's own radiance
				float dayFactor = smoothstep( -0.08, 0.3, vSunDirection.y );
				vec3 sunColor = vSunE * Fex * 0.22 * 0.04; // 0.22 ~ albedo/pi, 0.04 = exposure; the aerial composite adds the eye-leg extinction
				vec3 skyAmbient = Lin * 0.04 + vec3( 0.0, 0.0003, 0.00075 );

				// Beer-powder self-shadow from the sampled density
				float depth = max( 0.0, cloudNoise - threshold );
				float beer = exp( depth * -4.0 );
				float powder = 1.0 - beer * beer; // beer*beer == exp(-8*depth)
				float shade = mix( 0.45, 1.0, clamp( beer * powder * 2.6, 0.0, 1.0 ) ); // 2.6 = 1/0.385, normalizes beer*powder peak to 1

				// Henyey-Greenstein forward lobe ( g = 0.7 ): silver lining on rims toward the sun
				float silver = clamp( 0.51 / pow( 1.49 - cosTheta * 1.4, 1.5 ), 0.0, 3.0 ); // 0.51=1-g^2, 1.49=1+g^2, 1.4=2g
				float edge = cloudMask * ( 1.0 - cloudMask ) * 4.0;

				vec3 cloudColor = skyAmbient + sunColor * shade;
				cloudColor += sunColor * silver * edge * 0.6;
				cloudColor *= max( dayFactor, 0.03 );

				// Cloud opacity via Beer's law: density sets how solid the clouds get
				float alpha = ( 1.0 - exp( depth * cloudDensity * -12.0 ) ) * horizonFade;

				// Occlude the sun disc/glow behind opaque cloud
				texColor -= L0 * 0.04 * alpha;

				// Composite through the atmosphere so distant clouds dissolve into haze
				vec3 cloudAerial = mix( texColor, cloudColor, Fex );
				texColor = mix( texColor, cloudAerial, alpha );

			}

			gl_FragColor = vec4( texColor, 1.0 );

			#include <tonemapping_fragment>
			#include <colorspace_fragment>

		}`};import{Color as Fe,FrontSide as ut,HalfFloatType as ft,Matrix4 as Ve,Mesh as ht,PerspectiveCamera as dt,Plane as mt,ShaderMaterial as pt,UniformsLib as Qe,UniformsUtils as qe,Vector3 as ve,Vector4 as Xe,WebGLRenderTarget as gt}from"./three-0.186.1.module.min.js";var De=class extends ht{constructor(e,a={}){super(e),this.isWater=!0;let u=this,v=a.textureWidth!==void 0?a.textureWidth:512,n=a.textureHeight!==void 0?a.textureHeight:512,c=a.clipBias!==void 0?a.clipBias:0,C=a.alpha!==void 0?a.alpha:1,d=a.time!==void 0?a.time:0,E=a.waterNormals!==void 0?a.waterNormals:null,k=a.sunDirection!==void 0?a.sunDirection:new ve(.70707,.70707,0),Q=new Fe(a.sunColor!==void 0?a.sunColor:16777215),f=new Fe(a.waterColor!==void 0?a.waterColor:8355711),O=a.eye!==void 0?a.eye:new ve(0,0,0),I=a.distortionScale!==void 0?a.distortionScale:20,se=a.side!==void 0?a.side:ut,q=a.fog!==void 0?a.fog:!1,$=new mt,_=new ve,N=new ve,H=new ve,M=new Ve,Z=new ve(0,0,-1),L=new Xe,oe=new ve,ee=new ve,X=new Xe,ne=new Ve,D=new dt,pe=new gt(v,n,{type:ft}),fe={name:"MirrorShader",uniforms:qe.merge([Qe.fog,Qe.lights,{normalSampler:{value:null},mirrorSampler:{value:null},alpha:{value:1},time:{value:0},size:{value:1},distortionScale:{value:20},textureMatrix:{value:new Ve},sunColor:{value:new Fe(8355711)},sunDirection:{value:new ve(.70707,.70707,0)},eye:{value:new ve},waterColor:{value:new Fe(5592405)}}]),vertexShader:`
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
				}`},Y=new pt({name:fe.name,uniforms:qe.clone(fe.uniforms),vertexShader:fe.vertexShader,fragmentShader:fe.fragmentShader,lights:!0,side:se,fog:q});Y.uniforms.mirrorSampler.value=pe.texture,this.dispose=()=>pe.dispose(),Y.uniforms.textureMatrix.value=ne,Y.uniforms.alpha.value=C,Y.uniforms.time.value=d,Y.uniforms.normalSampler.value=E,Y.uniforms.sunColor.value=Q,Y.uniforms.waterColor.value=f,Y.uniforms.sunDirection.value=k,Y.uniforms.distortionScale.value=I,Y.uniforms.eye.value=O,u.material=Y,u.onBeforeRender=function(J,G,ie){if(N.setFromMatrixPosition(u.matrixWorld),H.setFromMatrixPosition(ie.matrixWorld),M.extractRotation(u.matrixWorld),_.set(0,0,1),_.applyMatrix4(M),oe.subVectors(N,H),oe.dot(_)>0)return;oe.reflect(_).negate(),oe.add(N),M.extractRotation(ie.matrixWorld),Z.set(0,0,-1),Z.applyMatrix4(M),Z.add(H),ee.subVectors(N,Z),ee.reflect(_).negate(),ee.add(N),D.position.copy(oe),D.up.set(0,1,0),D.up.applyMatrix4(M),D.up.reflect(_),D.lookAt(ee),D.far=ie.far,D.updateMatrixWorld(),D.projectionMatrix.copy(ie.projectionMatrix),ne.set(.5,0,0,.5,0,.5,0,.5,0,0,.5,.5,0,0,0,1),ne.multiply(D.projectionMatrix),ne.multiply(D.matrixWorldInverse),$.setFromNormalAndCoplanarPoint(_,N),$.applyMatrix4(D.matrixWorldInverse),L.set($.normal.x,$.normal.y,$.normal.z,$.constant);let B=D.projectionMatrix;X.x=(Math.sign(L.x)+B.elements[8])/B.elements[0],X.y=(Math.sign(L.y)+B.elements[9])/B.elements[5],X.z=-1,X.w=(1+B.elements[10])/B.elements[14],L.multiplyScalar(2/L.dot(X)),B.elements[2]=L.x,B.elements[6]=L.y,B.elements[10]=L.z+1-c,B.elements[14]=L.w,O.setFromMatrixPosition(ie.matrixWorld);let i=J.getRenderTarget(),g=J.xr.enabled,p=J.shadowMap.autoUpdate;u.visible=!1,J.xr.enabled=!1,J.shadowMap.autoUpdate=!1,J.setRenderTarget(pe),J.state.buffers.depth.setMask(!0),J.autoClear===!1&&J.clear(),J.render(G,D),u.visible=!0,J.xr.enabled=g,J.shadowMap.autoUpdate=p,J.setRenderTarget(i);let S=ie.viewport;S!==void 0&&J.state.viewport(S)}}};import{HalfFloatType as Tt,NoBlending as St,Timer as Ct,Vector2 as Ye,WebGLRenderTarget as _t}from"./three-0.186.1.module.min.js";var Se={name:"CopyShader",uniforms:{tDiffuse:{value:null},opacity:{value:1}},vertexShader:`

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


		}`};import{ShaderMaterial as Ze,UniformsUtils as Mt}from"./three-0.186.1.module.min.js";import{BufferGeometry as vt,Float32BufferAttribute as Ke,OrthographicCamera as xt,Mesh as yt}from"./three-0.186.1.module.min.js";var de=class{constructor(){this.isPass=!0,this.enabled=!0,this.needsSwap=!0,this.clear=!1,this.renderToScreen=!1}setSize(){}render(){console.error("THREE.Pass: .render() must be implemented in derived pass.")}dispose(){}},wt=new xt(-1,1,1,-1,0,1),Ie=class extends vt{constructor(){super(),this.setAttribute("position",new Ke([-1,3,0,-1,-1,0,3,-1,0],3)),this.setAttribute("uv",new Ke([0,2,0,0,2,0],2))}},bt=new Ie,we=class{constructor(e){this._mesh=new yt(bt,e)}dispose(){this._mesh.geometry.dispose()}render(e){e.render(this._mesh,wt)}get material(){return this._mesh.material}set material(e){this._mesh.material=e}};var Ce=class extends de{constructor(e,a="tDiffuse"){super(),this.textureID=a,this.uniforms=null,this.material=null,e instanceof Ze?(this.uniforms=e.uniforms,this.material=e):e&&(this.uniforms=Mt.clone(e.uniforms),this.material=new Ze({name:e.name!==void 0?e.name:"unspecified",defines:Object.assign({},e.defines),uniforms:this.uniforms,vertexShader:e.vertexShader,fragmentShader:e.fragmentShader})),this._fsQuad=new we(this.material)}render(e,a,u){this.uniforms[this.textureID]&&(this.uniforms[this.textureID].value=u.texture),this._fsQuad.material=this.material,this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(a),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var Ae=class extends de{constructor(e,a){super(),this.scene=e,this.camera=a,this.clear=!0,this.needsSwap=!1,this.inverse=!1}render(e,a,u){let v=e.getContext(),n=e.state;n.buffers.color.setMask(!1),n.buffers.depth.setMask(!1),n.buffers.color.setLocked(!0),n.buffers.depth.setLocked(!0);let c,C;this.inverse?(c=0,C=1):(c=1,C=0),n.buffers.stencil.setTest(!0),n.buffers.stencil.setOp(v.REPLACE,v.REPLACE,v.REPLACE),n.buffers.stencil.setFunc(v.ALWAYS,c,4294967295),n.buffers.stencil.setClear(C),n.buffers.stencil.setLocked(!0),e.setRenderTarget(u),this.clear&&e.clear(),e.render(this.scene,this.camera),e.setRenderTarget(a),this.clear&&e.clear(),e.render(this.scene,this.camera),n.buffers.color.setLocked(!1),n.buffers.depth.setLocked(!1),n.buffers.color.setMask(!0),n.buffers.depth.setMask(!0),n.buffers.stencil.setLocked(!1),n.buffers.stencil.setFunc(v.EQUAL,1,4294967295),n.buffers.stencil.setOp(v.KEEP,v.KEEP,v.KEEP),n.buffers.stencil.setLocked(!0)}},Be=class extends de{constructor(){super(),this.needsSwap=!1}render(e){e.state.buffers.stencil.setLocked(!1),e.state.buffers.stencil.setTest(!1)}};var Ue=class{constructor(e,a){if(this.renderer=e,this._pixelRatio=e.getPixelRatio(),a===void 0){let u=e.getSize(new Ye);this._width=u.width,this._height=u.height,a=new _t(this._width*this._pixelRatio,this._height*this._pixelRatio,{type:Tt}),a.texture.name="EffectComposer.rt1"}else this._width=a.width,this._height=a.height;this.renderTarget1=a,this.renderTarget2=a.clone(),this.renderTarget2.texture.name="EffectComposer.rt2",this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2,this.renderToScreen=!0,this.passes=[],this.copyPass=new Ce(Se),this.copyPass.material.blending=St,this.timer=new Ct}swapBuffers(){let e=this.readBuffer;this.readBuffer=this.writeBuffer,this.writeBuffer=e}addPass(e){this.passes.push(e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}insertPass(e,a){this.passes.splice(a,0,e),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}removePass(e){let a=this.passes.indexOf(e);a!==-1&&this.passes.splice(a,1)}isLastEnabledPass(e){for(let a=e+1;a<this.passes.length;a++)if(this.passes[a].enabled)return!1;return!0}render(e){this.timer.update(),e===void 0&&(e=this.timer.getDelta());let a=this.renderer.getRenderTarget(),u=!1;for(let v=0,n=this.passes.length;v<n;v++){let c=this.passes[v];if(c.enabled!==!1){if(c.renderToScreen=this.renderToScreen&&this.isLastEnabledPass(v),c.render(this.renderer,this.writeBuffer,this.readBuffer,e,u),c.needsSwap){if(u){let C=this.renderer.getContext(),d=this.renderer.state.buffers.stencil;d.setFunc(C.NOTEQUAL,1,4294967295),this.copyPass.render(this.renderer,this.writeBuffer,this.readBuffer,e),d.setFunc(C.EQUAL,1,4294967295)}this.swapBuffers()}Ae!==void 0&&(c instanceof Ae?u=!0:c instanceof Be&&(u=!1))}}this.renderer.setRenderTarget(a)}reset(e){if(e===void 0){let a=this.renderer.getSize(new Ye);this._pixelRatio=this.renderer.getPixelRatio(),this._width=a.width,this._height=a.height,e=this.renderTarget1.clone(),e.setSize(this._width*this._pixelRatio,this._height*this._pixelRatio)}this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.renderTarget1=e,this.renderTarget2=e.clone(),this.writeBuffer=this.renderTarget1,this.readBuffer=this.renderTarget2}setSize(e,a){this._width=e,this._height=a;let u=this._width*this._pixelRatio,v=this._height*this._pixelRatio;this.renderTarget1.setSize(u,v),this.renderTarget2.setSize(u,v);for(let n=0;n<this.passes.length;n++)this.passes[n].setSize(u,v)}setPixelRatio(e){this._pixelRatio=e,this.setSize(this._width,this._height)}dispose(){this.renderTarget1.dispose(),this.renderTarget2.dispose(),this.copyPass.dispose()}};import{Color as Pt}from"./three-0.186.1.module.min.js";var Ne=class extends de{constructor(e,a,u=null,v=null,n=null){super(),this.scene=e,this.camera=a,this.overrideMaterial=u,this.clearColor=v,this.clearAlpha=n,this.clear=!0,this.clearDepth=!1,this.needsSwap=!1,this.isRenderPass=!0,this._oldClearColor=new Pt}render(e,a,u){let v=e.autoClear;e.autoClear=!1;let n,c;this.overrideMaterial!==null&&(c=this.scene.overrideMaterial,this.scene.overrideMaterial=this.overrideMaterial),this.clearColor!==null&&(e.getClearColor(this._oldClearColor),e.setClearColor(this.clearColor,e.getClearAlpha())),this.clearAlpha!==null&&(n=e.getClearAlpha(),e.setClearAlpha(this.clearAlpha)),this.clearDepth==!0&&e.clearDepth(),e.setRenderTarget(this.renderToScreen?null:u),this.clear===!0&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),e.render(this.scene,this.camera),this.clearColor!==null&&e.setClearColor(this._oldClearColor),this.clearAlpha!==null&&e.setClearAlpha(n),this.overrideMaterial!==null&&(this.scene.overrideMaterial=c),e.autoClear=v}};import{AdditiveBlending as kt,Color as Je,HalfFloatType as Ge,MeshBasicMaterial as At,ShaderMaterial as Le,UniformsUtils as et,Vector2 as be,Vector3 as ze,WebGLRenderTarget as je}from"./three-0.186.1.module.min.js";import{Color as Et}from"./three-0.186.1.module.min.js";var $e={name:"LuminosityHighPassShader",uniforms:{tDiffuse:{value:null},luminosityThreshold:{value:1},smoothWidth:{value:1},defaultColor:{value:new Et(0)},defaultOpacity:{value:0}},vertexShader:`

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

		}`};var _e=class x extends de{constructor(e,a=1,u,v){super(),this.strength=a,this.radius=u,this.threshold=v,this.resolution=e!==void 0?new be(e.x,e.y):new be(256,256),this.clearColor=new Je(0,0,0),this.needsSwap=!1,this.renderTargetsHorizontal=[],this.renderTargetsVertical=[],this.nMips=5;let n=Math.round(this.resolution.x/2),c=Math.round(this.resolution.y/2);this.renderTargetBright=new je(n,c,{type:Ge,depthBuffer:!1}),this.renderTargetBright.texture.name="UnrealBloomPass.bright",this.renderTargetBright.texture.generateMipmaps=!1;for(let k=0;k<this.nMips;k++){let Q=new je(n,c,{type:Ge,depthBuffer:!1});Q.texture.name="UnrealBloomPass.h"+k,Q.texture.generateMipmaps=!1,this.renderTargetsHorizontal.push(Q);let f=new je(n,c,{type:Ge,depthBuffer:!1});f.texture.name="UnrealBloomPass.v"+k,f.texture.generateMipmaps=!1,this.renderTargetsVertical.push(f),n=Math.round(n/2),c=Math.round(c/2)}let C=$e;this.highPassUniforms=et.clone(C.uniforms),this.highPassUniforms.luminosityThreshold.value=v,this.highPassUniforms.smoothWidth.value=.01,this.materialHighPassFilter=new Le({uniforms:this.highPassUniforms,vertexShader:C.vertexShader,fragmentShader:C.fragmentShader}),this.separableBlurMaterials=[];let d=[6,10,14,18,22];n=Math.round(this.resolution.x/2),c=Math.round(this.resolution.y/2);for(let k=0;k<this.nMips;k++)this.separableBlurMaterials.push(this._getSeparableBlurMaterial(d[k])),this.separableBlurMaterials[k].uniforms.invSize.value=new be(1/n,1/c),n=Math.round(n/2),c=Math.round(c/2);this.compositeMaterial=this._getCompositeMaterial(this.nMips),this.compositeMaterial.uniforms.blurTexture1.value=this.renderTargetsVertical[0].texture,this.compositeMaterial.uniforms.blurTexture2.value=this.renderTargetsVertical[1].texture,this.compositeMaterial.uniforms.blurTexture3.value=this.renderTargetsVertical[2].texture,this.compositeMaterial.uniforms.blurTexture4.value=this.renderTargetsVertical[3].texture,this.compositeMaterial.uniforms.blurTexture5.value=this.renderTargetsVertical[4].texture,this.compositeMaterial.uniforms.bloomStrength.value=a,this.compositeMaterial.uniforms.bloomRadius.value=.1;let E=[1,.8,.6,.4,.2];this.compositeMaterial.uniforms.bloomFactors.value=E,this.bloomTintColors=[new ze(1,1,1),new ze(1,1,1),new ze(1,1,1),new ze(1,1,1),new ze(1,1,1)],this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,this.copyUniforms=et.clone(Se.uniforms),this.blendMaterial=new Le({uniforms:this.copyUniforms,vertexShader:Se.vertexShader,fragmentShader:Se.fragmentShader,premultipliedAlpha:!0,blending:kt,depthTest:!1,depthWrite:!1,transparent:!0}),this._oldClearColor=new Je,this._oldClearAlpha=1,this._basic=new At,this._fsQuad=new we(null)}dispose(){for(let e=0;e<this.renderTargetsHorizontal.length;e++)this.renderTargetsHorizontal[e].dispose();for(let e=0;e<this.renderTargetsVertical.length;e++)this.renderTargetsVertical[e].dispose();this.renderTargetBright.dispose();for(let e=0;e<this.separableBlurMaterials.length;e++)this.separableBlurMaterials[e].dispose();this.compositeMaterial.dispose(),this.blendMaterial.dispose(),this._basic.dispose(),this._fsQuad.dispose()}setSize(e,a){let u=Math.round(e/2),v=Math.round(a/2);this.renderTargetBright.setSize(u,v);for(let n=0;n<this.nMips;n++)this.renderTargetsHorizontal[n].setSize(u,v),this.renderTargetsVertical[n].setSize(u,v),this.separableBlurMaterials[n].uniforms.invSize.value=new be(1/u,1/v),u=Math.round(u/2),v=Math.round(v/2)}render(e,a,u,v,n){e.getClearColor(this._oldClearColor),this._oldClearAlpha=e.getClearAlpha();let c=e.autoClear;e.autoClear=!1,e.setClearColor(this.clearColor,0),n&&e.state.buffers.stencil.setTest(!1),this.renderToScreen&&(this._fsQuad.material=this._basic,this._basic.map=u.texture,e.setRenderTarget(null),e.clear(),this._fsQuad.render(e)),this.highPassUniforms.tDiffuse.value=u.texture,this.highPassUniforms.luminosityThreshold.value=this.threshold,this._fsQuad.material=this.materialHighPassFilter,e.setRenderTarget(this.renderTargetBright),e.clear(),this._fsQuad.render(e);let C=this.renderTargetBright;for(let d=0;d<this.nMips;d++)this._fsQuad.material=this.separableBlurMaterials[d],this.separableBlurMaterials[d].uniforms.colorTexture.value=C.texture,this.separableBlurMaterials[d].uniforms.direction.value=x.BlurDirectionX,e.setRenderTarget(this.renderTargetsHorizontal[d]),e.clear(),this._fsQuad.render(e),this.separableBlurMaterials[d].uniforms.colorTexture.value=this.renderTargetsHorizontal[d].texture,this.separableBlurMaterials[d].uniforms.direction.value=x.BlurDirectionY,e.setRenderTarget(this.renderTargetsVertical[d]),e.clear(),this._fsQuad.render(e),C=this.renderTargetsVertical[d];this._fsQuad.material=this.compositeMaterial,this.compositeMaterial.uniforms.bloomStrength.value=this.strength,this.compositeMaterial.uniforms.bloomRadius.value=this.radius,this.compositeMaterial.uniforms.bloomTintColors.value=this.bloomTintColors,e.setRenderTarget(this.renderTargetsHorizontal[0]),e.clear(),this._fsQuad.render(e),this._fsQuad.material=this.blendMaterial,this.copyUniforms.tDiffuse.value=this.renderTargetsHorizontal[0].texture,n&&e.state.buffers.stencil.setTest(!0),this.renderToScreen?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(u),this._fsQuad.render(e)),e.setClearColor(this._oldClearColor,this._oldClearAlpha),e.autoClear=c}_getSeparableBlurMaterial(e){let a=[],u=e/3;for(let c=0;c<e;c++)a.push(.39894*Math.exp(-.5*c*c/(u*u))/u);let v=[],n=[];for(let c=1;c<e;c+=2){let C=a[c],d=c+1<e?a[c+1]:0,E=C+d;v.push((c*C+(c+1)*d)/E),n.push(E)}return new Le({defines:{KERNEL_PAIRS:v.length},uniforms:{colorTexture:{value:null},invSize:{value:new be(.5,.5)},direction:{value:new be(.5,.5)},centerWeight:{value:a[0]},gaussianOffsets:{value:v},gaussianWeights:{value:n}},vertexShader:`

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

				}`})}};_e.BlurDirectionX=new be(1,0);_e.BlurDirectionY=new be(0,1);import{ColorManagement as zt,RawShaderMaterial as Rt,UniformsUtils as Ft,LinearToneMapping as Dt,ReinhardToneMapping as Bt,CineonToneMapping as Ut,AgXToneMapping as Nt,ACESFilmicToneMapping as Lt,NeutralToneMapping as Wt,CustomToneMapping as Ot,SRGBTransfer as Vt}from"./three-0.186.1.module.min.js";var Re={name:"OutputShader",uniforms:{tDiffuse:{value:null},toneMappingExposure:{value:1}},vertexShader:`
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

		}`};var We=class extends de{constructor(){super(),this.isOutputPass=!0,this.uniforms=Ft.clone(Re.uniforms),this.material=new Rt({name:Re.name,uniforms:this.uniforms,vertexShader:Re.vertexShader,fragmentShader:Re.fragmentShader}),this._fsQuad=new we(this.material),this._outputColorSpace=null,this._toneMapping=null}render(e,a,u){this.uniforms.tDiffuse.value=u.texture,this.uniforms.toneMappingExposure.value=e.toneMappingExposure,(this._outputColorSpace!==e.outputColorSpace||this._toneMapping!==e.toneMapping)&&(this._outputColorSpace=e.outputColorSpace,this._toneMapping=e.toneMapping,this.material.defines={},zt.getTransfer(this._outputColorSpace)===Vt&&(this.material.defines.SRGB_TRANSFER=""),this._toneMapping===Dt?this.material.defines.LINEAR_TONE_MAPPING="":this._toneMapping===Bt?this.material.defines.REINHARD_TONE_MAPPING="":this._toneMapping===Ut?this.material.defines.CINEON_TONE_MAPPING="":this._toneMapping===Lt?this.material.defines.ACES_FILMIC_TONE_MAPPING="":this._toneMapping===Nt?this.material.defines.AGX_TONE_MAPPING="":this._toneMapping===Wt?this.material.defines.NEUTRAL_TONE_MAPPING="":this._toneMapping===Ot&&(this.material.defines.CUSTOM_TONE_MAPPING=""),this.material.needsUpdate=!0),this.renderToScreen===!0?(e.setRenderTarget(null),this._fsQuad.render(e)):(e.setRenderTarget(a),this.clear&&e.clear(e.autoClearColor,e.autoClearDepth,e.autoClearStencil),this._fsQuad.render(e))}dispose(){this.material.dispose(),this._fsQuad.dispose()}};var He=new WeakMap;function tt({sounds:x=[],base:e,root:a,report:u=console.warn,dimension:v="3d"}){let n=new Map(x.map(t=>[t.id,t])),c=new Map,C=new Map,d=new Set,E=new Map,k=new Map,Q=new AbortController,f,O,I,se,q,$,_=!1,N=!1,H=!1,M=.55,Z=He.get(a);Z&&(H=Z.muted,M=Z.volume);let L={},oe=[0,0,0],ee=new Set,X=new Set,ne="outside",D=0;function pe(){if(!(f||_)){f=new AudioContext({sampleRate:48e3}),O=f.createGain(),I=f.createDynamicsCompressor(),I.threshold.value=-12,I.knee.value=12,I.ratio.value=8,I.attack.value=.003,I.release.value=.2,O.connect(I).connect(f.destination);for(let t of["effects","ambience","music"])L[t]=f.createGain(),L[t].gain.value=t==="ambience"?.45:1,L[t].connect(O);q=f.createConvolver(),se=f.createGain(),$=f.createBiquadFilter(),$.type="lowpass",$.frequency.value=8e3,q.connect($).connect(se).connect(O),Y(ne),fe()}}function fe(){O&&O.gain.setTargetAtTime(H||N?0:M*M,f.currentTime,.03)}function Y(t){if(!["outside","small-room","hall"].includes(t))throw Error("audio: unknown room");if(ne=t,!f)return;se.gain.value=t==="outside"?0:t==="hall"?.2:.1;let y=Math.floor(f.sampleRate*(t==="hall"?1.7:.35)),w=f.createBuffer(2,y,f.sampleRate),W=1973;for(let U=0;U<2;U++){let te=w.getChannelData(U);for(let T=0;T<y;T++)W=Math.imul(W,1664525)+1013904223>>>0,te[T]=(W/2147483648-1)*Math.pow(1-T/y,3)}q.buffer=w}async function J(t){if(c.has(t))return c.get(t);if(C.has(t))return C.get(t);let y=n.get(t);if(!y)throw Error("audio: unknown imported sound "+t);let w=(async()=>{let W=y.files?.[0];if(!W||!W.file.startsWith("sounds/")||!/^sounds\/[a-z0-9-]+\.wav$/.test(W.file))throw Error("audio: invalid sample path");let U=new URL(e,document.baseURI),te=new URL(W.file,U);if(te.origin!==new URL(document.baseURI).origin||!te.pathname.startsWith(U.pathname))throw Error("audio: sample outside game");let T=await fetch(te,{signal:Q.signal});if(!T.ok)throw Error("audio: "+t+" HTTP "+T.status);let le=await T.arrayBuffer();if(le.byteLength!==W.bytes)throw Error("audio: size mismatch "+t);if(crypto.subtle&&[...new Uint8Array(await crypto.subtle.digest("SHA-256",le))].map(V=>V.toString(16).padStart(2,"0")).join("")!==W.sha256)throw Error("audio: checksum mismatch "+t);let ae=await f.decodeAudioData(le);return _?null:(c.set(t,ae),ae)})();C.set(t,w);try{return await w}finally{C.delete(t)}}function G(t){if(d.has(t)){d.delete(t);try{t.source.stop()}catch{}t.source.disconnect(),t.gain.disconnect(),t.pan?.disconnect()}}function ie(t,y=.18){if(!(!d.has(t)||t.fading)){t.fading=!0,t.gain.gain.setTargetAtTime(0,f.currentTime,y/3);try{t.source.stop(f.currentTime+y)}catch{}}}function B(t){!t.position||!t.pan?.pan||t.fading||(t.pan.pan.value=Math.max(-1,Math.min(1,(t.position[0]-oe[0])/450)),t.gain.gain.setTargetAtTime(t.volume/(1+Math.hypot(t.position[0]-oe[0],t.position[1]-oe[1])/500),f.currentTime,.03))}function i(t,y,w={}){if(!y||_||N||H||f.state!=="running")return null;let W=n.get(t),U=w.loop??W?.loop??!1,te=w.bus||(U&&t!=="engine"?"ambience":"effects");if(!L[te])throw Error("audio: unknown bus");let T=U&&[...d].find(he=>he.loop&&he.id===t&&!he.fading);if(T)return w.position&&(T.position=w.position.slice(),T.pan?.positionX?[T.pan.positionX.value,T.pan.positionY.value,T.pan.positionZ.value]=w.position:B(T)),T;if(te==="ambience"&&[...d].filter(he=>he.bus==="ambience").length>=4)return null;if(d.size>=32){let he=[...d].filter(ye=>!ye.loop).sort((ye,Me)=>ye.priority-Me.priority||ye.start-Me.start)[0];if(!he||he.priority>(w.priority??1))return null;G(he)}let le=f.createBufferSource(),ae=f.createGain();le.buffer=y,le.loop=U,le.playbackRate.value=w.rate??(U?1:1+(Math.random()-.5)*.06);let me=Math.min(1,Math.max(0,w.gain??W?.gain??.6));ae.gain.setValueAtTime(U?0:me,f.currentTime),U&&ae.gain.linearRampToValueAtTime(me,f.currentTime+.3);let V;le.connect(ae),w.position?(v==="3d"?(V=f.createPanner(),V.panningModel="HRTF",V.distanceModel="inverse",V.refDistance=2,V.maxDistance=80,V.rolloffFactor=1.3,[V.positionX.value,V.positionY.value,V.positionZ.value]=w.position):V=f.createStereoPanner(),ae.connect(V).connect(L[te])):ae.connect(L[te]),te==="effects"&&(V||ae).connect(q);let xe={id:t,source:le,gain:ae,pan:V,loop:U,bus:te,volume:me,position:w.position?.slice(),priority:w.priority??1,start:f.currentTime};return v==="2d"&&V&&(ae.gain.cancelScheduledValues(f.currentTime),B(xe)),d.add(xe),le.onended=()=>G(xe),le.start(),xe}async function g(t,y={}){if(!n.has(t))return u("audio: unknown imported sound "+t),null;if(!f||_||N||H||f.state!=="running")return null;if(y.position&&(!Array.isArray(y.position)||y.position.length!==3||y.position.some(T=>!Number.isFinite(T))))throw Error("audio: invalid position");for(let T of["rate","gain","cooldown","priority"])if(y[T]!==void 0&&!Number.isFinite(y[T]))throw Error("audio: invalid "+T);let w=D,W=k.get(t),U=f.currentTime;if(U-(E.get(t)??-99)<(y.cooldown??.045))return null;E.set(t,U);let te=y.loop??n.get(t).loop;try{let T=await J(t);return w!==D||W!==k.get(t)||!te&&f.currentTime-U>.2||y.ambient&&!l.includes(t)?null:i(t,T,y)}catch(T){return!_&&T.name!=="AbortError"&&!X.has(t)&&(X.add(t),u(String(T))),null}}let p=new Map;function S(t){let y={hit:[160,80],damage:[120,60],death:[180,90,45],respawn:[330,440,660],pickup:[660,990],win:[440,554,660,880],lose:[220,164,110]};if(!Object.hasOwn(y,t)||!f||_||N||H||f.state!=="running")return null;let w="cue:"+t,W=f.currentTime;if(W-(E.get(w)??-99)<.12)return null;if(E.set(w,W),!p.has(t)){let U=y[t],te=U.length*.085,T=f.createBuffer(1,Math.ceil(te*f.sampleRate),f.sampleRate),le=T.getChannelData(0),ae=0,me=1973;for(let V=0;V<le.length;V++){let xe=V/f.sampleRate,he=Math.min(U.length-1,Math.floor(xe/.085)),ye=xe%.085;ae+=2*Math.PI*U[he]/f.sampleRate,me=Math.imul(me,1664525)+1013904223>>>0;let Me=["hit","damage","death"].includes(t)?(me/2147483648-1)*.25:0,Pe=Math.min(1,ye/.004)*Math.max(0,1-ye/.085)**2;le[V]=(Math.sin(ae)*.35+Math.sin(ae*2)*.06+Me)*Pe}p.set(t,T)}return i(w,p.get(t),{gain:.5,rate:1,priority:2})}let l=[];async function z(t){l=[...new Set(t)].slice(0,4);let y=new Set(l);for(let w of d)w.bus==="ambience"&&!y.has(w.id)&&ie(w);for(let w of l)!ee.has(w)&&![...d].some(W=>W.loop&&W.id===w&&!W.fading)&&(ee.add(w),g(w,{loop:!0,ambient:!0,bus:"ambience"}).finally(()=>ee.delete(w)))}async function j(t){if(!(!t.isTrusted||_||N)){pe();try{await f.resume(),await Promise.all([...n.keys()].map(y=>J(y).catch(w=>{!X.has(y)&&w.name!=="AbortError"&&(X.add(y),u("audio: "+w.message))}))),_||await z(l)}catch(y){_||u("audio: "+y.message)}}}return a.addEventListener("pointerdown",j,{signal:Q.signal}),document.addEventListener("keydown",j,{signal:Q.signal}),{play:g,cue:S,ambience:z,setRoom:Y,unlock:j,stop(t){if(n.has(t)){k.set(t,(k.get(t)||0)+1);for(let y of[...d])y.id===t&&G(y)}},setVolume(t){M=Math.max(0,Math.min(1,Number.isFinite(t)?t:.55)),He.set(a,{muted:H,volume:M}),fe()},setMuted(t){if(H=!!t,He.set(a,{muted:H,volume:M}),D++,H)for(let y of[...d])G(y);fe(),H||z(l)},setPaused(t){if(N!==!!t){if(N=!!t,D++,N){for(let y of[...d])G(y);f?.suspend().catch(()=>{})}else f&&f.resume().then(()=>z(l)).catch(()=>{});fe()}},listener(t,y=[0,0,-1],w=[0,1,0]){if(oe.splice(0,3,...t),!f)return;if(v==="2d")for(let U of d)B(U);let W=f.listener;for(let[U,te]of[["position",t],["forward",y],["up",w]])for(let T=0;T<3;T++)W[U+"XYZ"[T]]&&(W[U+"XYZ"[T]].value=te[T])},connectMusic(t){if(!f)throw Error("audio: unlock with player interaction first");if(t.context!==f)throw Error("audio: music must use the game AudioContext");return t.connect(L.music),()=>t.disconnect(L.music)},get context(){return f},get preferences(){return{muted:H,volume:M}},reset(){D++;for(let t of[...d])G(t);E.clear(),z(l)},stats(){return{voices:d.size,loops:[...d].filter(t=>t.loop).length,buffers:c.size,cue_buffers:p.size,state:f?.state||"locked"}},dispose(){if(!_){_=!0,Q.abort();for(let t of[...d])G(t);c.clear(),p.clear(),C.clear(),q?.disconnect(),$?.disconnect(),se?.disconnect(),Object.values(L).forEach(t=>t.disconnect()),O?.disconnect(),I?.disconnect(),f?.close().catch(()=>{})}}}}var it=(x,e,a)=>Math.max(e,Math.min(a,x));function It({adapter:x,config:e,root:a,report:u=console.warn,controls:v=!0}){e||={effects:[],sounds:[],bindings:[],quality:"auto"};let n=new Map((e.effects||[]).map(i=>[i.id,i])),c=new Map,C=new AbortController,d=tt({sounds:e.sounds,base:e.base,root:a,report:u,dimension:x.dimension}),E={events:{},effects:{},sounds:{}},k=(i,g)=>{(Object.keys(i).length<64||Object.hasOwn(i,g))&&(i[g]=Math.min(1e6,(i[g]||0)+1))},Q=new Map;for(let i of e.bindings||[])Q.set(i.event,[...Q.get(i.event)||[],i.sound]);let f=e.quality||"auto",O=2,I=0,se=!1,q=document.hidden,$=!1,_=0,N=0,H=!0,M=0,Z=matchMedia("(prefers-reduced-motion: reduce)"),L=Z.matches;function oe(){x.quality(O,L)}function ee(i,g={}){let p=n.get(i);if(!p)throw Error("presentation: unknown imported effect "+i);let S={...p.defaults};for(let[l,z]of Object.entries(g))if(["position","normal"].includes(l)){if(!Array.isArray(z)||z.length!==3||z.some(j=>!Number.isFinite(j)||Math.abs(j)>1e6))throw Error("presentation: invalid "+l);S[l]=z.slice()}else if(l==="color"){if(!/^#[a-f0-9]{6}$/i.test(z))throw Error("presentation: invalid color");S[l]=z}else if(l==="fixed"){if(typeof z!="boolean")throw Error("presentation: invalid fixed");S[l]=z}else if(["intensity","scale","lifetime","cycle","hour","width","depth","y","speed","density","radius"].includes(l)){if(!Number.isFinite(z))throw Error("presentation: invalid "+l);S[l]=it(z,l==="y"?-1e4:0,l==="cycle"?86400:l==="width"||l==="depth"?1e3:l==="hour"?24:l==="lifetime"?120:100)}else throw Error("presentation: unknown parameter "+l);return S}function X(i,g={}){let p=ee(i,g);return c.set(i,p),x.set(i,p),ie}function ne(i,g={}){if(se||q||$)return;let p=ee(i,g);if(i.startsWith("blood")&&!H){n.has("metal-sparks")&&x.emit("metal-sparks",p);return}x.emit(i,p),k(E.effects,i)}let D=new Map,pe={shot:["muzzle-flash"],hit:["blood-spray","blood-pool","blood-decal","metal-sparks","stone-debris","hit-flash"],pickup:["pickup-glow"],splash:["water-splash","water-ripple"],win:["magic"]};function fe(i,g=[0,0,0],p=[0,1,0],S="flesh"){if(se||q||$)return;k(E.events,i);for(let j of D.has(i)?D.get(i).effects:pe[i]||[])n.has(j)&&(!j.startsWith("blood")||S==="flesh")&&(j!=="metal-sparks"||S==="metal")&&(j!=="stone-debris"||S==="stone")&&ne(j,{position:g,normal:p});let l=D.has(i)?D.get(i).sounds:Q.get(i),z=l?.[Math.floor(Math.random()*l.length)];z?(k(E.sounds,z),d.play(z,{position:x.dimension==="2d"?[g[0],g[1],0]:g})):e.feedback===!0&&!D.has(i)&&d.cue(i)}function Y(){d.setPaused(se||q)}C.signal.addEventListener("abort",()=>d.dispose(),{once:!0}),document.addEventListener("visibilitychange",()=>{q=document.hidden,Y()},{signal:C.signal}),Z.addEventListener("change",i=>{L=i.matches,oe()},{signal:C.signal});let G=(n.get(e.environment)?.sounds||[]).filter(i=>e.sounds?.find(g=>g.id===i)?.loop);Q.has("ambient")&&G.push(...Q.get("ambient"));let ie={audio:d,set:X,emit:ne,event:fe,bindEvent(i,{effect:g,sound:p}={}){if(typeof i!="string"||!i||i.length>64)throw Error("presentation: invalid event name");if(g&&!n.has(g))throw Error("presentation: event effect was not imported "+g);if(p&&!e.sounds?.some(S=>S.id===p))throw Error("presentation: event sound was not imported "+p);return D.set(i,{effects:g?[g]:[],sounds:p?[p]:[]}),ie},setEnvironment(i){let g=n.get(i);if(g?.category!=="environment")throw Error("presentation: environment was not imported");c.clear(),x.clear?.();for(let p of g.effects)X(p);return G=g.sounds.filter(p=>e.sounds?.find(S=>S.id===p)?.loop),d.ambience(G),ie},setPaused(i){se=!!i,Y()},setActive(i){q=!i||document.hidden,Y()},setBlood(i){H=!!i,x.blood?.(H)},setQuality(i){if(!["auto","low","medium","high"].includes(i))throw Error("presentation: unknown quality");f=i,O=i==="low"?0:i==="medium"?1:2,_=N=0,oe()},registerSurface(...i){return x.registerSurface(...i)},applyObject(i,g,p){return x.applyObject(i,g,ee(g,p))},update(i){if(se||q||$)return;let g=it(i,0,.1);I+=g,f==="auto"&&(i>.025?(_+=g,N=0):(N+=g,_=Math.max(0,_-g)),_>2&&O>0&&(O--,_=0,oe()),N>12&&O<2&&(O++,N=0,oe())),x.update(g,I)?.thunder&&(M=I+1.2),M&&I>=M&&(M=0,e.sounds?.some(S=>S.id==="thunder")&&d.play("thunder",{gain:.4}));let p=c.get("day-night");if(p&&G.includes("forest-day")&&G.includes("forest-night")){let S=p.fixed?p.hour:(p.hour+I*24/Math.max(1,p.cycle))%24;d.ambience([S>6&&S<19?"forest-day":"forest-night",...G.filter(l=>!l.startsWith("forest-"))])}else d.ambience(G)},render(){x.render?.()},resize(){x.resize?.()},reset(){I=M=0,x.reset(),d.reset(),_=N=0;for(let i of Object.values(E))for(let g of Object.keys(i))delete i[g]},audit(){return{events:{...E.events},effects:{...E.effects},sounds:{...E.sounds}}},stats(){return{...x.stats(),...d.stats(),quality:["low","medium","high"][O],time:I}},dispose(){$||($=!0,C.abort(),B?.remove(),x.dispose())}},B;if(v&&(n.size||e.sounds?.length||e.feedback===!0)){B=document.createElement("div"),B.className="aurago-game-presentation",B.style.cssText="position:absolute;right:12px;top:12px;z-index:1100;display:flex;gap:8px;align-items:center;padding:7px 10px;border:1px solid #ffffff30;border-radius:12px;background:#101822d9;color:#fff;font:12px system-ui;max-width:90%";let i=document.createElement("button");i.textContent=d.preferences.muted?"\u266B \xD7":"\u266A",i.title="Sound",i.setAttribute("aria-label","Mute sound"),i.setAttribute("aria-pressed",String(d.preferences.muted));let g=d.preferences.muted;i.onclick=()=>{g=!g,i.textContent=g?"\u266B \xD7":"\u266A",i.setAttribute("aria-pressed",String(g)),d.setMuted(g)};let p=document.createElement("input");p.type="range",p.min="0",p.max="100",p.value=String(Math.round(d.preferences.volume*100)),p.style.width="68px",p.setAttribute("aria-label","Volume"),p.oninput=()=>d.setVolume(+p.value/100);let S=document.createElement("select");S.setAttribute("aria-label","Effects quality");for(let l of["auto","low","medium","high"])S.add(new Option(l,l));if(S.value=f,S.onchange=()=>ie.setQuality(S.value),B.append(i,p,S),[...n.keys()].some(l=>l.startsWith("blood"))){let l=document.createElement("button");l.textContent="\u25CF",l.title="Blood effects",l.setAttribute("aria-label","Blood effects"),l.setAttribute("aria-pressed","true"),l.onclick=()=>{ie.setBlood(!H),l.setAttribute("aria-pressed",String(H))},B.append(l)}B.style.colorScheme="dark",B.style.accentColor="#71dfce",B.style.flexWrap="wrap";for(let l of B.querySelectorAll("button,select"))l.style.cssText="font:inherit;color:inherit;border:1px solid #ffffff28;border-radius:7px;background:#243443;padding:6px;min-height:"+(matchMedia("(pointer: coarse)").matches?44:32)+"px";a.append(B)}for(let i of n.values())i.category!=="environment"&&X(i.id);return ie.setQuality(f),d.ambience(G),Y(),ie}var Gt=`attribute float size;attribute float alpha;attribute float kind;varying vec3 tint;varying float opacity;varying float shape;
void main(){tint=color;opacity=alpha;shape=kind;vec4 p=modelViewMatrix*vec4(position,1.);gl_Position=projectionMatrix*p;gl_PointSize=clamp(size*650./max(1.,-p.z),1.,kind<.5?34.:160.);}`,jt=`varying vec3 tint;varying float opacity;varying float shape;void main(){vec2 p=gl_PointCoord*2.-1.;float a;
if(shape<.5){p.x*=6.;a=(1.-smoothstep(.35,1.,length(p)))*.5;}
else if(shape<1.5){a=(1.-smoothstep(.05,1.,length(p)))*.5;}
else if(shape<2.5){a=1.-smoothstep(.65,1.,length(p));}
else if(shape<3.5){p.y*=1.8;a=1.-smoothstep(.5,1.,length(p));}
else {p.y*=3.;a=(1.-smoothstep(0.,1.,length(p)))*.16;}
if(a*opacity<.015)discard;gl_FragColor=vec4(tint,a*opacity);}`,Ht={uniforms:{tDiffuse:{value:null},time:{value:0},vignette:{value:0},grain:{value:0},heat:{value:0},underwater:{value:0},grade:{value:0}},vertexShader:"varying vec2 uv0;void main(){uv0=uv;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`uniform sampler2D tDiffuse;uniform float time,vignette,grain,heat,underwater,grade;varying vec2 uv0;
void main(){vec2 p=uv0;p.x+=(sin(p.y*32.+time*2.)*.002+sin(p.y*77.-time)*.001)*(heat+underwater);vec3 c=texture2D(tDiffuse,p).rgb;
c=mix(c,c*vec3(.55,.86,1.04),underwater*.6);c=mix(c,pow(max(c,vec3(0.)),vec3(.97))*vec3(1.035,1.01,.96),grade*.4);
c*=1.-vignette*.6*smoothstep(.15,.8,length(uv0-.5));c+=(fract(sin(dot(uv0+time,vec2(12.9898,78.233)))*43758.5453)-.5)*grain*.035;gl_FragColor=vec4(c,1.);}`};function Di({scene:x,camera:e,renderer:a,sun:u,ambient:v}){let n=new m.Group;n.name="AuraGo presentation",x.add(n);let c=new Map,C=new Map,d=[],E=[],k=[],Q=[],f=new Map,O=2,I=!1,se=!0,q=0,$=!1,_=71237,N=0,H=6,M=()=>(_^=_<<13,_^=_>>>17,_^=_<<5,(_>>>0)/4294967296),Z=new m.Vector3,L=new m.Vector3,oe=new m.Raycaster,ee=new m.Vector3,X={background:x.background,fog:x.fog,sun:u&&{color:u.color.clone(),intensity:u.intensity,position:u.position.clone()},ambient:v?.intensity},ne=4e3,D=new Float32Array(ne*3),pe=new Float32Array(ne*3),fe=new Float32Array(ne),Y=new Float32Array(ne),J=new Float32Array(ne),G=new m.BufferGeometry;for(let[r,h,F]of[["position",D,3],["color",pe,3],["size",fe,1],["alpha",Y,1],["kind",J,1]])G.setAttribute(r,new m.BufferAttribute(h,F).setUsage(m.DynamicDrawUsage));G.setDrawRange(0,0);let ie=new m.ShaderMaterial({vertexShader:Gt,fragmentShader:jt,vertexColors:!0,transparent:!0,depthWrite:!1}),B=new m.Points(G,ie);B.frustumCulled=!1,n.add(B),d.push(G,ie);let i,g,p,S,l,z,j,t,y,w,W=new m.Color,U=new m.Color;function te(){if(i)return;i=new ke,i.scale.setScalar(210),i.material.uniforms.cloudCoverage.value=0,i.material.uniforms.auraSkyColor={value:new m.Color(.42,.62,.78)},i.material.fragmentShader=`uniform vec3 auraSkyColor;
`+i.material.fragmentShader.replace("gl_FragColor = vec4( texColor, 1.0 );","vec3 d=normalize(vWorldPosition-cameraPosition);vec3 atmosphere=mix(auraSkyColor,auraSkyColor*vec3(.1,.28,.52),pow(max(0.,d.y),.45));gl_FragColor = vec4(mix(atmosphere,clamp(texColor*.035,0.,1.),.3),1.0);"),n.add(i),d.push(i.geometry,i.material);let r=new m.BufferGeometry,h=new Float32Array(1200*3);for(let R=0;R<1200;R++)L.set(M()-.5,M()-.5,M()-.5).normalize().multiplyScalar(190),h.set(L.toArray(),R*3);r.setAttribute("position",new m.BufferAttribute(h,3));let F=new m.PointsMaterial({color:13164287,size:.65,transparent:!0,depthWrite:!1,fog:!1});g=new m.Points(r,F),n.add(g),d.push(r,F);let b=new m.SphereGeometry(4,24,16),s=new m.MeshBasicMaterial({color:14147815,fog:!1});S=new m.Mesh(b,s),S.position.set(-80,100,-120),n.add(S),d.push(b,s);let o=new m.SphereGeometry(205,32,16),P=new m.ShaderMaterial({side:m.BackSide,transparent:!0,depthWrite:!1,uniforms:{time:{value:0},coverage:{value:.5},night:{value:0}},vertexShader:"varying vec3 p;void main(){p=position;gl_Position=projectionMatrix*modelViewMatrix*vec4(position,1.);}",fragmentShader:`varying vec3 p;uniform float time,coverage,night;
float hash(vec2 p){return fract(sin(dot(p,vec2(127.1,311.7)))*43758.5453);}float noise(vec2 x){vec2 i=floor(x),f=fract(x);f=f*f*(3.-2.*f);return mix(mix(hash(i),hash(i+vec2(1,0)),f.x),mix(hash(i+vec2(0,1)),hash(i+1.),f.x),f.y);}
void main(){vec3 d=normalize(p);vec2 uv=night>1.1?vec2(atan(d.z,d.x),asin(d.y))*3.:d.xz/max(.12,d.y)*1.7+vec2(time*.015,0);float n=0.,a=.55;for(int i=0;i<5;i++){n+=a*noise(uv);uv=uv*2.03+.71;a*=.5;}float c=smoothstep(1.-coverage,1.15-coverage,n)*(night>1.1?1.:smoothstep(0.,.2,d.y));gl_FragColor=night>1.1?vec4(mix(vec3(.05,.12,.3),vec3(.3,.07,.4),n),c*.45):vec4(mix(vec3(.78,.87,.95),vec3(.09,.12,.18),night),c*.42);}`});p=new m.Mesh(o,P),n.add(p),d.push(o,P)}function T(){j||(j=new Ue(a),j.addPass(new Ne(x,e)),t=new _e(new m.Vector2(512,512),.3,.35,.8),j.addPass(t),y=new Ce(Ht),j.addPass(y),w=new We,j.addPass(w),Me())}function le(r,h){l&&(n.remove(l),l.geometry.dispose(),l.material.dispose(),l.dispose(),z.dispose());let F=new Uint8Array(4096*4);for(let o=0;o<4096;o++)F[o*4]=128+Math.sin(o*.37)*32,F[o*4+1]=128+Math.cos(o*.51)*32,F[o*4+2]=245,F[o*4+3]=255;z=new m.DataTexture(F,64,64),z.wrapS=z.wrapT=m.RepeatWrapping,z.magFilter=z.minFilter=m.LinearFilter,z.needsUpdate=!0;let b=new m.PlaneGeometry(h.width||80,h.depth||80,32,32);l=new De(b,{textureWidth:512,textureHeight:512,waterNormals:z,sunDirection:new m.Vector3(1,1,1),sunColor:16772305,waterColor:r==="water-river"?2387301:1204096,distortionScale:r==="water-ocean"?3:1.2,fog:!!x.fog}),l.material.fragmentShader=l.material.fragmentShader.replace("vec3 outgoingLight = albedo;","vec3 outgoingLight = mix(albedo,waterColor,.4);").replace("gl_FragColor = vec4( outgoingLight, alpha );",`
            float wave=sin(worldPosition.x*.32+time)*cos(worldPosition.z*.24-time*.6);
            float foam=smoothstep(.94,.995,wave)*(.3+.7*sin(worldPosition.x*8.+worldPosition.z*9.)*sin(worldPosition.x*8.+worldPosition.z*9.));
            outgoingLight=mix(outgoingLight,vec3(.65,.87,.9),foam*.25);
            gl_FragColor = vec4(outgoingLight, alpha);`),l.rotation.x=-Math.PI/2,l.position.fromArray(h.position||[0,h.y??-.05,20]),l.userData.base=b.attributes.position.array.slice(),l.userData.id=r,l.userData.speed=h.speed||.6;let s=l.onBeforeRender;l.onBeforeRender=function(...o){O===2&&s.apply(this,o)},n.add(l)}function ae(r,h){if(c.set(r,h),r.startsWith("sky-")||r==="day-night"){for(let F of c.keys())r.startsWith("sky-")&&F.startsWith("sky-")&&F!==r&&c.delete(F);te()}["water-lake","water-river","water-ocean"].includes(r)&&le(r,h),["bloom","color-grade","vignette","film-grain","heat-haze","underwater"].includes(r)&&T(),r==="fog-distance"&&(x.fog=new m.FogExp2(10268850,h.density??.018))}function me(r,h){return C.size?(oe.set(L.set(r,200,h),ee.set(0,-1,0)),oe.intersectObjects([...C.keys()],!0)[0]?.point.y??0):0}function V(r,h,F){let b=[500,1500,4e3][O];if(E.length>=b)return;let s=r.startsWith("rain"),o=r==="snow",P=r==="smoke"||r.startsWith("fog"),R=r==="wind-leaves",A=r.startsWith("blood"),re=r==="fire",ce=r==="engine-trail",ge=r==="water-splash",ue=r==="stone-debris",ot=A?h.color||"#9f162a":P?"#9caebb":s||ge?"#b3d7ed":ue?"#a69b89":o?"#f5faff":R?"#9eae55":ce||r==="magic"||r==="teleport"||r==="pickup-glow"?"#76ddff":re?"#ff792e":"#ffbd57";W.set(h.color&&h.color!=="#ffffff"?h.color:ot);let Ee=F||h.position||[0,1,0],Te=h.scale||1;E.push({id:r,x:Ee[0],y:Ee[1],z:Ee[2],vx:(M()-.5)*(s?2:6)*Te,vy:s?-26:o?-1.5:P?.5+M():(M()*5+1)*Te,vz:(M()-.5)*(s?1:6)*Te,age:0,life:s?2.5:o?12:P?5:h.lifetime||1.5,size:(s?.25:o?.09:P?1.8:A?.11:.18)*Te,c:W.toArray(),kind:s?0:r.startsWith("fog")?4:P?1:R?3:2,floor:s||o?me(Ee[0],Ee[2]):0});let K=E[E.length-1];if((re||ce)&&(K.kind=1,K.size=(re?.5:.25)*Te,K.life=re?1:.5,K.vx*=.1,K.vz*=.1,K.vy=re?2:0,ce)){let Oe=h.normal||[0,0,-1];K.vx=Oe[0]*5,K.vy=Oe[1]*5,K.vz=Oe[2]*5}(r==="muzzle-flash"||r==="hit-flash")&&(K.life=.12,K.size=.6*Te,K.kind=1,K.vx=K.vy=K.vz=0),h.normal&&(A||r==="metal-sparks"||ue||ge)&&(K.vx+=h.normal[0]*3,K.vy+=h.normal[1]*3,K.vz+=h.normal[2]*3),(r==="wind-dust"||R)&&(K.vx=3,K.vy=-.15,K.life=8,K.size=R?.16:.5,K.kind=R?3:1)}function xe(r,h){if(!se)return;for(r==="blood-pool"&&(h={...h,position:[h.position?.[0]||0,me(h.position?.[0]||0,h.position?.[2]||0),h.position?.[2]||0],normal:[0,1,0]});k.length>=[24,48,96][O];){let A=k.shift();A.mesh.removeFromParent(),A.mesh.geometry.dispose(),A.mesh.material.dispose(),A.tex?.dispose()}let F=document.createElement("canvas");F.width=F.height=96;let b=F.getContext("2d");b.fillStyle=h.color||"#8c1625",b.beginPath();for(let A=0;A<=24;A++){let re=A/24*Math.PI*2,ce=25+M()*19,ge=48+Math.cos(re)*ce,ue=48+Math.sin(re)*ce;A?b.lineTo(ge,ue):b.moveTo(ge,ue)}b.fill();for(let A=0;A<14;A++)b.beginPath(),b.arc(M()*96,M()*96,1+M()*3,0,7),b.fill();let s=new m.CanvasTexture(F),o=new m.PlaneGeometry((r==="blood-pool"?1.2:.5)*(h.scale||1),(r==="blood-pool"?1:.6)*(h.scale||1)),P=new m.MeshBasicMaterial({map:s,transparent:!0,depthWrite:!1,polygonOffset:!0,polygonOffsetFactor:-2,side:m.DoubleSide}),R=new m.Mesh(o,P);ee.fromArray(h.normal||[0,1,0]),ee.lengthSq()<.001&&ee.set(0,1,0),ee.normalize(),R.quaternion.setFromUnitVectors(new m.Vector3(0,0,1),ee),R.position.fromArray(h.position||[0,0,0]).addScaledVector(ee,.006),n.add(R),k.push({mesh:R,tex:s,age:0,life:h.lifetime||30})}function he(r,h){if(!["blood-pool","blood-decal","blood-spray","hit-flash","metal-sparks","stone-debris","fire","smoke","embers","explosion","muzzle-flash","engine-trail","magic","teleport","pickup-glow","water-splash","water-ripple"].includes(r))return;if(r==="blood-pool"||r==="blood-decal"){xe(r,h);return}if(I&&(r==="hit-flash"||r==="muzzle-flash"))return;if(r==="hit-flash"){for(let b of Q)b.id==="hit-flash"&&(b.age=0);V(r,{...h,color:"#efffff",lifetime:.08,scale:2});return}let F=Math.round((r==="explosion"?90:r==="muzzle-flash"?8:30)*Math.min(2,h.intensity??1));if(r!=="water-ripple")for(let b=0;b<F;b++)V(r,h);if(r==="explosion"){for(let b=0;b<18;b++)V("smoke",{...h,color:"#515760",scale:1.2,lifetime:3});for(let b of E.slice(-F-18,-18))b.vx*=2,b.vz*=2,b.vy*=1.5}if(r==="water-ripple"){for(;k.length>=[24,48,96][O];){let P=k.shift();P.mesh.removeFromParent(),P.mesh.geometry.dispose(),P.mesh.material.dispose(),P.tex?.dispose()}let b=new m.RingGeometry(.09,.1,32),s=new m.MeshBasicMaterial({color:11789555,transparent:!0,opacity:.65,side:m.DoubleSide,depthWrite:!1}),o=new m.Mesh(b,s);o.rotation.x=-Math.PI/2,o.position.fromArray(h.position||[0,.02,0]),n.add(o),k.push({mesh:o,age:0,life:h.lifetime||1.4,ripple:!0,scale:h.scale||1})}}function ye(r,h){let F=!1;if(q=h,e.getWorldPosition(Z),i&&!c.has("day-night")&&![...c.keys()].some(s=>s.startsWith("sky-"))&&(i.visible=g.visible=p.visible=S.visible=!1),i&&(c.has("day-night")||[...c.keys()].some(s=>s.startsWith("sky-")))){let s=c.get("day-night"),o=[...c.keys()].find(ce=>ce.startsWith("sky-"))||"sky-clear",R=((s?s.fixed?s.hour:(s.hour+h*24/Math.max(1,s.cycle))%24:o==="sky-night"||o==="sky-space"?0:o==="sky-sunset"?17.4:11)-6)/24*Math.PI*2,A=Math.max(0,Math.sin(R)),re=1-Math.min(1,A*3);i.position.copy(Z),p.position.copy(Z),g.position.copy(Z),S.position.copy(Z).add(L.set(-80,100,-120)),i.material.uniforms.sunPosition.value.set(Math.cos(R)*100,Math.sin(R)*100,-30),i.material.uniforms.turbidity.value=o==="sky-storm"?18:5,i.material.uniforms.rayleigh.value=2,i.material.uniforms.auraSkyColor.value.set(o==="sky-storm"?6714244:A<.3?14189931:8371940),i.visible=re<.95,g.visible=re>.15,g.material.opacity=re,S.visible=o!=="sky-space"&&re>.4,re>.7&&(x.background=U.set(o==="sky-space"?396065:1121334)),u&&(u.position.set(Math.cos(R)*35,Math.sin(R)*40,15),u.intensity=.12+A*2.7,u.color.setRGB(1,.65+A*.28,.48+A*.4)),v&&(v.intensity=.3+A*1.3),p.material.uniforms.time.value=I?0:q,p.material.uniforms.coverage.value=o==="sky-cloudy"?.65:o==="sky-storm"?.85:.38,p.material.uniforms.night.value=re,p.visible=!0,p.material.uniforms.night.value=o==="sky-space"?1.5:re,x.fog&&x.fog.color.setRGB(.12+A*.45,.16+A*.48,.24+A*.45)}let b=[...c.keys()].find(s=>["rain-light","rain-heavy","snow","wind-dust","wind-leaves"].includes(s));if(b){N+=r*(b==="rain-heavy"?650:b==="rain-light"?220:b==="snow"?75:25)*[.3,.65,1][O]*Math.min(3,c.get(b).intensity??1);let s=Math.min(32,Math.floor(N));for(N=Math.min(1,N-s);s-- >0;)V(b,c.get(b),[Z.x+(M()-.5)*34,Z.y+8+M()*7,Z.z+(M()-.5)*34])}for(let s of["fire","smoke","embers","engine-trail","fog-ground","fog-zone"])if(c.has(s)&&(s.startsWith("fog")||c.get(s).position)&&M()<r*18*Math.min(3,c.get(s).intensity??1)){let o=c.get(s),P=s.startsWith("fog"),R=o.position||[Z.x,.5,Z.z],A=s==="fog-zone"?o.radius||12:18;V(s,{...o,scale:P?5*(o.scale||1):o.scale,lifetime:P?8:o.lifetime||2},P?[R[0]+(M()-.5)*A*2,R[1],R[2]+(M()-.5)*A*2]:R)}c.has("thunderstorm")&&q>H&&(F=!0,u&&!I&&(u.intensity=5),H=q+5+M()*12);for(let s=E.length-1;s>=0;s--){let o=E[s];if(o.age+=r,o.age>o.life){E.splice(s,1);continue}if(o.x+=o.vx*r,o.y+=o.vy*r,o.z+=o.vz*r,!o.id.startsWith("rain")&&o.id!=="snow"&&o.kind!==1&&o.kind!==4&&(o.vy-=6*r),o.y<o.floor){o.id.startsWith("rain")&&O>0&&M()<.08&&he("water-ripple",{position:[o.x,o.floor+.02,o.z],lifetime:.6,scale:.25}),E.splice(s,1);continue}}for(let s=0;s<E.length;s++){let o=E[s];D.set([o.x,o.y,o.z],s*3),pe.set(o.c,s*3),fe[s]=o.size*(o.kind===1?1+o.age*.2:1),Y[s]=(o.id.startsWith("rain")?Math.min(1,o.age*8):1)*(1-o.age/o.life),J[s]=o.kind}G.setDrawRange(0,E.length);for(let s of Object.values(G.attributes))s.needsUpdate=!0;for(let s=k.length-1;s>=0;s--){let o=k[s];o.age+=r,o.mesh.material.opacity=Math.min(1,(o.life-o.age)/Math.min(5,o.life)),o.ripple&&o.mesh.scale.setScalar((1+o.age*7)*o.scale),o.age>o.life&&(o.mesh.removeFromParent(),o.mesh.geometry.dispose(),o.mesh.material.dispose(),o.tex?.dispose(),k.splice(s,1))}for(let s of Q)s.age+=r,s.clock.value=I?0:q,s.progress.value=s.id==="dissolve"?Math.min(1,s.age/s.life):s.id==="hit-flash"?Math.max(0,1-s.age*8):0;if(l){l.material.uniforms.time.value=q*l.userData.speed;let s=l.geometry.attributes.position,o=l.userData.base;for(let P=0;P<s.count;P++)s.array[P*3+2]=Math.sin(o[P*3]*.3+q)*Math.cos(o[P*3+1]*.25-q*.6)*(l.userData.id==="water-ocean"?.25:.035);s.needsUpdate=!0,l.geometry.computeVertexNormals(),l.material.uniforms.sunDirection.value.copy(u?.position||L.set(1,1,1)).normalize()}if(j){t.enabled=O>0&&c.has("bloom"),t.strength=.3*(c.get("bloom")?.intensity??1);let s=y.uniforms;s.time.value=q;for(let[o,P]of[["vignette","vignette"],["grain","film-grain"],["heat","heat-haze"],["underwater","underwater"],["grade","color-grade"]])s[o].value=c.has(P)?I&&["heat","underwater"].includes(o)?0:Math.min(2,c.get(P).intensity??1):0}return{thunder:F}}function Me(){let r=a.getSize(new m.Vector2);j?.setSize(r.x,r.y)}function Pe(){E.length=0;for(let r of k)r.mesh.removeFromParent(),r.mesh.geometry.dispose(),r.mesh.material.dispose(),r.tex?.dispose();k.length=0,G.setDrawRange(0,0),N=0,H=6,_=71237;for(let r of Q)r.age=0,r.progress.value=0,r.clock.value=0}return{dimension:"3d",set:ae,emit:he,update:ye,resize:Me,reset:Pe,clear(){c.clear(),Pe(),x.fog=X.fog,i&&(i.visible=g.visible=p.visible=S.visible=!1),l&&(l.removeFromParent(),l.geometry.dispose(),l.material.dispose(),l.dispose(),z.dispose(),l=null)},quality(r,h){O=r,I=h,j&&j.setPixelRatio(Math.min(a.getPixelRatio(),r===0?.75:r===1?1:1.5))},blood(r){if(se=r,!se)for(let h of k)h.ripple||(h.life=0)},render(){j?j.render(0):a.render(x,e)},registerSurface(r,h="ground"){return C.set(r,h),()=>C.delete(r)},applyObject(r,h,F={}){if(!["hologram","dissolve","hit-flash"].includes(h))throw Error("presentation: unsupported object effect");if(I&&h==="hit-flash")return()=>{};f.get(r)?.(),f.size>=128&&f.values().next().value();let b=[];r.traverse(P=>{if(!P.isMesh)return;let R=P.material,A=(Array.isArray(R)?R:[R]).map(re=>{let ce=re.clone(),ge={material:ce,id:h,age:0,life:F.lifetime||2,progress:{value:0},clock:{value:0}};return ce.transparent=!0,ce.onBeforeCompile=ue=>{ue.uniforms.auraProgress=ge.progress,ue.uniforms.auraTime=ge.clock,ue.vertexShader=`varying vec3 auraPosition;
`+ue.vertexShader.replace("#include <begin_vertex>",`#include <begin_vertex>
auraPosition=position;`),ue.fragmentShader=`uniform float auraProgress,auraTime;varying vec3 auraPosition;
`+ue.fragmentShader.replace("#include <dithering_fragment>",h==="dissolve"?"float n=fract(sin(dot(floor(auraPosition*35.),vec3(12.9898,78.233,37.719)))*43758.5453);if(n<auraProgress)discard;gl_FragColor.rgb+=vec3(1.,.3,.04)*(1.-smoothstep(0.,.07,n-auraProgress));":h==="hologram"?"float scan=.6+.4*sin(auraPosition.y*90.-auraTime*5.);gl_FragColor=vec4(vec3(.13,.72,1.)*scan,.38+scan*.3);":"gl_FragColor.rgb=mix(gl_FragColor.rgb,vec3(1.),auraProgress);")},ce.customProgramCacheKey=()=>"aurago-"+h,Q.push(ge),b.push(()=>{ce.dispose();let ue=Q.indexOf(ge);ue>=0&&Q.splice(ue,1)}),ce});P.material=Array.isArray(R)?A:A[0],b.push(()=>{P.material=R})});let s=!1,o=()=>{s||(s=!0,b.forEach(P=>P()),f.delete(r))};return f.set(r,o),o},stats(){return{particles:E.length,decals:k.length,surfaces:C.size,objects:Q.length}},dispose(){if(!$){$=!0,Pe(),n.removeFromParent();for(let r of f.values())r();d.forEach(r=>r.dispose()),l&&(l.geometry.dispose(),l.material.dispose(),l.dispose(),z.dispose()),j?.dispose(),t?.dispose(),y?.dispose(),w?.dispose(),C.clear(),x.background=X.background,x.fog=X.fog,u&&X.sun&&(u.color.copy(X.sun.color),u.intensity=X.sun.intensity,u.position.copy(X.sun.position)),v&&(v.intensity=X.ambient)}}}}export{It as createPresentation,Di as createThreeAdapter};
