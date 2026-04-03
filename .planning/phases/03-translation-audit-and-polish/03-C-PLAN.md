---
phase: 03-translation-audit-and-polish
plan: C
type: execute
wave: 1
depends_on: []
files_modified:
  - ui/config.html
  - ui/css/config.css
  - ui/js/config/main.js
autonomous: true
requirements:
  - CONS-03

must_haves:
  truths:
    - "All vault-modal-* patterns replaced with standard modal-overlay + modal-card"
    - "Delete confirmation modal uses consistent modal pattern"
  artifacts:
    - path: "ui/config.html"
      provides: "Vault delete modal HTML"
    - path: "ui/css/config.css"
      provides: "Vault modal CSS (to be removed)"
    - path: "ui/js/config/main.js"
      provides: "Modal open/close logic"
  key_links:
    - from: "ui/config.html"
      to: "ui/js/config/main.js"
      via: "vaultDeleteConfirm(), vaultDeleteCancel()"
    - from: "ui/shared.js"
      to: "ui/config.html"
      via: "showModal()"
---

<objective>
Consolidate the vault-delete-overlay (vault-modal-* pattern) in config.html to use the standard modal-overlay + modal-card pattern from shared.js.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@ui/config.html
@ui/index.html
@ui/shared.js
@ui/css/config.css
@ui/js/config/main.js

<!-- Standard modal pattern from index.html: -->
<!-- <div id="modal-overlay" class="modal-overlay"> -->
<!--     <div class="modal-card"> -->
<!--         <div id="modal-title" class="modal-title"></div> -->
<!--         <div id="modal-message" class="modal-body"></div> -->
<!--         <div class="modal-actions"></div> -->
<!--     </div> -->
<!-- </div> -->

<!-- Current vault-modal pattern in config.html: -->
<!-- <div id="vault-delete-overlay" class="vault-modal-overlay is-hidden"> -->
<!--     <div class="vault-modal-card"> -->
<!--         <div class="vault-modal-icon">☠️</div> -->
<!--         <h2 id="vault-modal-title" class="vault-modal-title"></h2> -->
<!--         <div id="vault-modal-desc" class="vault-modal-desc"></div> -->
<!--         <input id="vault-confirm-input" type="text" ...> -->
<!--         <div class="vault-modal-actions">...</div> -->
<!--     </div> -->
<!-- </div> -->
</context>

<tasks>

<task type="auto">
  <name>Task 1: Read current vault-modal implementation</name>
  <files>ui/config.html, ui/js/config/main.js</files>
  <action>
Read the vault-modal implementation to understand:
1. How vaultDeleteConfirm() and vaultDeleteCancel() work
2. How vaultCheckWord() works (the "DELETE" word confirmation)
3. What translation keys are used for the modal text

Read these sections:
- ui/config.html lines 95-120 (vault-delete-overlay HTML)
- grep for vaultDelete in ui/js/config/main.js to find the JS functions

Per D-27: Consolidate to modal-overlay + modal-card pattern (Phase 2 established unified components).
Per D-28: Remove vault-modal-* variant patterns - convert to standard modal.
  </action>
  <verify>
Bash command to verify vault-modal usage:
grep -n "vault-modal\|vaultDelete\|vault-confirm" ui/js/config/main.js | head -20
  </verify>
  <done>JS functions understood - ready for refactoring</done>
</task>

<task type="auto">
  <name>Task 2: Convert vault-delete-overlay to standard modal pattern</name>
  <files>ui/config.html</files>
  <action>
Replace the vault-modal structure in config.html with the standard modal-overlay pattern.

Current structure (lines 97-114):
```html
<div id="vault-delete-overlay" class="vault-modal-overlay is-hidden">
    <div class="vault-modal-card">
        <div class="vault-modal-icon">☠️</div>
        <h2 id="vault-modal-title" class="vault-modal-title"></h2>
        <div id="vault-modal-desc" class="vault-modal-desc">
        </div>
        <input id="vault-confirm-input" type="text" class="field-input vault-confirm-input" oninput="vaultCheckWord()"
            autocomplete="off">
        <div id="vault-confirm-word" class="is-hidden"></div>
        <div class="vault-modal-actions">
            <button id="vault-cancel-btn" type="button" onclick="vaultDeleteCancel()"
                class="vault-btn vault-btn-cancel"></button>
            <button id="vault-confirm-btn" type="button" onclick="vaultDeleteConfirm()" disabled
                class="vault-btn vault-btn-confirm"></button>
        </div>
    </div>
</div>
```

Convert to:
```html
<div id="vault-delete-overlay" class="modal-overlay">
    <div class="modal-card">
        <div class="modal-icon" style="font-size:3rem;text-align:center;">☠️</div>
        <h2 id="modal-title" class="modal-title" style="color:#f87171;text-align:center;"></h2>
        <div id="modal-message" class="modal-body">
            <p id="vault-modal-desc" class="modal-desc"></p>
            <input id="vault-confirm-input" type="text" class="field-input vault-confirm-input" oninput="vaultCheckWord()"
                autocomplete="off" style="margin-top:12px;">
            <div id="vault-confirm-word" class="is-hidden"></div>
        </div>
        <div class="modal-actions">
            <button id="vault-cancel-btn" type="button" onclick="vaultDeleteCancel()"
                class="btn btn-secondary"></button>
            <button id="vault-confirm-btn" type="button" onclick="vaultDeleteConfirm()" disabled
                class="btn btn-danger"></button>
        </div>
    </div>
</div>
```

Keep the same IDs for JS compatibility: vault-confirm-input, vault-confirm-word, vault-cancel-btn, vault-confirm-btn.
  </action>
  <verify>
grep -n "vault-modal-overlay" ui/config.html
grep -n "modal-overlay" ui/config.html
  </verify>
  <done>
- vault-modal-overlay class replaced with modal-overlay
- vault-modal-card class replaced with modal-card
</done>
</task>

<task type="auto">
  <name>Task 3: Update JS to use standard modal functions</name>
  <files>ui/js/config/main.js</files>
  <action>
Update the vault delete JS functions in ui/js/config/main.js to use the standard modal pattern.

The key changes:
1. vaultDeleteShow() - should add 'active' class to vault-delete-overlay instead of removing 'is-hidden'
2. vaultDeleteCancel() - should remove 'active' class from vault-delete-overlay
3. Keep vaultCheckWord() logic the same

Read the current implementation and update:
- Use classList.add('active') and classList.remove('active') instead of classList.remove('is-hidden')
- This matches how shared.js initModals() handles modals

Per D-29: Verify all modal triggers use consistent showModal() calls from shared.js.

If the vault delete modal needs special behavior (typing "DELETE"), keep that logic but use the standard modal activation pattern.
  </action>
  <verify>
grep -n "vaultDelete\|vault-delete-overlay" ui/js/config/main.js | head -20
  </verify>
  <done>JS updated to use classList.add/remove('active') for modal</done>
</task>

<task type="auto">
  <name>Task 4: Clean up vault-modal CSS (keep only if needed for compatibility)</name>
  <files>ui/css/config.css</files>
  <action>
Review the vault-modal CSS in ui/css/config.css (around lines 980-1030).

Check if any styles are needed for the new modal-card-based implementation. The standard modal-card styles from shared.css should handle most styling.

If .vault-modal-overlay, .vault-modal-card etc. have special styling needed:
- Consider adding CSS aliases or updating to use standard modal-card

The vault-modal had special red accent styling for delete confirmation - this should be preserved via inline styles or a .modal-card-danger variant.

Per D-27: Consolidate to modal-overlay + modal-card pattern.
  </action>
  <verify>
grep -n "\.vault-modal" ui/css/config.css | head -20
  </verify>
  <done>vault-modal-* CSS either removed or replaced with modal equivalents</done>
</task>

</tasks>

<verification>
- [ ] vault-modal-overlay replaced with modal-overlay in config.html
- [ ] JS updated to use classList.add/remove('active') pattern
- [ ] vault-modal CSS cleaned up
- [ ] Delete confirmation still requires typing "DELETE" to confirm
</verification>

<success_criteria>
- vault-delete-overlay uses modal-overlay + modal-card classes
- All vault-modal-* CSS classes are no longer referenced (or are aliases)
- Delete confirmation still requires typing "DELETE" to confirm
- Modal opens/closes correctly via JS
</success_criteria>

<output>
After completion, create `.planning/phases/03-translation-audit-and-polish/03-C-SUMMARY.md`
</output>
