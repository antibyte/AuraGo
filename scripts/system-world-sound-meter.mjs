// Loudness meter for the System World sound survey (phase 5): WAV read/write (PCM 16/24, 32-bit float), integrated
// loudness per ITU-R BS.1770-4 (K-weighting at 48 kHz, 400 ms blocks, 75 % overlap, -70 LUFS absolute and -10 LU
// relative gates), true peak by 4x oversampling (49-tap windowed sinc), sample peak and RMS. Pure Node, no dependencies.
const PRE={b:[1.53512485958697,-2.69169618940638,1.19839281085285],a:[1,-1.69065929318241,.73248077421585]};
const RLB={b:[1,-2,1],a:[1,-1.99004745483398,.99007225036621]};
export function readWav(bytes){
  const v=new DataView(bytes.buffer,bytes.byteOffset,bytes.byteLength),tag=o=>String.fromCharCode(v.getUint8(o),v.getUint8(o+1),v.getUint8(o+2),v.getUint8(o+3));
  if(tag(0)!=='RIFF'||tag(8)!=='WAVE')throw Error('not a WAV file');
  let o=12,format=0,channels=0,rate=0,bits=0,data=null;
  while(o+8<=v.byteLength){const id=tag(o),size=v.getUint32(o+4,true),body=o+8;
    if(id==='fmt '){format=v.getUint16(body,true);channels=v.getUint16(body+2,true);rate=v.getUint32(body+4,true);bits=v.getUint16(body+14,true);if(format===0xfffe)format=v.getUint16(body+24,true);}
    else if(id==='data')data={offset:body,size:Math.min(size,v.byteLength-body)};
    o=body+size+(size&1);}
  if(!data||!channels)throw Error('WAV without fmt or data');
  if(!(format===3&&bits===32)&&!(format===1&&(bits===16||bits===24)))throw Error('unsupported WAV format '+format+'/'+bits);
  const step=bits/8,frames=Math.floor(data.size/(step*channels)),out=Array.from({length:channels},()=>new Float32Array(frames));
  for(let i=0;i<frames;i++)for(let c=0;c<channels;c++){const p=data.offset+(i*channels+c)*step;
    out[c][i]=format===3?v.getFloat32(p,true):bits===16?v.getInt16(p,true)/32768:(v.getUint8(p)|v.getUint8(p+1)<<8|v.getInt8(p+2)<<16)/8388608;}
  return {sampleRate:rate,channels:out};
}
// bits 32 = IEEE float (default), bits 24 = integer PCM (clamped to full scale).
export function writeWav(channels,sampleRate,{bits=32}={}){
  if(bits!==32&&bits!==24)throw Error('writeWav: bits must be 32 (float) or 24 (PCM)');
  const ch=channels.length,len=channels[0].length,step=bits/8,bytes=44+len*ch*step,v=new DataView(new ArrayBuffer(bytes)),w=(o,s)=>{for(let i=0;i<4;i++)v.setUint8(o+i,s.charCodeAt(i));};
  w(0,'RIFF');v.setUint32(4,bytes-8,true);w(8,'WAVE');w(12,'fmt ');v.setUint32(16,16,true);v.setUint16(20,bits===32?3:1,true);v.setUint16(22,ch,true);v.setUint32(24,sampleRate,true);
  v.setUint32(28,sampleRate*ch*step,true);v.setUint16(32,ch*step,true);v.setUint16(34,bits,true);w(36,'data');v.setUint32(40,len*ch*step,true);
  let o=44;
  if(bits===32){for(let i=0;i<len;i++)for(let c=0;c<ch;c++){v.setFloat32(o,channels[c][i],true);o+=4;}}
  else for(let i=0;i<len;i++)for(let c=0;c<ch;c++){const n=Math.max(-8388608,Math.min(8388607,Math.round(channels[c][i]*8388608)));v.setUint8(o,n&255);v.setUint8(o+1,(n>>8)&255);v.setUint8(o+2,(n>>16)&255);o+=3;}
  return Buffer.from(v.buffer);
}
function biquad(x,{b,a}){const y=new Float64Array(x.length);let x1=0,x2=0,y1=0,y2=0;
  for(let i=0;i<x.length;i++){const s=x[i],o=b[0]*s+b[1]*x1+b[2]*x2-a[1]*y1-a[2]*y2;x2=x1;x1=s;y2=y1;y1=o;y[i]=o;}return y;}
export function integratedLoudness(channels,sampleRate){
  if(sampleRate!==48000)throw Error('meter: the K-weighting coefficients are for 48 kHz');
  const block=Math.round(.4*sampleRate),hop=Math.round(.1*sampleRate),len=channels[0].length;if(len<block)return -Infinity;
  const sums=channels.map(c=>{const k=biquad(biquad(c,PRE),RLB),s=new Float64Array(len+1);for(let i=0;i<len;i++)s[i+1]=s[i]+k[i]*k[i];return s;});
  const z=[];for(let s=0;s+block<=len;s+=hop){let v=0;for(const p of sums)v+=(p[s+block]-p[s])/block;z.push(v);}
  const L=v=>-.691+10*Math.log10(v),mean=a=>a.reduce((x,y)=>x+y,0)/a.length,abs=z.filter(v=>v>0&&L(v)>-70);if(!abs.length)return -Infinity;
  const rel=L(mean(abs))-10,gated=abs.filter(v=>L(v)>rel);return L(mean(gated));
}
const TAPS=49,PHASES=4,FIR=(()=>{const h=new Float64Array(TAPS),c=(TAPS-1)/2;
  for(let i=0;i<TAPS;i++){const x=(i-c)/PHASES,s=x===0?1:Math.sin(Math.PI*x)/(Math.PI*x);h[i]=s*(.5-.5*Math.cos(2*Math.PI*i/(TAPS-1)));}
  const out=[];for(let p=0;p<PHASES;p++){const taps=[];for(let i=p;i<TAPS;i+=PHASES)taps.push(h[i]);const sum=taps.reduce((a,b)=>a+b,0);out.push(Float64Array.from(taps,v=>v/sum));}return out;})();
export function truePeak(channels){let peak=0;
  for(const x of channels)for(const taps of FIR)for(let n=0;n<x.length;n++){let y=0;for(let j=0;j<taps.length&&j<=n;j++)y+=x[n-j]*taps[j];const a=Math.abs(y);if(a>peak)peak=a;}
  return peak;}
export function measure(channels,sampleRate,{skip=0}={}){
  const start=Math.min(channels[0].length,Math.round(skip*sampleRate)),cut=channels.map(c=>c.subarray(start));
  let sample=0,power=0;for(const c of cut)for(let i=0;i<c.length;i++){const a=Math.abs(c[i]);if(a>sample)sample=a;power+=c[i]*c[i];}
  const n=cut.reduce((s,c)=>s+c.length,0),dB=v=>v>0?20*Math.log10(v):-Infinity;
  return {lufs:integratedLoudness(cut,sampleRate),truePeakDb:dB(Math.max(sample,truePeak(cut))),samplePeakDb:dB(sample),rmsDb:dB(Math.sqrt(power/Math.max(1,n)))};
}
