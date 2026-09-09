/* Local botanical textures on curved leaf meshes; ceramic and flowers in Three.js r128. */
(function () {
    'use strict';
    const T = window.THREE, G = window.AuraLeafyGeometry;
    function leafGeometry(variant=0,rows=12,cols=8) {
        const positions=[],uv=[],indices=[];
        for(let j=0;j<=rows;j++)for(let i=0;i<=cols;i++){
            const t=j/rows,u=i/cols*2-1;
            const w=Math.pow(Math.sin(Math.PI*t),.58)*(.42-.23*t)*(1-variant*.055);
            positions.push(u*w,t+.16*Math.pow(1-t,5)*Math.pow(Math.abs(u),.65),
                (.03*(1-u*u)-.012*Math.pow(Math.abs(u),3))*Math.sin(Math.PI*t)+.09*t*t+.022*Math.sin(t*5+u*2+variant)*t);
            uv.push(i/cols,t);
        }
        for(let j=0;j<rows;j++)for(let i=0;i<cols;i++){const a=j*(cols+1)+i;indices.push(a,a+1,a+cols+2,a,a+cols+2,a+cols+1);}
        const g=new T.BufferGeometry();g.setAttribute('position',new T.Float32BufferAttribute(positions,3));g.setAttribute('uv',new T.Float32BufferAttribute(uv,2));g.setIndex(indices);g.computeVertexNormals();return g;
    }
    let artwork = null, preparing = null;
    function prepare() {
        if (artwork) return Promise.resolve();
        if (preparing) return preparing;
        preparing = new Promise((resolve, reject) => {
            const image = new Image();
            image.onload = () => {
                try {
                    const canvas = document.createElement('canvas');
                    canvas.width = image.naturalWidth; canvas.height = image.naturalHeight;
                    const ctx = canvas.getContext('2d', {willReadFrequently:true});
                    ctx.drawImage(image,0,0);
                    const pixels = ctx.getImageData(0,0,canvas.width,canvas.height).data, regions = [];
                    // Generated leaves are not exactly centered; split at the actual clear gutter.
                    function gutter(vertical) {
                        const span=vertical?canvas.width:canvas.height,depth=vertical?canvas.height:canvas.width;
                        for(let offset=0;offset<span*.08;offset++)for(const direction of [1,-1]){
                            const line=Math.floor(span/2)+offset*direction;
                            let clear=true;
                            for(let i=0;i<depth;i++){
                                const index=vertical?i*canvas.width+line:line*canvas.width+i;
                                if(pixels[index*4+3]>=64){clear=false;break;}
                            }
                            if(clear)return line;
                        }
                        throw Error('Missing Leafy atlas gutter');
                    }
                    const xs=[0,gutter(true),canvas.width],ys=[0,gutter(false),canvas.height];
                    for (let v=0;v<4;v++) {
                        const left=xs[v%2],right=xs[v%2+1],top=ys[Math.floor(v/2)],bottom=ys[Math.floor(v/2)+1];
                        let x0=right,y0=bottom,x1=left,y1=top;
                        for(let y=top;y<bottom;y++)for(let x=left;x<right;x++){
                            if(pixels[(y*canvas.width+x)*4+3]<64)continue;
                            x0=Math.min(x0,x);y0=Math.min(y0,y);x1=Math.max(x1,x);y1=Math.max(y1,y);
                        }
                        if(x1-x0<32||y1-y0<32)throw Error('Missing Leafy blade');
                        // The petiole defines the attachment, independent of unequal basal lobes.
                        const mid=(x0+x1)/2, band=(x1-x0)*.055;
                        let base=mid,found=false;
                        for(let y=y1;y>y1-(y1-y0)*.12&&!found;y--){
                            let sum=0,count=0;
                            for(let x=Math.ceil(mid-band);x<=mid+band;x++)if(pixels[(y*canvas.width+x)*4+3]>128){sum+=x;count++;}
                            if(count){base=sum/count;found=true;}
                        }
                        regions.push({x0:Math.max(left,x0-2),y0:Math.max(top,y0-2),x1:Math.min(right-1,x1+2),y1:Math.min(bottom-1,y1+2),base});
                    }
                    // Repack without resampling, leaving 16 transparent pixels against mip bleed.
                    const packed=document.createElement('canvas');
                    const cell=Math.ceil(Math.max(...regions.map(r=>Math.max(r.x1-r.x0,r.y1-r.y0))))+32;
                    packed.width=packed.height=cell*2;
                    const target=packed.getContext('2d');
                    regions.forEach((r,v)=>{
                        const x=v%2*cell+16,y=Math.floor(v/2)*cell+16,w=r.x1-r.x0,h=r.y1-r.y0;
                        target.drawImage(image,r.x0,r.y0,w,h,x,y,w,h);
                        r.base+=x-r.x0;r.x0=x;r.y0=y;r.x1=x+w;r.y1=y+h;
                    });
                    artwork={image:packed,regions,width:packed.width,height:packed.height}; resolve();
                } catch(err) {preparing=null;reject(err);}
            };
            image.onerror=()=>{preparing=null;reject(Error('Leafy artwork unavailable'));};
            const path='/img/leafy/leaves-natural.png';
            image.src=window.AuraLazyAssets?.versionedURL(path)||path;
        });
        return preparing;
    }
    function bladeGeometry(variant) {
        const region=artwork.regions[variant%4],height=region.y1-region.y0;
        const positions=[],uv=[],indices=[],rows=12,cols=8;
        const curl=[.07,-.035,.13,.04,-.055,.09,.02,.11][variant];
        const twist=[-.08,.04,.07,-.06,.05,-.035,.085,-.045][variant];
        for(let j=0;j<=rows;j++)for(let i=0;i<=cols;i++){
            const t=j/rows,u=i/cols*2-1,pixelX=region.x0+(region.x1-region.x0)*i/cols;
            const x=(pixelX-region.base)/height;
            const edge=Math.pow(Math.abs(u),3)*Math.sin(Math.PI*t);
            positions.push(x+.025*Math.sin(t*3.8+variant)*t,
                t+.009*Math.sin(t*8+variant)*edge,
                .018*(1-u*u)*Math.sin(Math.PI*t)+curl*t*t+twist*u*t+.012*Math.sin(t*11+variant)*edge);
            uv.push(pixelX/artwork.width,1-(region.y1-t*height)/artwork.height);
        }
        for(let j=0;j<rows;j++)for(let i=0;i<cols;i++){const a=j*(cols+1)+i;indices.push(a,a+1,a+cols+2,a,a+cols+2,a+cols+1);}
        const geometry=new T.BufferGeometry();
        geometry.setAttribute('position',new T.Float32BufferAttribute(positions,3));
        geometry.setAttribute('uv',new T.Float32BufferAttribute(uv,2));
        geometry.setIndex(indices);geometry.computeVertexNormals();return geometry;
    }
    function bladeMaterial(texture) {
        return new T.MeshPhysicalMaterial({
            map:texture,roughness:.64,metalness:0,envMapIntensity:.18,
            clearcoat:.07,clearcoatRoughness:.52,
            alphaTest:.26,alphaToCoverage:true,side:T.DoubleSide
        });
    }
    function merge(geometries) {
        const pos=[],norm=[],uv=[];
        for(const geo of geometries){
            const g=geo.index?geo.toNonIndexed():geo;
            pos.push(...g.attributes.position.array);norm.push(...g.attributes.normal.array);
            if(g.attributes.uv)uv.push(...g.attributes.uv.array);
            if(g!==geo)g.dispose();geo.dispose();
        }
        const out=new T.BufferGeometry();out.setAttribute('position',new T.Float32BufferAttribute(pos,3));out.setAttribute('normal',new T.Float32BufferAttribute(norm,3));
        if(uv.length)out.setAttribute('uv',new T.Float32BufferAttribute(uv,2));return out;
    }
    function studioEnvironment(renderer) {
        const c=document.createElement('canvas');c.width=512;c.height=256;const x=c.getContext('2d');
        const gr=x.createLinearGradient(0,0,0,256);gr.addColorStop(0,'#d9e7ed');gr.addColorStop(.4,'#697879');gr.addColorStop(.53,'#131b21');gr.addColorStop(1,'#55513c');x.fillStyle=gr;x.fillRect(0,0,512,256);
        for(const [p,w]of [[70,28],[360,65]]){const g=x.createLinearGradient(p-w,0,p+w,0);g.addColorStop(0,'rgba(255,255,255,0)');g.addColorStop(.5,'rgba(255,249,230,.9)');g.addColorStop(1,'rgba(255,255,255,0)');x.fillStyle=g;x.fillRect(p-w,15,w*2,160);}
        const texture=new T.CanvasTexture(c);texture.encoding=T.sRGBEncoding;texture.mapping=T.EquirectangularReflectionMapping;
        const pm=new T.PMREMGenerator(renderer),target=pm.fromEquirectangular(texture);texture.dispose();pm.dispose();return target;
    }
    function potGroup(light,scale,moisture) {
        const group=new T.Group(),ceramic=new T.MeshStandardMaterial({color:light?0xe5dfca:0x111918,roughness:.21,metalness:.3,envMapIntensity:.65});
        const gold=new T.MeshStandardMaterial({color:0x9c763b,roughness:.28,metalness:.8});
        const soil=new T.MeshStandardMaterial({color:new T.Color(moisture>20?0x272019:0x61503a).convertSRGBToLinear(),roughness:1,envMapIntensity:.05});
        const profiles=[[37,3],[47,5],[51,16],[61,103],[62,117],[59,121],[54,119],[53,111],[48,100]];
        const body=new T.Mesh(new T.LatheGeometry(profiles.map(p=>new T.Vector2(...p)),64),ceramic);group.add(body);
        for(const [radius,y,tube] of [[59,118,2.2],[45,5,2],[55,0,2]]){
            const ring=new T.Mesh(new T.TorusGeometry(radius,tube,8,64),gold);ring.rotation.x=Math.PI/2;ring.position.y=y;group.add(ring);
        }
        const dirt=new T.Mesh(new T.CircleGeometry(53,48),soil);dirt.rotation.x=-Math.PI/2;dirt.position.y=111;group.add(dirt);
        const moss=new T.InstancedMesh(new T.IcosahedronGeometry(1,1),new T.MeshStandardMaterial({color:new T.Color(0x394b16).convertSRGBToLinear(),roughness:.95,envMapIntensity:.07}),38),dummy=new T.Object3D();
        for(let i=0;i<38;i++){const r=Math.sqrt(G.random(45,i))*47,a=G.random(67,i)*6.28;dummy.position.set(Math.cos(a)*r,112,Math.sin(a)*r);dummy.scale.set(3+G.random(89,i)*4,1+G.random(12,i)*3,3+G.random(90,i)*5);dummy.rotation.set(0,a,0);dummy.updateMatrix();moss.setMatrixAt(i,dummy.matrix);}group.add(moss);
        const badge=new T.Mesh(new T.TorusGeometry(19,.8,8,48),gold);badge.position.set(0,57,58);group.add(badge);
        const emblem=new T.Mesh(leafGeometry(),gold);emblem.position.set(-7,46,59);emblem.rotation.z=-.4;emblem.scale.set(17,23,10);group.add(emblem);
        group.rotation.x=.20;group.scale.setScalar(scale);return group;
    }
    function make(canvas) {
        if(!artwork)throw Error('Prepare Leafy artwork before creating its renderer');
        const renderer=new T.WebGLRenderer({canvas,alpha:true,antialias:true,powerPreference:'low-power'});
        renderer.outputEncoding=T.sRGBEncoding;renderer.toneMapping=T.ACESFilmicToneMapping;renderer.toneMappingExposure=1.05;
        renderer.setClearColor(0,0);
        const scene=new T.Scene(),camera=new T.OrthographicCamera(0,1920,1080,0,.1,3000);camera.position.set(0,0,1500);camera.lookAt(0,0,0);
        const environment=studioEnvironment(renderer);scene.environment=environment.texture;
        scene.add(new T.HemisphereLight(0xe7efd4,0x33392c,.55));
        const key=new T.DirectionalLight(0xffefd0,1.1);key.position.set(-500,900,800);scene.add(key);
        const fill=new T.DirectionalLight(0xcde6fd,.35);fill.position.set(700,400,300);scene.add(fill);
        const texture=new T.Texture(artwork.image);texture.encoding=T.sRGBEncoding;texture.anisotropy=Math.min(4,renderer.capabilities.getMaxAnisotropy());texture.needsUpdate=true;
        const wind={value:0};let group=null,model=null,disposed=false,highlight=null,frames=0;
        function clear(){
            if(!group)return;
            group.traverse(o=>{o.geometry?.dispose();if(o.material){for(const m of Array.isArray(o.material)?o.material:[o.material]){if(m.map&&m.map!==texture)m.map.dispose();m.dispose();}}});
            scene.remove(group);group=null;highlight=null;
        }
        function mesh(g,m){const o=new T.Mesh(g,m);group.add(o);return o;}
        function update(state,width,height,anchor,light=false){
            clear();group=new T.Group();scene.add(group);model=G.layout(state,width,height,anchor);
            renderer.setPixelRatio(Math.min(devicePixelRatio||1,1.5,Math.sqrt(4000000/(width*height))));
            renderer.setSize(width,height,false);camera.right=width;camera.top=height;camera.updateProjectionMatrix();
            const stem=new T.MeshStandardMaterial({color:new T.Color(state.dead?0x675339:0x527630).convertSRGBToLinear(),roughness:.75,envMapIntensity:.12});
            const tubes=[];
            for(const b of model.branches){
                if(b.points.length<2)continue;
                const pts=b.points.map(p=>new T.Vector3(p.x,p.y,p.z)),curve=new T.CatmullRomCurve3(pts);
                const geo=new T.TubeGeometry(curve,Math.max(8,pts.length*2),Math.max(1.1,3.6-b.id*.024)*model.scale,5,false);
                // Taper the stem along its centerline.
                const positions=geo.attributes.position;
                for(let i=0;i<positions.count;i++){const f=Math.floor(i/6)/(Math.max(8,pts.length*2)),center=curve.getPointAt(Math.min(1,f)),taper=.28+.72*(1-f);positions.setXYZ(i,center.x+(positions.getX(i)-center.x)*taper,center.y+(positions.getY(i)-center.y)*taper,center.z+(positions.getZ(i)-center.z)*taper);}
                geo.computeVertexNormals();tubes.push(geo);
                // A curled growing tendril continues past the newest node.
                if(!b.capped&&!state.dead){const end=b.points.at(-1),curl=[];for(let i=0;i<12;i++){const t=i/11,a=end.angle+t*4.3,r=12*(1-t*.65)*model.scale;curl.push(new T.Vector3(end.x+(Math.cos(a)-Math.cos(end.angle))*r,end.y+(Math.sin(a)-Math.sin(end.angle))*r,end.z+2));}tubes.push(new T.TubeGeometry(new T.CatmullRomCurve3(curl),16,.75*model.scale,4,false));}
            }
            for(const leaf of model.leaves){
                const start=new T.Vector3(leaf.originX,leaf.originY,leaf.originZ),end=new T.Vector3(leaf.x,leaf.y,leaf.z);
                const middle=start.clone().lerp(end,.5);middle.z+=2*model.scale;
                tubes.push(new T.TubeGeometry(new T.QuadraticBezierCurve3(start,middle,end),3,.65*model.scale,3,false));
            }
            if(tubes.length)mesh(merge(tubes),stem);else stem.dispose();
            const wilt=state.dead?1:Math.max(0,(35-state.vitality)/35,(20-state.moisture)/45);
            const dummy=new T.Object3D();
            for(let v=0;v<8;v++){
                const list=model.leaves.filter(l=>l.variant===v);
                if(!list.length)continue;
                const material=bladeMaterial(texture);
                material.onBeforeCompile=shader=>{
                    shader.uniforms.leafyWind=wind;
                    shader.uniforms.leafyDry={value:state.dead?1:wilt*.5};
                    shader.vertexShader='uniform float leafyWind;\n'+shader.vertexShader;
                    shader.vertexShader=shader.vertexShader.replace('#include <begin_vertex>','#include <begin_vertex>\ntransformed.x += sin(position.y*3.14)*leafyWind*.015; transformed.z += sin(position.y*5.)*leafyWind*.035;');
                    shader.fragmentShader='uniform float leafyDry;\n'+shader.fragmentShader;
                    shader.fragmentShader=shader.fragmentShader.replace('#include <map_fragment>','#include <map_fragment>\nfloat dryLuma=dot(diffuseColor.rgb,vec3(.2126,.7152,.0722)); diffuseColor.rgb=mix(diffuseColor.rgb,dryLuma*vec3(1.45,.74,.30),leafyDry);');
                };
                const inst=new T.InstancedMesh(bladeGeometry(v),material,list.length);
                list.forEach((l,i)=>{dummy.position.set(l.x,l.y,l.z);dummy.rotation.set(l.tilt+wilt*.9,l.turn,l.angle+wilt*(l.tint>.5?.65:-.65));dummy.scale.set(l.size*l.aspect*(1-wilt*.25),l.size,l.size*(1+wilt*.3));dummy.updateMatrix();inst.setMatrixAt(i,dummy.matrix);inst.setColorAt(i,new T.Color().setRGB(.83+l.tint*.17,.88+l.tint*.12,.80+l.tint*.16));});
                inst.instanceMatrix.needsUpdate=true;group.add(inst);
            }
            if(model.flowers.length){
                const petalGeometry=leafGeometry(1,6,4),material=new T.MeshStandardMaterial({color:0xfcebd4,roughness:.46,side:T.DoubleSide});
                const petals=new T.InstancedMesh(petalGeometry,material,model.flowers.length*5),centers=new T.InstancedMesh(new T.SphereGeometry(1,8,6),new T.MeshStandardMaterial({color:0xdcb044,roughness:.55}),model.flowers.length);
                model.flowers.forEach((f,i)=>{
                    for(let j=0;j<5;j++){dummy.position.set(f.x,f.y,f.z);dummy.rotation.set((1-f.open)*1.3,0,j*6.28/5+f.phase);dummy.scale.set(f.size*.64,f.size*(.28+.72*f.open),f.size*.55);dummy.updateMatrix();petals.setMatrixAt(i*5+j,dummy.matrix);petals.setColorAt(i*5+j,new T.Color(f.variant?0xf7bfbe:0xfff8db));}
                    dummy.position.set(f.x,f.y,f.z+3);dummy.rotation.set(0,0,0);dummy.scale.setScalar(f.size*.18);dummy.updateMatrix();centers.setMatrixAt(i,dummy.matrix);
                });group.add(petals,centers);
            }
            const pot=potGroup(light,model.scale*1.35,state.moisture);pot.position.set(model.root.x,model.root.y-151*model.scale,55);group.add(pot);
            const c=document.createElement('canvas');c.width=256;c.height=64;const ctx=c.getContext('2d'),gradient=ctx.createRadialGradient(128,32,4,128,32,118);gradient.addColorStop(0,'rgba(0,0,0,.4)');gradient.addColorStop(1,'rgba(0,0,0,0)');ctx.fillStyle=gradient;ctx.fillRect(0,0,256,64);
            const shadow=mesh(new T.PlaneGeometry(175*model.scale,36*model.scale),new T.MeshBasicMaterial({map:new T.CanvasTexture(c),transparent:true,depthWrite:false}));shadow.position.set(model.root.x,model.root.y-155*model.scale,0);
            render();
        }
        function render(value=0){if(disposed)return;wind.value=value;renderer.render(scene,camera);frames++;}
        function select(selection){
            if(highlight){group.remove(highlight);highlight.geometry.dispose();highlight.material.dispose();highlight=null;}
            if(selection){
                const ids=G.descendants(model,selection.branch,selection.node),geos=[];
                for(const b of model.branches)if(ids.has(b.id)){const ps=b.points.slice(b.id===selection.branch?Math.max(0,selection.node-1):0);if(ps.length>1)geos.push(new T.TubeGeometry(new T.CatmullRomCurve3(ps.map(p=>new T.Vector3(p.x,p.y,105))),ps.length*2,5*model.scale,5,false));}
                if(geos.length){highlight=mesh(merge(geos),new T.MeshBasicMaterial({color:0xffbd74,transparent:true,opacity:.8,depthTest:false}));}
            }render();
        }
        function bake(){
            const old=group;old.visible=false;
            const atlas=document.createElement('canvas');atlas.width=1024;atlas.height=512;const ctx=atlas.getContext('2d');
            const cam=new T.OrthographicCamera(-128,128,220,-36,.1,3000);cam.position.z=1500;renderer.setPixelRatio(1);renderer.setSize(256,256,false);
            const items=[];
            for(let i=0;i<4;i++){const o=new T.Mesh(bladeGeometry(i),bladeMaterial(texture));o.scale.setScalar(180);o.position.y=4;o.rotation.y=(i-1.5)*.12;items.push(o);}
            for(const light of [false,true]){const pot=potGroup(light,1.6,80);items.push(pot);}
            for(const pink of [false,true]){const f=new T.Group();for(let j=0;j<5;j++){const p=new T.Mesh(leafGeometry(1,6,4),new T.MeshStandardMaterial({color:pink?0xf3b8b8:0xffeed0,side:T.DoubleSide,roughness:.5}));p.scale.setScalar(77);p.rotation.z=j*Math.PI*2/5;p.position.y=100;f.add(p);}const center=new T.Mesh(new T.SphereGeometry(12,12,8),new T.MeshStandardMaterial({color:0xe3b54a}));center.position.set(0,100,12);f.add(center);items.push(f);}
            items.forEach((o,i)=>{scene.add(o);renderer.render(scene,cam);ctx.drawImage(canvas,(i%4)*256,Math.floor(i/4)*256,256,256);scene.remove(o);o.traverse(m=>{m.geometry?.dispose();m.material?.dispose();});});
            old.visible=true;return atlas.toDataURL('image/png');
        }
        return {update,render,select,bake,get layout(){return model;},metrics:()=>({frames,calls:renderer.info.render.calls,triangles:renderer.info.render.triangles,geometries:renderer.info.memory.geometries,textures:renderer.info.memory.textures}),
            dispose(){if(disposed)return;disposed=true;clear();texture.dispose();environment.dispose();renderer.dispose();renderer.forceContextLoss();}};
    }
    window.AuraLeafyRenderer={prepare,create:make};
})();
