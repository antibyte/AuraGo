# openscad_render

Render OpenSCAD source in the managed Virtual Desktop OpenSCAD container. Use it when the user is working in the OpenSCAD desktop app or asks for a parametric CAD model, STL, PNG preview, or related export.

## Use

- Provide complete OpenSCAD code in `source_scad`; the service writes it as `model.scad`.
- Choose export formats with `exports`, commonly `png` and `stl`.
- Use `defines` for deterministic `-D name=value` parameters instead of concatenating shell strings.
- Set `render_mode` to `preview` for faster preview-style renders or `render` for final geometry.
- Set `save_to_desktop` when the user asks to keep the outputs in the Virtual Desktop.

## Safety

The compiler container is offline and has no exposed ports. Do not use generic filesystem paths, remote user paths, or `remote.files.*` to locate OpenSCAD outputs; use the returned job files, download URLs, and saved desktop paths.
