import { Plugin, Injector, Inject, ICommandService, IUndoRedoService, CommandType, UniverInstanceType } from '@univerjs/core';

import { SheetInterceptorService, AFTER_CELL_EDIT, EffectRefRangId, handleDefaultRangeChangeWithEffectRefCommands } from '@univerjs/sheets';
import { ISheetClipboardService } from '@univerjs/sheets-ui';
import { deserializeRangeWithSheet, serializeRange, serializeRangeWithSheet } from '@univerjs/engine-formula';

// crypto.randomUUID only exists in a secure context, which a plain-HTTP LAN install is not.
function uuid() {
    const c = globalThis.crypto;
    if (c && typeof c.randomUUID === 'function') return c.randomUUID();
    const bytes = new Uint8Array(16);
    if (c && typeof c.getRandomValues === 'function') c.getRandomValues(bytes);
    else for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
    bytes[6] = (bytes[6] & 15) | 64;
    bytes[8] = (bytes[8] & 63) | 128;
    const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
    return hex.slice(0, 8) + '-' + hex.slice(8, 12) + '-' + hex.slice(12, 16) + '-' + hex.slice(16, 20) + '-' + hex.slice(20);
}
function chartRangeUsable(ref) {
    try {
        const { range } = deserializeRangeWithSheet(ref);
        return range.endRow > range.startRow && range.endColumn > range.startColumn;
    } catch (_) {
        return false;
    }
}

// Host-owned charts, images and print settings share the editor's undo stack.
class AuraSheetsActions extends Plugin {
    static pluginName = 'AURAGO_SHEETS_ACTIONS';
    static type = UniverInstanceType.UNIVER_SHEET;
    constructor(config, injector, commands, history) {
        super();
        this._injector = injector;
        this.config = config;
        this.commands = commands;
        this.history = history;
    }
    onStarting() {
        const mutation = 'aurago.mutation.sheets-state';
        this.config.bridge.clipboard = () => this._injector.get(ISheetClipboardService);
        // Univer 1.x is sequential: run after format parsers (0), before the terminal interceptor (-1).
        if(this.config.parse) this.disposeWithMe(this._injector.get(SheetInterceptorService).writeCellInterceptor.intercept(AFTER_CELL_EDIT,{priority:-0.5,handler:(cell,context,next)=>next(this.config.parse(cell,context))}));
        this.disposeWithMe(this.commands.registerCommand({id:mutation,type:CommandType.MUTATION,
            handler:(_,params)=>{this.config.set(structuredClone(params.value));return true;}}));
        this.disposeWithMe(this._injector.get(SheetInterceptorService).interceptCommand({getMutations:command=>{
            const before=structuredClone(this.config.get()),after=structuredClone(before),p=command.params||{};
            const structure=Object.values(EffectRefRangId).includes(command.id),rename=command.id==='sheet.command.set-worksheet-name',remove=command.id==='sheet.command.remove-sheet',copy=command.id==='sheet.command.copy-sheet';
            if(!structure&&!rename&&!remove&&!copy)return {undos:[],redos:[]};
            const book=this.config.book?.(),oldName=book?.getSheetBySheetId(p.subUnitId)?.getSheetName();
            const transform=(ref,qualified=false)=>{if(!ref)return ref;const parsed=deserializeRangeWithSheet(ref);if(qualified&&parsed.sheetName!==oldName)return ref;if(rename)return serializeRangeWithSheet(p.name,parsed.range);if(remove)return '#REF!';const range=handleDefaultRangeChangeWithEffectRefCommands(parsed.range,command);return range?(qualified?serializeRangeWithSheet(parsed.sheetName,range):serializeRange(range)):'#REF!';};
            if(!copy){
                after.charts=after.charts.filter(c=>!(remove&&c.sheet===p.subUnitId));
                for(const c of after.charts){
                    // Preserved charts keep their references: the server rejects every change to them.
                    if(c.readonly)continue;
                    if(c.sheet===p.subUnitId&&structure){c.range=transform(c.range);const anchor=transform(c.anchor);if(anchor!=='#REF!')c.anchor=anchor.split(':')[0];}
                    for(const s of c.series||[])for(const key of ['name','categories','values'])if(s[key]?.includes('!'))s[key]=transform(s[key],true);
                }
                // A chart whose data range collapsed or lost its sheet cannot be written back; drop it
                // inside the same undo step instead of leaving an unsaveable workbook behind.
                const broken=c=>(c.series||[]).some(s=>['name','categories','values'].some(key=>s[key]==='#REF!'));
                after.charts=after.charts.filter(c=>c.readonly||(!broken(c)&&(!c.range||chartRangeUsable(c.range))));
            }
            if(Array.isArray(after.images)){
                if(copy)after.images.push(...before.images.filter(image=>image.sheet===p.subUnitId).map(image=>({...image,id:uuid(),sheet:p.targetSubUnitId})));
                after.images=after.images.filter(image=>!(remove&&image.sheet===p.subUnitId));
                for(const image of after.images)if(image.sheet===p.subUnitId&&structure)image.anchor=transform(image.anchor).split(':')[0];
                after.images=after.images.filter(image=>image.anchor!=='#REF!');
            }
            if(remove&&after.print&&after.print.sheet===p.subUnitId)after.print={};
            if(JSON.stringify(before)===JSON.stringify(after))return {undos:[],redos:[]};
            return {undos:[{id:mutation,params:{unitId:p.unitId,value:before}}],redos:[{id:mutation,params:{unitId:p.unitId,value:after}}]};
        }}));
        this.config.bridge.commit = (value,unitId) => {
            const before = structuredClone(this.config.get()), after = structuredClone(value);
            this.commands.syncExecuteCommand(mutation,{unitId,value:after});
            this.history.pushUndoRedo({unitID:unitId,undoMutations:[{id:mutation,params:{unitId,value:before}}],redoMutations:[{id:mutation,params:{unitId,value:after}}]});
        };
    }
}
Inject(Injector)(AuraSheetsActions,undefined,1);
ICommandService(AuraSheetsActions,undefined,2);
IUndoRedoService(AuraSheetsActions,undefined,3);
export function installSheetActions(univer,config) {
    const bridge={};
    univer.registerPlugin(AuraSheetsActions,{...config,bridge});
    return bridge;
}
