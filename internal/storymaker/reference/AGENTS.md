# Storymaker agent references

## Purpose

Runtime instructions and executable examples delivered to the isolated Storymaker agent by `storymaker_catalog reference`.

## Ownership

- `animation.md` describes supported storyboard controls and composition.
- `code.md` describes the restricted code-scene API and revision workflow.
- `effects.js` is the embedded, executable example shown in the code reference.
- `svg-assets.js` demonstrates a custom SVG-path motif combined with relevant catalog props; it is delivered in the same code reference.
- `../studio_reference.go` embeds these sources; `../../server/storymaker_agent_profile.go` tells the agent when to read them.

## Local Contracts

- Describe implemented APIs only. PixiJS/GSAP/Three.js stay internal; no raw library, DOM, filesystem or network access is granted.
- The animation reference names the three canonical 3D pets, old-ID compatibility and automatic facial/rig behavior.
- Examples must use real catalog IDs, explicit scene time and existing sandbox/deadline rules. State what is automatic (including scene `speaker` lip-sync) and what is unsupported, and guide spatial separation (`spot_overlap` avoidance) and cross-scene continuity.
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
