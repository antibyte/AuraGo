// MIT. Phaser 4's public filter contract; one render node per game renderer.
const shader=`#pragma phaserTemplate(shaderName)
precision mediump float;
uniform sampler2D uMainSampler;
uniform float auraTime;
uniform vec4 auraFlags,auraMore,auraObject;
varying vec2 outTexCoord;
#pragma phaserTemplate(fragmentHeader)
float hash(vec2 p){return fract(sin(dot(p,vec2(12.9898,78.233)))*43758.5453);}
void main(){
 vec2 uv=outTexCoord,p=uv;p.x+=(sin(uv.y*42.+auraTime*2.)*.002+sin(uv.y*97.-auraTime)*.001)*(auraFlags.z+auraFlags.w);
 vec4 c=boundedSampler(uMainSampler,p);
 if(auraMore.x>0.){vec3 glow=vec3(0.);for(int i=0;i<4;i++){vec2 d=vec2(float(i/2)*2.-1.,mod(float(i),2.)*2.-1.)*.003;vec3 s=boundedSampler(uMainSampler,p+d).rgb;glow+=max(vec3(0.),s-.65);}c.rgb+=glow*.12*auraMore.x;}
 c.rgb=mix(c.rgb,c.rgb*vec3(1.05,1.015,.95),auraMore.y*.4);
 c.rgb*=vec3(1.-auraMore.z,1.-auraMore.z*.93,1.-auraMore.z*.8);
 c.rgb=mix(c.rgb,c.rgb*vec3(.55,.85,1.1),auraFlags.w*.6);
 c.rgb*=1.-auraFlags.x*.55*smoothstep(.2,.8,length(uv-.5));c.rgb+=(hash(uv+auraTime)-.5)*auraFlags.y*.035;
 if(auraObject.x>2.5)c.rgb=mix(c.rgb,vec3(1.),auraObject.y);
 else if(auraObject.x>1.5){float n=hash(floor(uv*180.));if(n<auraObject.y)discard;c.rgb+=vec3(1.,.3,.05)*(1.-smoothstep(0.,.07,n-auraObject.y));}
 else if(auraObject.x>.5){float scan=.6+.4*sin(uv.y*450.-auraTime*5.);c.rgb=vec3(.13,.72,1.)*scan*c.a;c.a*=.38+scan*.3;}
 gl_FragColor=c;
}`;
export function addFilter(scene,camera){
    const P=globalThis.Phaser,manager=scene.game.renderer.renderNodes,name='AuraPresentation1';
    if(!manager.hasNode(name)){
        class PresentationFilter extends P.Renderer.WebGL.RenderNodes.BaseFilterShader {
            constructor(manager){super(name,manager,null,shader)}
            setupUniforms(c){for(const [name,value] of Object.entries(c.uniforms))this.programManager.setUniform(name,value)}
        }
        manager.addNodeConstructor(name,PresentationFilter);
    }
    const controller=new P.Filters.Controller(camera,name);
    controller.uniforms={auraTime:0,auraFlags:[0,0,0,0],auraMore:[0,0,0,0],auraObject:[0,0,0,0]};
    camera.filters.internal.add(controller);return controller;
}
