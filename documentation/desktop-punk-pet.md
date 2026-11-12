# Punk 3D desktop companion

Open the Desktop pet picker and select **Punk 3D**. It is an additional companion alongside the existing **Punk** sprite. The current selection and pet size are preserved; a size of 1.5–2.0 makes the human character easier to see.

The character uses an actual skinned 3D model, thirty locally packaged Mesh2Motion movements and blended transitions. It follows the pointer with its head and upper body, changes its facing direction when walking, and reacts to AuraGo's existing thinking, working, testing, completion and error events. Idle behavior chooses gestures, movement and rest poses.

- Click or focus the character and press Enter/Space to greet.
- Double-click to dance.
- Drag to pick it up; it hangs and sways with movement. Release to place it, or enable the existing gravity option to let it land on the dock/taskbar.
- Right-click for greeting, nodding, dancing, sitting, sleeping and standing up, plus the usual pet controls. Rest poses return to idle automatically.
- Speech bubbles continue to show the existing pet/agent messages.

The Three.js renderer, model, textures and motions load only when this companion is selected. Rendering pauses in hidden tabs; reduced-motion preferences stop continuous animation and pointer tracking. Missing resources or WebGL context loss restore the existing animated sprite fallback. No external animation service, login or paid API is needed while using the pet.

Mesh2Motion supplies the movement library. The body/hand model and rig were authored separately from AuraGo's Punk reference using NC AI and Blender. The character supports body and finger animation; it has no facial-expression/lip-sync rig or cloth/hair simulation. It does not inspect screen content or operate other windows.

Authoring and reproducible build/verification commands are in `assets/desktop-pets/AGENTS.md`; upstream licensing and pinned provenance are in `licences/mesh2motion/NOTICE.md`.

## Zombert 3D

Select **Zombert 3D** for the detailed zombie companion. It has the same controls,
thirty offline Mesh2Motion movements and sprite fallback as Punk 3D. The original
**Zombert** sprite remains separately selectable; adding the model never changes
the active pet automatically.

The model was generated from Zombert's retained ImageGen T-pose reference through
VARCO's OAuth MCP workflow, then finger-rigged and prepared in Blender. Its runtime
has 50,000 triangles, 53 connected joints and 1024-pixel packed textures. The
180,000-triangle original, VARCO finger rig and editable Blender source remain in
`assets/desktop-pets/zombert/`. `generation.json` records task ids and hashes;
neither provider credentials nor a cloud connection are needed during playback.
Body and fingers animate; facial expressions, lip-sync and cloth simulation are
not part of this rig.
