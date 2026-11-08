// Measure opaque joint cross-sections; keep all retained ImageGen pixels unchanged.
import { LOWER_ARM, LOWER_LEG } from './dims.mjs';
export function jointSection(bitmap, rect, fraction) {
    const [x,y,w,h]=rect, requested=Math.floor((h-1)*fraction);
    for(let radius=0;radius<h;radius++)for(const row of radius?[requested-radius,requested+radius]:[requested]){
        if(row<0||row>=h)continue;
        let left=w,right=-1;
        for(let column=0;column<w;column++)if(bitmap.pixels[((y+row)*bitmap.width+x+column)*4+3]>=192){left=Math.min(left,column);right=column;}
        if(right>=left)return {center:(left+right+1)/2/w,width:(right-left+1)/w,fraction:(row+0.5)/h};
    }
    throw new Error('Empty sprite joint cross-section');
}

export function fitHand(image, bitmap, body, state, wrist=body.forearm*0.65) {
    const section=jointSection(bitmap,image.rect,0.08);
    const length=body.forearm*(state==='fist'||state==='hold'?1.6:2.2);
    const scale=Math.min(wrist/(section.width*image.rect[2]),length/image.rect[3]);
    image.box=[-section.center*image.rect[2]*scale,LOWER_ARM-section.fraction*image.rect[3]*scale,image.rect[2]*scale,image.rect[3]*scale];
    return image;
}

export function headAnchor(bitmap, rect) {
    const sections=[0.25,0.35,0.45].map(fraction=>jointSection(bitmap,rect,fraction));
    return {x:sections.reduce((sum,s)=>sum+s.center,0)/sections.length*rect[2],
        width:sections.reduce((sum,s)=>sum+s.width,0)/sections.length*rect[2]};
}

export function fitFaces(variants, bitmap) {
    const reference=variants.neutral,anchor=headAnchor(bitmap,reference.rect);
    const scale=reference.box[2]/reference.rect[2],x=reference.box[0]+anchor.x*scale,y=reference.box[1];
    for(const image of Object.values(variants)){
        const measured=headAnchor(bitmap,image.rect),size=anchor.width*scale/measured.width;
        // Register the skull, excluding the moving jaw; generated grid cells do not share a reliable head pivot.
        image.box=[x-measured.x*size,y,image.rect[2]*size,image.rect[3]*size];
    }
}

function fit(slot, bitmap, width, length, start, end) {
    const a=jointSection(bitmap,slot.image.rect,start),b=jointSection(bitmap,slot.image.rect,end);
    const height=length/(b.fraction-a.fraction);
    slot.box=[-a.center*width,-a.fraction*height,width,height];
    slot.shear=-(b.center-a.center)*width/length;
    return {start:a.width*width,end:b.width*width};
}

export function calibrateAttachments(parts, bitmap, hands, body, view) {
    const part=name=>parts.find(p=>p.name===name);
    const yaw=view==='front'?0:view==='profile'?Math.PI/2:38*Math.PI/180;
    const halfWidth=(front,depth)=>Math.hypot(front*Math.cos(yaw),depth*Math.sin(yaw));
    const torso=halfWidth(body.shoulderX+20,body.chestD),hips=halfWidth(body.hipHW,body.hipD);
    part('torso').box=[-torso,-body.torsoLen-35,torso*2,body.torsoLen+50];
    part('torso').torsoCap={y:-body.torsoLen,radius:torso*0.7,neck:body.neckHW};
    part('hips').box=[-hips,-42,hips*2,110];
    for(const side of ['l','r']){
        const thigh=part('upper_leg_'+side),shin=part('lower_leg_'+side);
        const knee=fit(thigh,bitmap,body.thigh*1.25,body.upperLeg,0.12,0.90).end;
        const shinWidth=knee/jointSection(bitmap,shin.image.rect,0.12).width;
        fit(shin,bitmap,shinWidth,LOWER_LEG,0.12,0.92);
        const arm=part('upper_arm_'+side),forearm=part('lower_arm_'+side);
        const upper=fit(arm,bitmap,body.arm*1.7,body.upperArm,0.20,0.87),elbow=upper.end;
        arm.shoulderCap=upper.start/2;
        const forearmWidth=elbow/jointSection(bitmap,forearm.image.rect,0.12).width;
        const wrist=Math.min(fit(forearm,bitmap,forearmWidth,LOWER_ARM,0.12,0.90).end,body.forearm*0.65);
        for(const [state,image] of Object.entries(part('hand_'+side).variants||{})){
            // A loose cuff must not enlarge a hand beyond the body's anatomical proportions.
            fitHand(image,hands,body,state,wrist);
        }
    }
}
