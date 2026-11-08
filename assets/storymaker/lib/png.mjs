// Lossless deterministic PNG slicing for accepted ImageGen masters; no artwork synthesis or matting.
import { inflateSync, deflateSync, constants } from 'node:zlib';
const signature = Buffer.from([137,80,78,71,13,10,26,10]);
const table = Uint32Array.from({ length: 256 }, (_, n) => {
    for (let k = 0; k < 8; k++) n = (n & 1) ? 0xedb88320 ^ (n >>> 1) : n >>> 1;
    return n >>> 0;
});
function checksum(data) { let n = 0xffffffff; for (const b of data) n = table[(n ^ b) & 255] ^ (n >>> 8); return (n ^ 0xffffffff) >>> 0; }
function paeth(a,b,c) { const p = a+b-c, x=Math.abs(p-a),y=Math.abs(p-b),z=Math.abs(p-c); return x<=y && x<=z ? a : y<=z ? b : c; }
export function decodePNG(bytes) {
    if (!bytes.subarray(0,8).equals(signature)) throw new Error('Not a PNG master');
    const width=bytes.readUInt32BE(16), height=bytes.readUInt32BE(20), channels=bytes[25]===6 ? 4 : bytes[25]===2 ? 3 : 0;
    if (bytes[24]!==8 || !channels || bytes[28]!==0 || !width || !height || width*height>16777216) throw new Error('Master must be an 8-bit, non-interlaced RGB/RGBA PNG under 16 megapixels');
    const chunks=[];
    for(let at=8;at<bytes.length;){
        const length=bytes.readUInt32BE(at), end=at+12+length;
        if(end>bytes.length || checksum(bytes.subarray(at+4,end-4))!==bytes.readUInt32BE(end-4)) throw new Error('Corrupt PNG chunk');
        if(bytes.toString('ascii',at+4,at+8)==='IDAT') chunks.push(bytes.subarray(at+8,end-4));
        at=end;
    }
    const stride=width*channels, raw=inflateSync(Buffer.concat(chunks),{maxOutputLength:(stride+1)*height});
    if(raw.length!==(stride+1)*height)throw new Error('Invalid PNG pixel size');
    const pixels=Buffer.alloc(width*height*4), rows=Buffer.alloc(stride*height);
    for(let y=0;y<height;y++){
        const filter=raw[y*(stride+1)]; if(filter>4)throw new Error('Invalid PNG filter');
        for(let x=0;x<stride;x++){
            const i=y*stride+x,a=x>=channels?rows[i-channels]:0,b=y?rows[i-stride]:0,c=y&&x>=channels?rows[i-stride-channels]:0;
            rows[i]=(raw[y*(stride+1)+1+x]+(filter===1?a:filter===2?b:filter===3?Math.floor((a+b)/2):filter===4?paeth(a,b,c):0))&255;
        }
        for(let x=0;x<width;x++){
            const from=y*stride+x*channels,to=(y*width+x)*4;
            pixels[to]=rows[from];pixels[to+1]=rows[from+1];pixels[to+2]=rows[from+2];pixels[to+3]=channels===4?rows[from+3]:255;
        }
    }
    return {width,height,pixels};
}
function chunk(type,data){const bytes=Buffer.alloc(data.length+12);bytes.writeUInt32BE(data.length);bytes.write(type,4);data.copy(bytes,8);bytes.writeUInt32BE(checksum(bytes.subarray(4,-4)),bytes.length-4);return bytes;}
export function encodePNG({width,height,pixels}){
    if(!Number.isInteger(width)||!Number.isInteger(height)||width<1||height<1||pixels.length!==width*height*4)throw new Error('Invalid PNG dimensions');
    const header=Buffer.alloc(13);header.writeUInt32BE(width);header.writeUInt32BE(height,4);header[8]=8;header[9]=6;
    const rows=Buffer.alloc((width*4+1)*height);
    for(let y=0;y<height;y++)pixels.copy(rows,y*(width*4+1)+1,y*width*4,(y+1)*width*4);
    return Buffer.concat([signature,chunk('IHDR',header),chunk('IDAT',deflateSync(rows,{level:9,strategy:constants.Z_FIXED})),chunk('IEND',Buffer.alloc(0))]);
}
export function cropPNG(image,rect){
    const [x,y,width,height]=rect.map(Math.round);
    if(x<0||y<0||width<1||height<1||x+width>image.width||y+height>image.height)throw new Error('Crop is outside its master');
    const pixels=Buffer.alloc(width*height*4);
    for(let row=0;row<height;row++)image.pixels.copy(pixels,row*width*4,((y+row)*image.width+x)*4,((y+row)*image.width+x+width)*4);
    return {width,height,pixels};
}
export function thumbnailPNG(image, maxEdge=256){
    const scale=Math.min(1,maxEdge/Math.max(image.width,image.height));
    const width=Math.max(1,Math.round(image.width*scale)),height=Math.max(1,Math.round(image.height*scale));
    const pixels=Buffer.alloc(width*height*4);
    for(let y=0;y<height;y++)for(let x=0;x<width;x++){
        const source=((Math.min(image.height-1,Math.floor((y+0.5)/scale))*image.width)+Math.min(image.width-1,Math.floor((x+0.5)/scale)))*4;
        image.pixels.copy(pixels,(y*width+x)*4,source,source+4);
    }
    return {width,height,pixels};
}
export function cellRect(image,index,columns,rows){
    const x0=Math.floor(index%columns*image.width/columns),y0=Math.floor(Math.floor(index/columns)*image.height/rows);
    const x1=Math.floor((index%columns+1)*image.width/columns),y1=Math.floor((Math.floor(index/columns)+1)*image.height/rows);
    return [x0,y0,x1-x0,y1-y0];
}
export function cellBounds(image,index,columns,rows){
    const [x0,y0,w,h]=cellRect(image,index,columns,rows),x1=x0+w,y1=y0+h;
    let left=x1,top=y1,right=x0,bottom=y0;
    for(let y=y0;y<y1;y++)for(let x=x0;x<x1;x++)if(image.pixels[(y*image.width+x)*4+3]>=192){left=Math.min(left,x);right=Math.max(right,x);top=Math.min(top,y);bottom=Math.max(bottom,y);}
    if(right<left||bottom<top)throw new Error(`Empty master cell ${index}`);
    const pad=2;left=Math.max(x0,left-pad);top=Math.max(y0,top-pad);right=Math.min(x1-1,right+pad);bottom=Math.min(y1-1,bottom+pad);
    return [left,top,right-left+1,bottom-top+1];
}
export function cellSubjectBounds(image,index,columns,rows){
    // Generated heads can cross nominal column boundaries; include the gutter, then isolate the connected subject.
    const cell=cellRect(image,index,columns,rows),margin=Math.floor(cell[2]/4);
    const x0=Math.max(0,cell[0]-margin),y0=cell[1],w=Math.min(image.width,cell[0]+cell[2]+margin)-x0,h=cell[3];
    const seen=new Uint8Array(w*h),queue=new Int32Array(w*h);
    let best=0,bounds;
    for(let seed=0;seed<w*h;seed++){
        if(seen[seed]||image.pixels[((y0+Math.floor(seed/w))*image.width+x0+seed%w)*4+3]<32)continue;
        let read=0,end=1,left=w,top=h,right=0,bottom=0;queue[0]=seed;seen[seed]=1;
        while(read<end){
            const at=queue[read++],x=at%w,y=Math.floor(at/w);left=Math.min(left,x);right=Math.max(right,x);top=Math.min(top,y);bottom=Math.max(bottom,y);
            for(const [dx,dy]of [[-1,0],[1,0],[0,-1],[0,1]]){
                const nx=x+dx,ny=y+dy,n=ny*w+nx;
                if(nx<0||ny<0||nx>=w||ny>=h||seen[n]||image.pixels[((y0+ny)*image.width+x0+nx)*4+3]<32)continue;
                seen[n]=1;queue[end++]=n;
            }
        }
        if(end>best){best=end;bounds=[x0+left,y0+top,right-left+1,bottom-top+1];}
    }
    if(!bounds)throw new Error(`Empty master subject ${index}`);
    return bounds;
}
