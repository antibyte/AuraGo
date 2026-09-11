import {build} from 'esbuild';
import fs from 'node:fs/promises';
const check=process.argv.includes('--check');
if(JSON.parse(await fs.readFile('node_modules/three/package.json')).version!=='0.185.1')throw Error('Pinned Three.js 0.185.1 required');
for(const [entry,name] of [['phaser','aurago-effects-2d-1.js'],['three','aurago-effects-3d-1.js']]){
    const result=await build({entryPoints:['assets/game-maker-presentation/'+entry+'.js'],bundle:true,format:'esm',target:'es2022',minify:true,legalComments:'eof',write:false,plugins:[{name:'shared-three',setup(b){
        b.onResolve({filter:/^three$/},()=>({path:'./three-0.185.1.module.min.js',external:true}));
        // Pinned upstream Water hides its target. Expose disposal of that owned resource.
        b.onLoad({filter:/[\\/]objects[\\/]Water\.js$/},async args=>{const src=await fs.readFile(args.path,'utf8'),anchor='material.uniforms[ \'mirrorSampler\' ].value = renderTarget.texture;';if(!src.includes(anchor))throw Error('Water disposal patch requires review');return {contents:src.replace(anchor,anchor+'\nthis.dispose = () => renderTarget.dispose();'),loader:'js'}});
    }}]});
    const data=Buffer.from(result.outputFiles[0].text.replace(/[ \t]+$/gm,'')),path='internal/gamemaker/runtime/'+name;
    if(check){if((await fs.readFile(path,'utf8')).replace(/\r\n/g,'\n')!==data.toString())throw Error('Rebuild '+path)}else await fs.writeFile(path,data);
    console.log(name+': '+data.length+' bytes');
}
