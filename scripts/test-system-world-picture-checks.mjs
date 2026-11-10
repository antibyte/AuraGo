// System World picture checks: PNG round trip, all five decode filters and colour type 2, clipped share and blob,
// masks, percentiles, lit share, tone count, downscale and compose, budget init and check. Synthetic images only.
import assert from 'node:assert/strict';
import zlib from 'node:zlib';
import {decodePNG,encodePNG,clipped,pixelRect,maskOf,maskLumas,percentile,litShare,toneCount,downscale,compose,budgetInit,budgetCheck,luma} from './system-world-picture-checks.mjs';

const image=(w,h,fill=[0,0,0,255])=>{const data=new Uint8Array(w*h*4);for(let i=0;i<w*h;i++)data.set(fill,i*4);return {width:w,height:h,data};};
const paint=(img,x0,y0,x1,y1,c)=>{for(let y=y0;y<y1;y++)for(let x=x0;x<x1;x++)img.data.set(c,(y*img.width+x)*4);};

// 1. Round trip through the encoder.
{ const img=image(7,5,[10,20,30,255]);paint(img,2,1,5,4,[250,251,252,255]);
  const back=decodePNG(encodePNG(img));assert.equal(back.width,7);assert.equal(back.height,5);assert.deepEqual([...back.data],[...img.data]); }
// 2. Every filter type (one per row) and colour type 2 (RGB), hand-built.
{ const w=4,h=5,rgb=[];for(let y=0;y<h;y++)for(let x=0;x<w;x++)rgb.push((x*37+y*11)&255,(x*5+y*90)&255,(x*71+y*3)&255);
  const stride=w*3,raw=Buffer.alloc((stride+1)*h);
  for(let y=0;y<h;y++){const f=y;raw[y*(stride+1)]=f;for(let i=0;i<stride;i++){
    const cur=rgb[y*stride+i],a=i>=3?rgb[y*stride+i-3]:0,b=y?rgb[(y-1)*stride+i]:0,c=i>=3&&y?rgb[(y-1)*stride+i-3]:0;
    let pred=0;if(f===1)pred=a;else if(f===2)pred=b;else if(f===3)pred=(a+b)>>1;else if(f===4){const p=a+b-c,pa=Math.abs(p-a),pb=Math.abs(p-b),pc=Math.abs(p-c);pred=pa<=pb&&pa<=pc?a:pb<=pc?b:c;}
    raw[y*(stride+1)+1+i]=(cur-pred)&255;}}
  const chunk=(kind,body)=>{const head=Buffer.alloc(8);head.writeUInt32BE(body.length,0);head.write(kind,4,'latin1');return Buffer.concat([head,body,Buffer.alloc(4)]);};
  const ihdr=Buffer.alloc(13);ihdr.writeUInt32BE(w,0);ihdr.writeUInt32BE(h,4);ihdr[8]=8;ihdr[9]=2;
  const png=Buffer.concat([Buffer.from([137,80,78,71,13,10,26,10]),chunk('IHDR',ihdr),chunk('IDAT',zlib.deflateSync(raw)),chunk('IEND',Buffer.alloc(0))]);
  const img=decodePNG(png);for(let i=0;i<w*h;i++)assert.deepEqual([...img.data.subarray(i*4,i*4+4)],[rgb[i*3],rgb[i*3+1],rgb[i*3+2],255],'pixel '+i); }
// 3. Clipped share and the largest blob inside a rectangle given in fractions of the frame.
{ const img=image(100,50);paint(img,10,10,14,14,[255,255,255,255]);paint(img,60,20,61,22,[255,255,255,255]);paint(img,70,30,80,31,[255,255,200,255]);
  const all=clipped(img);assert.equal(all.pixels,18);assert.equal(all.largest,16);assert.ok(Math.abs(all.share-18/5000)<1e-12);
  const left=clipped(img,[0,0,.5,1]);assert.equal(left.pixels,16);assert.equal(left.area,2500);
  assert.deepEqual(pixelRect(img,[.1,.2,.5,1.4]),[10,10,50,50]);assert.equal(clipped(img,[.5,.5,.5,.9]).share,0); }
// 4. Masks, lumas, percentiles, lit share.
{ const mask=image(10,10);paint(mask,0,0,5,10,[255,0,0,255]);paint(mask,5,0,10,10,[0,250,10,255]);
  const red=maskOf(mask,[255,0,0]),green=maskOf(mask,[0,255,0]);assert.equal(red.reduce((a,b)=>a+b,0),50);assert.equal(green.reduce((a,b)=>a+b,0),50);
  const img=image(10,10,[20,20,20,255]);paint(img,0,0,2,10,[200,200,200,255]);
  assert.ok(Math.abs(litShare(img,red)-.4)<1e-12,'20 of 50 pane pixels lit');
  const l=maskLumas(img,red);assert.equal(l.length,50);assert.ok(Math.abs(percentile(l,.99)-luma(200,200,200))<1e-9);assert.ok(Math.abs(percentile(l,.5)-20)<1e-9);
  assert.ok(Number.isNaN(percentile([],.5))); }
// 5. Tone count: three colour fields inside the mask are three tones, a grey field one, an empty mask no blocks.
{ const w=48,h=24,img=image(w,h),mask=new Uint8Array(w*h).fill(1);
  paint(img,0,0,16,24,[200,190,170,255]);paint(img,16,0,32,24,[70,110,140,255]);paint(img,32,0,48,24,[150,80,60,255]);
  assert.equal(toneCount(img,mask).tones,3);assert.equal(toneCount(image(w,h,[120,120,120,255]),mask).tones,1);
  assert.equal(toneCount(img,new Uint8Array(w*h)).blocks,0); }
// 6. Downscale (box filter) and compose (tiles on a dark ground with a gap).
{ const img=image(4,2);paint(img,0,0,2,2,[100,0,0,255]);paint(img,2,0,4,2,[0,200,0,255]);
  const half=downscale(img,2);assert.equal(half.width,2);assert.equal(half.height,1);assert.deepEqual([...half.data],[100,0,0,255,0,200,0,255]);
  const sheet=compose([half,half,half],2,1);assert.equal(sheet.width,7);assert.equal(sheet.height,5);
  assert.deepEqual([...sheet.data.subarray((1*sheet.width+1)*4,(1*sheet.width+1)*4+4)],[100,0,0,255],'first tile at the gap offset'); }
// 7. Budget: the base is the larger median of two runs; base +10 % passes, more fails; the view-triangle limit is strict.
{ const run=(tier,ms,tri)=>({info:{tier},shots:[{scene:'street-agent',frameMs:{median:ms,p95:ms*1.2},stats:{viewTriangles:tri,triangles:tri*2}}]});
  const budget=budgetInit([run('high',10,700000),run('high',11,700000),run('low',5,300000)]);
  assert.equal(budget.frameMs['street-agent'].high,11);assert.equal(budget.frameMs['street-agent'].low,5);assert.equal(budget.margin,.1);
  assert.deepEqual(budgetCheck(budget,[run('high',12.1,640000)]).failures,[]);
  assert.equal(budgetCheck(budget,[run('high',12.2,640000)]).failures.length,1);
  assert.equal(budgetCheck(budget,[run('high',11,650000)]).failures.length,1,'650 000 is not below the limit');
  assert.equal(budgetCheck(budget,[run('low',5,999999)]).failures.length,0,'the triangle limit applies to its own scene and tier'); }
console.log('System World picture checks: PNG codec (5 filters, RGB and RGBA), clipped share and blobs, masks, lit share, tones, sheets and budget passed.');
