import fs from 'node:fs';
import { pathToFileURL } from 'node:url';

export async function writeContract(source, destination) {
  const { tools } = await import(pathToFileURL(source + '/scripts/layerling-mcp-tools.mjs'));
  const contract = {};
  for (const tool of tools) {
    let operation = tool.name.replace('layerling_', '');
    // File bytes travel through Desktop storage, never through model arguments.
    if (operation === 'list_editors' || operation === 'import_file') continue;
    if (operation === 'read_scene') operation = 'get_scene';
    const schema = structuredClone(tool.inputSchema);
    for (const key of ['editorId', 'editorNumber', 'timeoutMs']) delete schema.properties[key];
    schema.additionalProperties = false;
    contract[operation] = { description: tool.description, schema };
  }
  const object = properties => ({ type: 'object', properties, additionalProperties: false });
  const file = { type: 'string', minLength: 1, maxLength: 512 };
  for (const operation of ['open_project', 'save_project', 'import_model']) {
    contract[operation] = { description: 'Use a Desktop workspace path. Saving a new path is create-only.', schema: { ...object({path:file}), required:['path'] } };
  }
  contract.export_model = { description:'Export to a Desktop path; existing files cause a conflict.', schema:{ ...object({path:file,format:{type:'string',enum:['stl','obj','3mf','step','png']}}), required:['path','format'] } };
  fs.mkdirSync(destination, {recursive:true});
  fs.writeFileSync(destination + '/contract.json', JSON.stringify(contract, null, 2) + '\n');
}
