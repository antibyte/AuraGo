# Goldfish Adventure Visual Development

## Purpose

Original ImageGen visual development for the planned Desktop point-and-click
adventure **Bitte nicht das Universum füttern**. The design and production gates
live in `../../plans/2026-11-06-goldfish-adventure.md`.

## Ownership

- This directory owns source concepts, approved character references and their
  generation prompts. It contains no shipped game runtime.
- Future runtime artwork belongs in `ui/img/goldfish-adventure/`; its integration
  must follow `ui/AGENTS.md` and `ui/js/desktop/apps/AGENTS.md`.

## Local Contracts

- Generate original backgrounds, characters, props, inventory art, icons and
  decorative interface artwork with ImageGen. Preserve prompts and reference
  relationships; keep readable localized text outside bitmap artwork.
- Concepts are proposals, not approved layered scenes, animation-ready rigs or
  measured runtime assets. Never claim generated stills prove animation quality.
- Use approved character references for subsequent poses. Preserve body shape,
  face, asymmetrical parts, scale, light direction and palette across outputs.
- Keep alpha on cutouts. Record actual dimensions; enlarged output is not native
  high-resolution art. Keep source masters distinct from packed runtime exports.
- Classical adventures and science-fiction comedy inform staging, humor and
  puzzle structure; characters, locations, dialogue and artwork remain original.

## Work Guidance

- Develop and review one complete playable scene before producing the full set.
- Export deterministically only after visual review: layers, occlusion masks,
  origins, poses and frame timing need explicit authoring and verification.
- Prompt records and asset metadata use English; quoted game copy uses German.

## Verification

- Inspect every delivered concept visually and verify image dimensions/alpha.
- Check local Markdown links, UTF-8 without BOM and `git diff --check`.
- Runtime, animation and full-game acceptance gates are defined in the plan;
  no runtime verification is implied by the source concepts.

## Child DOX Index

None.
