// A shared Three.js drawing surface feeds persistent Pixi character layers in preview, thumbnails and export.
(function(root){
    'use strict';
    var SM=root.StorymakerCore||(root.StorymakerCore={});
    function Renderer(owner){
        var T=root.StorymakerVendor.THREE;this.owner=owner;this.canvas=owner.env.makeCanvas(768,768);
        this.renderer=new T.WebGLRenderer({canvas:this.canvas,alpha:true,antialias:true,preserveDrawingBuffer:true,powerPreference:'low-power'});
        this.renderer.setSize(768,768,false);this.renderer.setClearColor(0,0);this.renderer.outputColorSpace=T.SRGBColorSpace;
        this.renderer.toneMapping=T.ACESFilmicToneMapping;this.renderer.toneMappingExposure=1.1;
        this.scene=new T.Scene();this.camera=new T.OrthographicCamera(-1.1,1.1,2.1,-.1,.1,20);this.camera.position.set(0,0,5);
        this.scene.add(new T.HemisphereLight(0xffffff,0x7b8395,2.6));
        var key=new T.DirectionalLight(0xfff3e8,3);key.position.set(-2,3,4);this.scene.add(key);
        var rim=new T.DirectionalLight(0xdbe6ff,1.6);rim.position.set(2,3,-2);this.scene.add(rim);
        this.instances=new Map();
    }
    Renderer.prototype.instance=function(key,descriptor){
        var existing=this.instances.get(key);if(existing&&existing.path===descriptor.model)return existing;
        if(existing)this.drop(key);
        var V=root.StorymakerVendor,T=V.THREE,model=V.cloneSkeleton(this.owner.modelAssets[descriptor.model]);
        var rig=SM.modelRig.create(model,this.owner.files[descriptor.motions]),group=new T.Group();group.add(model);this.scene.add(group);
        var canvas=this.owner.env.makeCanvas(768,768),texture=new V.Texture({source:new V.CanvasSource({resource:canvas,autoGarbageCollect:false})});
        var result={path:descriptor.model,group:group,model:model,rig:rig,canvas:canvas,ctx:canvas.getContext('2d'),texture:texture,sprite:new V.Sprite(texture)};
        this.instances.set(key,result);return result;
    };
    Renderer.prototype.draw=function(key,descriptor,state,presentation,time,entity){
        var instance=this.instance(key,descriptor);
        this.instances.forEach(function(v){v.group.visible=v===instance;});
        instance.group.position.set(0,0,0);instance.group.rotation.set(0,0,0);instance.group.scale.setScalar(2/1.75);
        this.scene.updateMatrixWorld(true);
        var floor=SM.modelRig.apply(instance.rig,state,time,presentation,entity);instance.group.position.y=-floor;
        instance.group.rotation.y=(presentation.view==='front'?0:presentation.view==='profile'?Math.PI/2:.5)*(state.facing<0?-1:1);
        this.renderer.render(this.scene,this.camera);
        instance.ctx.clearRect(0,0,768,768);instance.ctx.drawImage(this.canvas,0,0);instance.texture.source.update();
        return instance;
    };
    Renderer.prototype.drop=function(key){
        var v=this.instances.get(key);if(!v)return;
        v.model.traverse(function(o){if(o.skeleton)o.skeleton.dispose();});this.scene.remove(v.group);
        v.sprite.destroy();v.texture.destroy(true);v.canvas.width=v.canvas.height=1;this.instances.delete(key);
    };
    Renderer.prototype.prune=function(active){var self=this;this.instances.forEach(function(v,key){if(!active.has(key.split(':')[0]))self.drop(key);});};
    Renderer.prototype.dropModel=function(path){var self=this;this.instances.forEach(function(v,key){if(v.path===path)self.drop(key);});};
    Renderer.prototype.dispose=function(){var self=this;Array.from(this.instances.keys()).forEach(function(k){self.drop(k);});this.renderer.dispose();this.renderer.forceContextLoss();this.canvas.width=this.canvas.height=1;};
    SM.modelRender={Renderer:Renderer};
})(typeof globalThis!=='undefined'?globalThis:self);
