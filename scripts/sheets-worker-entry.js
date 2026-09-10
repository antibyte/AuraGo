import { createUniver } from '@univerjs/presets';
import { Plugin, Injector, Inject, UniverInstanceType, ObjectMatrix } from '@univerjs/core';
import { UniverSheetsCoreWorkerPreset } from '@univerjs/preset-sheets-core/worker';
import { functionStatistical, ICalculateFormulaService, IFormulaRuntimeService, FormulaExecuteStageType } from '@univerjs/engine-formula';

// Univer defaults to one iteration for circular formulas. Tabellen instead
// reports reference errors, using the engine's dependency graph and results.
class CircularReferenceErrors extends Plugin {
    static pluginName='AURAGO_CIRCULAR_REFERENCE_ERRORS';
    static type=UniverInstanceType.UNIVER_UNKNOWN;
    constructor(config,injector){super();this.injector=injector;}
    onStarting(){
        const runtime=this.injector.get(IFormulaRuntimeService);
        this.disposeWithMe(this.injector.get(ICalculateFormulaService).executionInProgressListener$.subscribe(state=>{
            if(state.stage!==FormulaExecuteStageType.CALCULATION_COMPLETED)return;
            if(!runtime.isCycleDependency())return;
            const result=runtime.getAllRuntimeData(),trees=result.dependencyTreeModelData||[],nodes=new Map(trees.map(tree=>[tree.treeId,tree]));
            const remaining=new Map(trees.map(tree=>[tree.treeId,tree.children.filter(id=>nodes.has(id)).length]));
            const ready=trees.filter(tree=>!remaining.get(tree.treeId));
            for(let i=0;i<ready.length;i++)for(const id of ready[i].parents)if(remaining.has(id)){
                const count=remaining.get(id)-1;remaining.set(id,count);if(count===0)ready.push(nodes.get(id));
            }
            for(const tree of trees)if(remaining.get(tree.treeId)>0&&tree.type===0){
                const sheet=(result.unitData[tree.unitId]??={})[tree.subUnitId]??=new ObjectMatrix();
                sheet.setValue(tree.row,tree.column,{v:'#REF!',t:1});
            }
        }));
    }
}
Inject(Injector)(CircularReferenceErrors,undefined,1);
const averageAlias=[functionStatistical.find(entry=>entry[1]==='AVERAGE')[0],'AVG'];
const {univer}=createUniver({presets:[UniverSheetsCoreWorkerPreset({formula:{function:[averageAlias]}})]});
univer.registerPlugin(CircularReferenceErrors);
