import { Plugin, Injector, Inject, ICommandService, IUndoRedoService, CommandType, UniverInstanceType } from '@univerjs/core';

import { SheetInterceptorService, AFTER_CELL_EDIT, EffectRefRangId, handleDefaultRangeChangeWithEffectRefCommands } from '@univerjs/sheets';
import { ISheetClipboardService } from '@univerjs/sheets-ui';
import { deserializeRangeWithSheet, serializeRange, serializeRangeWithSheet } from '@univerjs/engine-formula';

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
                for(const c of after.charts){if(c.sheet===p.subUnitId&&structure){c.range=transform(c.range);const anchor=transform(c.anchor);if(anchor!=='#REF!')c.anchor=anchor.split(':')[0];}for(const s of c.series||[])for(const key of ['name','categories','values'])if(s[key]?.includes('!'))s[key]=transform(s[key],true);}
                after.charts=after.charts.filter(c=>c.range!=='#REF!');
            }
            if(Array.isArray(after.images)){
                if(copy)after.images.push(...before.images.filter(image=>image.sheet===p.subUnitId).map(image=>({...image,id:crypto.randomUUID(),sheet:p.targetSubUnitId})));
                after.images=after.images.filter(image=>!(remove&&image.sheet===p.subUnitId));
                for(const image of after.images)if(image.sheet===p.subUnitId&&structure)image.anchor=transform(image.anchor).split(':')[0];
                after.images=after.images.filter(image=>image.anchor!=='#REF!');
            }
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
