# Storymaker agent references

## Purpose

Runtime instructions and executable examples delivered to the isolated Storymaker agent by `storymaker_catalog reference`.

## Ownership

- `animation.md` describes supported storyboard controls and composition.
- `code.md` describes the restricted code-scene API and revision workflow.
- `effects.js` is the embedded, executable example shown in the code reference.
- `svg-assets.js` demonstrates a custom SVG-path motif combined with relevant catalog props; it is delivered in the same code reference.
- `code-free.md` is the code reference of a free project (mode `free`): `code.md` without `api.add`, catalog ids and catalog search. It is the only reference a free project gets.
- `free-motion.js` is its embedded, executable example: shapes, text, tweens, a cue and `g.path` only, no catalog call. A free project's `put_code` refuses `api.add`.
- `../studio_reference.go` embeds these sources; `../../server/storymaker_agent_profile.go` tells the agent when to read them.

## Local Contracts

- Describe implemented APIs only. PixiJS/GSAP/Three.js stay internal; no raw library, DOM, filesystem or network access is granted.
- The animation reference names the three canonical 3D pets, old-ID compatibility, automatic facial/rig behavior and Phonon-only language/gender voice selection. Scene speaker and dialogue who must identify the speaking cast member.
- Examples must use real catalog IDs, explicit scene time and existing sandbox/deadline rules. State what is automatic (including scene `speaker` lip-sync) and what is unsupported, and guide spatial separation (`spot_overlap` avoidance) and cross-scene continuity.
- The free-mode references (`code-free.md`, `free-motion.js`) never mention `api.add`, `char.*`/`prop.*` ids or the catalog search; a test checks the served guide.
- Source writes require the revision returned by get_code. Bundle valid source before activating a code scene. Keep narration and existing unrelated scenes.
- Normal and compact profiles retain timing/easing controls; compactness must not silently remove supported animation features.
- Visual authoring starts with a concrete catalog search and semantic fit. Missing motifs use `g.path` SVG path data through the existing code-source workflow; no raw SVG/XML importer or file permission is implied. Keep speaking actors in storyboard scenes and custom illustration scenes as off-screen narration. Labels stay text, and decoration must explain the scene rather than fill space.

## Work Guidance

Update these references with the player API, schemas and agent profile together. Keep examples short enough for the 8 KiB agent source limit.

## Verification

- Storymaker reference/studio tests bundle the embedded example and check its runtime through the real player sources.
- Server profile and scripted-agent tests verify that the references are reachable in native and text modes.

## Child DOX Index

None.

## Choosing the picture

- `animation.md`, `code.md` and `code-free.md` each end their block part with a section `## Choosing the picture`: the decision table (process flow, loop cycle, parts hub or stack, A versus B compare, numbers chart or counter, thing icon card, key sentence headline), the variety rules (no block type three scenes in a row, 1 to 3 full boards in 7 scenes) and one worked example. The profile texts in `../../server/storymaker_agent_profile*.go` carry the same table in short form; change them together and bump the profile version. A test checks the section in all three files.
- Keep the block examples inside the compact story schema: no chart title, no compare points, no diagram flow switch. `code-free.md` stays shorter than `code.md` and never names `api.add` or catalog people and props.
