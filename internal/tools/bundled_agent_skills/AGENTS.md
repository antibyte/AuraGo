# Bundled Agent Skills

## Purpose

Reviewed, offline Agent Skill packages carried by the AuraGo binary.

## Ownership

This directory owns the Modern Web Guidance adapter, generated ZIP and provenance
manifest. `../agent_skills_embedded.go` installs them through the Agent Skill
Manager; upstream licensing is recorded in `../../../licences/`.

## Local Contracts

- `modern-web-guidance/SKILL.md` adapts GoogleChrome's workflow to AuraGo's native
  file readers. The ZIP contains every guide from the pinned distribution,
  unchanged, under `references/guides/`, an index and the upstream license.
- The uncompressed ZIP is wrapped in XZ (8 MiB dictionary) so repeated guide
  content shares compression; the existing Go XZ dependency opens it as a ZIP
  filesystem. Only `modern-web-guidance.zip.xz` is embedded.
  No Node/npm, remote fetch or upstream CLI runs during
  startup or skill use. Browser support data is a release snapshot.
- First installation enables the skill. Restart and verified bundle upgrades
  preserve the saved enable switch. Foreign, modified or blocked packages are
  not overwritten or granted binary trust. Upgrade sources must match a recorded
  `system:bundled` verification hash, including after a user rescan.
- Runtime files are created only by the Agent Skill Manager. No script in this
  directory is an agent-executable helper.

## Work Guidance

Update the reviewed commit/archive digest in
`../../../scripts/build-modern-web-guidance.py`, download that exact public
archive, then run the script with `--archive <download.zip>`. Review upstream
instructions, guide content and license changes before committing the new ZIP.
Do not hand-edit generated archive contents or its provenance manifest.

## Verification

- `python scripts/build-modern-web-guidance.py --check` (offline).
- `go test ./internal/tools -run 'TestEmbeddedAgentSkill|TestModernWebGuidance'`.
- Server startup tests and `node scripts/check-web-assets.mjs`.

## Child DOX Index

None.
