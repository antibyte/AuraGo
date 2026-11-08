# Storymaker agent references

## Purpose

Runtime instructions and executable examples delivered to the isolated Storymaker agent by `storymaker_catalog reference`.

## Ownership

- `animation.md` describes supported storyboard controls and composition.
- `code.md` describes the restricted code-scene API and revision workflow.
- `effects.js` is the embedded, executable example shown in the code reference.
- `../studio_reference.go` embeds these sources; `../../server/storymaker_agent_profile.go` tells the agent when to read them.

## Local Contracts

- Describe implemented APIs only. PixiJS/GSAP stay internal; no raw library, DOM, filesystem or network access is granted.
- Examples must use real catalog IDs, explicit scene time and existing sandbox/deadline rules. State what is automatic and what is unsupported.
- Source writes require the revision returned by get_code. Bundle valid source before activating a code scene. Keep narration and existing unrelated scenes.
- Normal and compact profiles retain timing/easing controls; compactness must not silently remove supported animation features.

## Work Guidance

Update these references with the player API, schemas and agent profile together. Keep examples short enough for the 8 KiB agent source limit.

## Verification

- Storymaker reference/studio tests bundle the embedded example and check its runtime through the real player sources.
- Server profile and scripted-agent tests verify that the references are reachable in native and text modes.

## Child DOX Index

None.
