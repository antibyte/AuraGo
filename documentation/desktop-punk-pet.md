# Desktop 3D companions

Open the Desktop pet picker, choose the **3D** filter and select a companion.
Twenty-seven of the 31 sprite pets have separate 3D entries. Adding models preserves the
current selection and size; 1.5–2.0 makes the human figures easier to see.

The catalog includes Punk, Zombert, Green, Hoodie, Professor, Silver, Rexi,
Purple Tentacle, Butler, Manager, Kommandantin, Vampir, Assistentin,
Terminator, Glibbi, Zorbit, Indiana Jones, Hana, Pippin, Pavlo, Buddy,
Minka and Kroko, plus OpenPets Default, Clippy, Tux and Dobby.
The original sprites remain selectable and serve as WebGL-failure fallbacks.
The MCP, Snoopy, Wall-E and Webbi 3D versions are retired; saved selections of
these models resolve to their original sprites. Existing workspace files remain intact.

Each companion uses a skinned 3D model, thirty locally packaged movements and
blended transitions. It follows the pointer, faces its walking direction and
reacts to AuraGo's thinking, working, testing, completion and error events.
Idle behavior chooses gestures, movement and rest poses.

- Click, or focus the character and press Enter/Space, to greet.
- Double-click to dance.
- Drag to pick it up. Release to place it, or enable gravity to let it land on
  the dock/taskbar.
- Right-click for greeting, nodding, dancing, sitting, sleeping and standing up,
  plus the usual pet controls. Rest poses return to idle automatically.
- Speech bubbles show the existing pet/agent messages. The foreground setting
  controls whether windows cover the companion.

The renderer, textures and motions load only for the selected model. Rendering
pauses in hidden tabs; reduced-motion preferences stop continuous animation and
pointer tracking. Loading hides the fallback sprite. Missing resources or WebGL
context loss restore it. No external service, login or paid API is needed during
playback.

## Rigs and movements

Sixteen humanoids use thirty pinned Mesh2Motion body movements each. Punk
retains its authored 71-joint rig and violet eyes; fourteen use 53 connected
VARCO body/finger joints. Vampir has 69 joints: the original body skeleton plus
two eight-segment tentacle chains. Human arm/finger weights do not deform its
surface. Smooth curve weights keep the cups and curled tips attached; original
procedural tentacle gestures layer over the body clips. The cape stays on the
body rather than following human hands.

Eleven non-humanoids use original procedural movements matched to their
anatomy. Buddy, Minka and Kroko preserve VARCO's native 29-joint quadruped skin
weights. Rexi has hind legs, forelimbs, jaw and flexible tail. Purple Tentacle
and Glibbi have flexible bases/tips. Birds have wings and feet, Pavlo has a
seven-part tail fan, and Clippy bends its wire body. These gestures share the
desktop controls but differ from human movements.

Body parts animate; facial expressions, lip-sync, cloth/hair simulation and
screen-content understanding are outside these rigs. The generated surface
can retain small source imperfections. The companions do not operate windows.

## Sources and reproducibility

Runtime models have about 50,000 triangles, at most 71 joints and packed
textures up to 1024 pixels. Original cloud outputs,
editable Blender sources, prompts,
references, rig landmarks and generation hashes live under
`assets/desktop-pets/<folder>/`; runtime exports live under `ui/3d/pets/`.
The active VARCO models use built-in ImageGen references. Retained source
folders include the four retired models and their original provenance. The
retired Wall-E source used an existing sprite and a Blender reconstruction.
`generation.json` records this distinction and the generation tasks.

Build and verification commands are in `assets/desktop-pets/AGENTS.md`.
Mesh2Motion animation artwork is CC0 and its retargeting code is MIT; see
`licences/mesh2motion/NOTICE.md`. The original creature motion code/data and Vampir's tentacle layer are
GPL-3.0-only. Those licenses and private cloud visibility do not relabel the
character designs or the original OpenPets artwork notices.
