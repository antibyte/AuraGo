// Reproducible, scoped file outputs for AuraGo's first-party NASSCAD wrapper.
import {readFileSync} from 'node:fs';
export const nasscadEmailFrameSource = readFileSync(new URL('../ui/js/desktop/apps/nasscad-email-frame.js',import.meta.url),'utf8').replaceAll('\r\n','\n');

export function adaptNasscadEmail(name, text) {
    if (name === 'NASSCAD_V4_7_0.htm' || name === 'index.html') {
        const anchor = 'async function _nasDownload(filename, blobOrString, mimeType){';
        if (!text.includes(anchor)) throw Error('NASSCAD download adapter anchor missing');
        text = text.replace(anchor, anchor + '\n  if(window.AuraNasscadEmail?.capture(filename, blobOrString, mimeType)) return;');
        const project = "function saveProject(list){\n  showSpinner('Save Project','Preparing…');\n  requestAnimationFrame(()=>{";
        if (!text.includes(project)) throw Error('NASSCAD project adapter anchor missing');
        text = text.replace(project, project.replace('requestAnimationFrame(', '(window.AuraNasscadEmail?.active ? queueMicrotask : requestAnimationFrame)('));
        text = text.replace('</body>', `<script>${nasscadEmailFrameSource}</script>\n</body>`);
    }
    if (['NASSCAD_V4_7_0.htm','index.html','nasscad-io.js','step-export.js','ifc-export.js'].includes(name)) {
        text = text.replaceAll("if(typeof showSaveFilePicker==='function'", "if(!window.AuraNasscadEmail?.active && typeof showSaveFilePicker==='function'");
        text = text.replaceAll('if(window.showSaveFilePicker &&', 'if(!window.AuraNasscadEmail?.active && window.showSaveFilePicker &&');
    }
    return text;
}
