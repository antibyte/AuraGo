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
