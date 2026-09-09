// MIT: AuraGo's adapter uses only the Apache-2.0 core's public operations.
import { blankDocumentBytes } from '@docx-editor.dev/core';
import { readOoxmlPackage, collectReviewItems, revisionItemsOf } from '@docx-editor.dev/core/store';
export { createDocxEditor, blankDocumentBytes, commandForSlot, commandForSlotValue, toolbarCommandState } from '@docx-editor.dev/core';
export { defaultFonts } from '@docx-editor.dev/fonts';
export { paintSemanticLayout } from '@docx-editor.dev/core/output';
export { tableCommand } from './writer-table-ops.js';
// Inspect preserved package trees without rewriting authored fonts or language.
export function documentResources(bytes, fonts) {
    const result=readOoxmlPackage(bytes==='blank'?blankDocumentBytes():new Uint8Array(bytes));
    if(!result.ok)throw new Error(result.reason);
    const pkg=result.package, families=new Set(), unsupported=new Set();
    let language='';
    function walk(node) {
        if(node.namespaceUri==='http://schemas.openxmlformats.org/wordprocessingml/2006/main') {
            if(node.localName==='rFonts')for(const attr of node.attributes)if(['ascii','hAnsi','eastAsia','cs'].includes(attr.localName) && attr.value)families.add(attr.value);
            if(node.localName==='lang' && !language)language=node.attributes.find(x=>x.localName==='val')?.value || '';
            if(['altChunk','object','pict'].includes(node.localName))unsupported.add(node.localName);
        }
        for(const child of node.children || [])walk(child);
    }
    for(const part of pkg.parts.values())walk(part.root);
    for(const name of pkg.partBytes.keys())if(/\/word\/(charts|embeddings)\//.test(name))unsupported.add(name.split('/')[2]);
    const available=new Set([...fonts.sources.map(x=>x.request.family),...(fonts.substitutions || []).map(x=>x.from.family)].map(x=>x.toLowerCase()));
    const missing=[...families].filter(x=>!available.has(x.toLowerCase()));
    const target=fonts.sources.find(x=>x.request.weight===400 && x.request.style==='normal')?.request.family;
    const substitutions=missing.flatMap(family=>[400,700].flatMap(weight=>['normal','italic'].map(style=>({from:{family,weight,style},to:{family:target,weight,style}}))));
    return {fonts:{...fonts,language:language || fonts.language,substitutions:[...(fonts.substitutions || []),...substitutions]},language,unsupported:[...unsupported],substitutions:missing.map(x=>x+' → '+target)};
}

export const reviewModule = {
    id: 'aurago-review',
    review: {
        displayModes: ['all-markup', 'proposed', 'original'],
        collectReviewItems,
        revisionItemsOfParagraph: (part, paragraphId) => revisionItemsOf(part).filter(item =>
            item.ranges.some(range => range.start.paragraphId === paragraphId || range.end.paragraphId === paragraphId)),
    },
};
