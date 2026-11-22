# Agent handoff: VARCO 3D authoring

Verified: 2026-11-21. Audience: another coding agent working in AuraGo.
Workspace: `C:\Users\Andi\Documents\repo\AuraGo`.

Use VARCO's authenticated MCP tools for cloud authoring, then retain and prepare
the results locally. Desktop pet playback and Aura Drift use packaged assets;
they do not call VARCO at runtime. This handoff does not authorize new generation,
spending points, replacing assets or deployment beyond the user's actual task.

## 1. Connect and inspect before changing anything

- Remote MCP endpoint: `https://3d.varco.ai/api/mcp`.
- The current machine registers it as `varco-3d`; this chat exposes tools with
  the prefix `mcp__varco_3d__`. Another client may use a different prefix.
- Register that remote endpoint in the receiving agent's MCP client and use its
  OAuth sign-in flow if necessary. Earlier setup requested `workflow:edit`.
  A copied Markdown file does not transfer authentication to another machine.
- Leave tokens in the MCP client's credential store. Do not copy OAuth codes,
  bearer tokens, API keys or old callback URLs into prompts or this repository.
- The earlier NC AI REST API/key path is a separate connection. Its key is not
  the login for this MCP endpoint. Do not assume balances are interchangeable;
  inspect the account actually used by the selected connection.

Read-only starting calls, using the operation names below with your host's prefix:

1. `list_flow_sessions({})`: discover the current account's saved flows.
2. `get_node_catalog({flowId})`: current types, ports, parameters and limits.
3. `get_flow_graph({flowId})`: saved nodes, connections, outputs and statuses.
4. `get_account_and_pricing({})`: current tier, action costs and availability.
5. `get_animation_catalog({flowId})` when animations are needed.

Authentication and all five reads worked during verification. No generation was
started. A flow named **Zombert 3D** existed; discover its current ID rather than
assuming it is an empty scratch flow. Successful historical outputs can coexist
with an empty source node. Do not rerun those jobs just to recover a download.

Always pass an explicit `flowId` after discovery. Omitting it targets the most
recently opened editor flow, which can change while the user works. These tools
do not require the editor to remain open. The graph is the **saved** state;
unsaved editor changes are not represented.

The currently exposed tools do not create a flow. If no suitable flow exists,
create a dedicated flow in the VARCO editor before continuing. Add new nodes to
an agreed work area; preserve existing nodes and successful outputs. Prefer the
`flowId`/`*_flow` API consistently instead of mixing legacy workflow aliases.

## 2. Prepare the reference

Use a reviewed image of one complete subject, with clear separation between
limbs, wheels and accessories. Avoid cropping, other characters, platform/base
geometry, background clutter and strong cast shadows. For a rigged character,
use a neutral pose with visible limbs; request the generator's T-pose option.
Inspect the actual reference before upload. Only set `useAlpha=1` when the image
really has a clean transparent background.

Keep the reference and exact image prompt in the feature's authoring folder.
Use the existing ImageGen workflow when a new reference is requested. VARCO
also offers image-generation nodes, but they can consume additional points;
uploading an already reviewed reference avoids that extra generation step.

## 3. Build image-to-3D nodes

The following are tool argument examples, not a runnable SDK. Replace angle-
bracket placeholders with returned values. Intrinsics are arrays of typed
values, even for a single value. Integer flags are `0`/`1`, not booleans.

Upload the local image bytes through `upload_image`:

```json
{"data":"<base64-encoded local image bytes>"}
```

Read the bytes locally and pass them directly to the tool; do not print the
base64 payload into chat or logs. Use the returned uploaded-image URL, not a
Windows file path, as the `ImageInput` value. The uploaded URL is an input
reference, not the eventual mesh download.

Create the source with `create_node`:

```json
{
  "flowId": "<flow-id>",
  "nodeType": "ImageInput",
  "x": 0,
  "y": 0,
  "intrinsic": {"value": [{"type": "image", "url": "<uploaded-image-url>"}]}
}
```

Create a character generator with `create_node`:

```json
{
  "flowId": "<flow-id>",
  "nodeType": "Generate3D",
  "x": 650,
  "y": 0,
  "intrinsic": {
    "count": [{"type": "int", "value": 1}],
    "polygonCount": [{"type": "int", "value": 50000}],
    "textureSize": [{"type": "int", "value": 1024}],
    "generateTexture": [{"type": "int", "value": 1}],
    "tPose": [{"type": "int", "value": 1}],
    "license": [{"type": "string", "value": "PRIVATE"}]
  }
}
```

This is a compact character example, not a universal preset. Set `tPose=0` for
props/vehicles. Re-read the feature's budgets and the live catalog before
choosing polygon count, texture size, topology or PBR options. A target of
50,000 polygons does not prove the exported triangle budget. Leave unspecified
model/quality choices at the user's remembered defaults unless their task or
the feature contract requires a particular choice. Do not silently replace a
blocked private option with public publication.

Connect them with `connect_edge`:

```json
{
  "flowId": "<flow-id>",
  "fromNodeId": "<image-node-id>",
  "fromPort": "out",
  "toNodeId": "<generate-node-id>",
  "toPort": "image"
}
```

Use `set_node_input({flowId,nodeId,intrinsic})` to change intrinsics later.
Re-read `get_flow_graph` after creation: the guessed coordinates above are only
initial positions. Use `move_node` to separate overlapping node rectangles.
Sizes can remain zero until an editor has rendered them.

## 4. Run once, poll and resume

Match each node's `pointAction` from the catalog to the live pricing response.
Check `availableToTier`, `blockedReason`, counts and the total intended work.
Do not hardcode prices or infer capabilities from a tier's display name.
The user's task must authorize the billable generation being started.
`availableToTier` is not a remaining-points balance. If the task requires a
balance check, use a current account source that actually returns remaining
points; do not claim it was verified from a pricing-only response.

Start only the intended action with `run_flow`:

```json
{"flowId":"<flow-id>","scope":["<generate-node-id>"]}
```

An empty `scope` runs **all action nodes**, including existing expensive work.
Record the flow/node IDs and submitted stage before waiting. `run_flow` returns
before completion. Poll `get_flow_graph({flowId})`, initially about every
20–30 seconds, then less often for long jobs. Keep the user informed.

The observed response envelope is `{kind, graph}`, with nodes under
`graph.nodes`. Each node includes `id`, `type`, `intrinsic`, `output`, `status`,
`position` and `size`. Check the output instance as well as the node status;
retain its returned task ID for provenance and reuse.

The current catalog describes Generate3D as typically around four minutes,
occasionally over ten and rarely around thirty. Rigging/animation also take
minutes. Use the live `durationHint`; a slow running job is not a failed job.
After a timeout, transport error or expired login, read the graph again before
submitting anything. Never blindly retry `run_flow` after an uncertain response.

## 5. Rigging and optional cloud animation

After mesh review, connect `Generate3D.out` to `Rig.mesh`. For `Rig`, use
`count=[{type:"int",value:1}]` and a `riggingMode` string from the current catalog:

| Subject | Rigging mode / preparation |
| --- | --- |
| Human-like character needing fingers | `humanoid-fingers` |
| Human-like character without finger requirements | `humanoid` |
| Suitable four-legged animal | `quadruped` |
| Tentacle, blob, bird, dinosaur or mechanical prop | Review anatomy; use the existing Blender rig workflow where automatic rigs are unsuitable. |

Run only the new rig node and poll it to completion. To reuse an existing mesh,
create a `3DInput` with `value=[{type:"3d",taskId:"<completed-mesh-task-id>"}]`.
To reuse a completed rig for animation, use **`Rigged3DInput`** with the completed
`RIG`/`AI_RIG` task ID. Task IDs and node IDs are different identifiers.

For cloud animation, connect `Rig.out` (or `Rigged3DInput.out`) to `Animate.mesh`.
Set `animation` to a **key returned by `get_animation_catalog`**, `count=1`, and
`inPlace=1` when the application controls translation. Treat event tiers as
eligible for `isPlus=true` entries when the current capabilities allow them.
Do not confuse VARCO animation keys with local Mesh2Motion clip IDs.

`Animate` has **no output port**. Its animated result belongs to the node card;
download that node's result. Inspect whether the selected export actually
contains the expected skeleton and animation tracks.

For AuraGo Desktop pets, thirty local movements already exist per model. Prefer
their established motion builders when they fit the task, rather than buying
or downloading cloud animations again. Humanoids use pinned CC0 Mesh2Motion
body clips; creatures use the repository's original anatomical movements.

## 6. Download, retain and inspect

Use `get_output_downloads` on the exact successful stage:

```json
{
  "flowId": "<flow-id>",
  "nodeId": "<generate-rig-or-animate-node-id>",
  "exportType": "glb",
  "outputIndex": 0,
  "pivotToBottom": null
}
```

Re-read the live output indices instead of assuming zero for multi-output jobs.
For static floor-standing props, `pivotToBottom=true` can be useful. For rigs,
preserve the export's coordinate/skin relationship and verify ground contact
before changing pivots. Download the returned file URL promptly; request a
fresh download link if it expires instead of regenerating the mesh.

Validate the HTTP result, non-empty GLB header/version, textures, bounds, triangle
count, skin weights, joint hierarchy and animations. Preserve the untouched
cloud GLB before preparing any runtime derivative. Record the reference/prompt,
input and output SHA-256 hashes, flow/node/task IDs, chosen options, filenames
and which result was reviewed. Retain an editable `source.blend` where required.
Never commit access tokens or temporary signed download URLs in provenance.

Review front/side/rear views and actual animated poses in Blender and the target
renderer. A successful cloud status is not visual approval. Check particularly:

- UVs/materials and complete texture atlases; avoid converting textured surfaces
  into per-face approximations that produce ragged color boundaries.
- Floating fragments, unwanted reconstructed wheels and duplicate reference
  meshes; detach moving parts deliberately instead of leaving loose geometry.
- Bone weights, rooted skeletons, joints/pivots, clipping and grounded feet.
- Tentacle/cape/eye/face behavior; do not force non-human anatomy onto arm bones.

## 7. AuraGo integration routes

Read the current root `AGENTS.md` and all guides required by the target feature.
These paths describe the established pipelines; inspect their CLI arguments and
inputs before running a builder for a new character.

| Target | Owner and retained inputs | Runtime / checks |
| --- | --- | --- |
| Desktop pets | [Pet owner](../assets/desktop-pets/AGENTS.md); `assets/desktop-pets/<folder>/` retains `generation.json`, `reference.png`, original `varco-source.glb` or `varco-rigged.glb`, `source.blend`, and anatomical `rig.json` where applicable. | `ui/3d/pets/<folder>/`; catalog/allowlist and UI contracts are documented by the owner. |
| Humanoid pet preparation | `build_zombert.py -- --pet=<folder>`; Punk has its own `build_model.py`. | `node scripts/build-punk-pet-motions.mjs --pet=<folder>` |
| Quadruped / other creature preparation | `build_quadruped_pet.py` or `build_creature_pet.py`, under `assets/desktop-pets/`, with `-- --pet=<folder>`. | `node scripts/build-creature-pet-motions.mjs --pet=<folder>` |
| Pet acceptance | [Desktop companion guide](desktop-punk-pet.md) and the pet owner. | `node scripts/test-punk-pet.mjs --pet=<folder>`; shared behavior: `node scripts/test-desktop-pet-motion.mjs`; notices: `python scripts/test-pet-notices.py`. |
| Aura Drift vehicles / scenery | [Pack owner](../assets/aura-drift/AGENTS.md) and [reviewed geometry](../assets/aura-drift/reviewed/AGENTS.md). The existing reviewed pack used the NC AI path; it is a local preparation precedent, not an MCP client. | Preserve native wheel/part nodes, pivots, UVs and placement bounds. Use `reviewed/check.py`, `test-aura-drift-assets.mjs` and the owner's browser review. |
| Aura Drift pet spectators | [Mascot owner](../ui/3d/aura-drift/pets/AGENTS.md). | Reuse active Desktop GLBs and curated local motion subsets; no fresh cloud generation is needed for an unchanged pet. |

Pet runtime budgets currently cap models at 60,000 triangles, 71 joints and
1024-square textures; verify the owner before a new export. Keep retired MCP,
Snoopy, WALL-E and Webbi 3D IDs excluded. Existing sprites and user selections
must survive. PRIVATE cloud visibility does not grant rights to character
designs; retain the model and motion license notices separately.

Regenerate affected bundles/manifests, update the nearest DOX owner, run focused
checks and visually inspect the delivered model. Commit only intended paths.
Report local preparation, commit, push, packaging and deployment separately.
Version-bound web resources and their matching binary must be activated
together; changing a source file alone does not update `aurago-test`.

## Troubleshooting OAuth response-decoding errors

An error such as `failed to resolve OAuth metadata before using stored
credentials` followed by `error decoding response body` happens before tool
discovery. It does not establish that the stored token or account is invalid.
Do not upload images, start jobs, discard credentials or repeat authorization
while the metadata transport is still broken.

For this endpoint, compare both protected-resource metadata routes:

- `https://3d.varco.ai/.well-known/oauth-protected-resource/api/mcp`
- `https://3d.varco.ai/.well-known/oauth-protected-resource`

Check HTTP status, content type, transfer encoding and whether an ordinary
strict HTTP client can read and parse the entire response. HTTP 200 alone is
insufficient. A response labelled `Transfer-Encoding: chunked` whose body starts
directly with JSON instead of a chunk length is malformed at the receiving end.
On Windows, inspect the TLS certificate issuer for local HTTPS inspection.
AVG/Avast Web Shield is a documented source of this exact failure pattern;
an observed inspection certificate is evidence of interception, not by itself
a completed A/B confirmation of the cause.

If local inspection is implicated, have the owner decide whether to test a
website exception limited to `3d.varco.ai`. AVG's website exception excludes the
domain from its scans/shields; it is broader than fixing a JSON parser. Never
disable protection globally or add an exception without the owner's approval.
Repeat the same HTTP and Codex MCP discovery tests after the approved change.
Only call the connection repaired after catalog and read-only account/flow
calls succeed. Keep model-generation jobs out of this diagnostic test.

References: [reported Codex/Avast decoding failure](https://github.com/openai/codex/issues/48504)
and [AVG website exceptions](https://support.avg.com/SupportArticleView?l=en&urlname=avg-antivirus-scan-exclusions).

## Resume note for the next agent

Leave a short record with: requested subject and destination; reference path and
hash; MCP flow/node/task IDs for each completed or running stage; last observed
status; authorized remaining work; raw and prepared file paths/hashes; checks
completed; visual issues; and the exact delivery boundary. Use local generation
records as the source of truth, not old chat claims or expiring download URLs.

Connection reference: [VARCO's user-provided onboarding article](https://3d.varco.ai/resources/user-guide/ko/connect-varco-3d-to-codex-or-claude-with-one-prompt).
Its page content could not be retrieved during this handoff; operation schemas,
authentication availability, pricing access and the saved graph were checked
directly through the live MCP connection instead.
