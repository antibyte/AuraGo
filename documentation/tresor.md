# Desktop Tresor

The built-in **Tresor** app keeps private notes and files in one vault per AuraGo installation. It does not share or synchronize them. A copy imported from the virtual Desktop leaves the original file in place. A deliberate export creates an ordinary, unencrypted copy outside the Tresor.

## Setup and recovery

Use a unique vault password of at least 16 characters. Setup creates a random 256-bit content key in the browser. A separately derived Argon2id key wraps it for password unlock; a random 256-bit recovery key wraps it again. The recovery key is shown once and must be entered again before the vault is stored. Keep it offline and separate from the installation backup. If both password and recovery key are lost, there is no reset path for the encrypted contents.

Changing the password replaces the password envelope and salt. It keeps the recovery key valid. Recovery unlock is available on the locked screen, so a lost password can be replaced after unlocking with the recovery key.

Notes save automatically after a short editing pause. Desktop window close waits for outstanding saves and reports failures. Explicit lock, the inactivity timer and page unload always clear the unlocked state immediately, even when a save fails or is still pending. Completed recovery drafts remain encrypted with the vault content key in local browser storage; unfinished draft encryption cannot delay locking. Drafts contain no plaintext, are removed after a successful save, and can be recovered after unlocking the same vault again. Dialogs preserve the draft title and text. A failed note load keeps the previous editor and its record association intact.

## Protection boundary

The browser uses Argon2id with 64 MiB memory, three passes, four lanes, a fresh 32-byte salt and a 32-byte output. The pinned `argon2id@1.0.1` library and its SIMD/non-SIMD WebAssembly are copied into the verified local UI resource set. Web Crypto AES-256-GCM encrypts the content key envelopes, each note, each file and each entry's metadata, including names, with a fresh 96-bit nonce and context-bound additional authenticated data. The server handles only ciphertext and key envelopes; it never receives the vault password, recovery key or decrypted contents.

The browser holds the unwrapped content key only while the app is unlocked. It locks on window close, page unload, explicit lock and after five minutes without interaction in the Tresor. Unlock and password change reload the current server header before using an envelope; failed unlock clears the candidate key and plaintext. Locking invalidates pending operations so delayed reads cannot restore decrypted state. The crypto wrapper wipes the complete Argon2 WASM workspace after each derivation. JavaScript strings and browser-managed copies cannot be securely erased; clearing references and byte arrays is best effort. Password and plaintext are not persisted in browser storage or the ordinary Desktop workspace. Normal agent file tools protect `data/tresor.db` and its WAL/SHM files through `SQLiteProtectedPaths`; unsafe host shell execution can bypass native file-tool guards and must remain disabled or isolated by policy.

Only an authenticated browser admin session may use `/api/desktop/tresor`; Desktop bearer tokens and auth-disabled installations are rejected. Writes require a matching Origin, and record changes require the current revision. The API sends `Cache-Control: no-store`. Remote access requires HTTPS; direct localhost access is allowed. Files are limited to 50 MiB of plaintext. Import and decryption happen in the browser.

Search filters decrypted entry titles locally while the vault is open.

The random 256-bit AES key is intended to remain resistant to known quantum attacks against stored data. ML-KEM addresses key exchange between parties and is not used for this local vault. No software can guarantee absolute security: a compromised browser, modified delivered scripts, malware or a weak password can expose contents while the vault is in use. A server administrator can replace browser resources, and an attacker with the database can attempt offline password guessing against the password envelope. See [RFC 9106](https://www.rfc-editor.org/rfc/rfc9106.html), [NIST's PQC FAQ](https://csrc.nist.gov/projects/post-quantum-cryptography/faqs) and [FIPS 203](https://csrc.nist.gov/pubs/fips/203/final).

## Backup and restore

The dedicated `data/tresor.db` is included in AuraGo's consistent SQLite snapshots and normal backup archive. Its records and names remain encrypted in the database and backup, but a backup can still contain other sensitive AuraGo files, so protect the entire archive. Restore the archive with the normal AuraGo backup importer; SQLite restore requires a service restart. Retain either the vault password or the recovery key independently of the archive, then unlock the restored Tresor. A raw copy of a live SQLite database without its WAL is not a valid backup.

## Checks

Run `npm run check:tresor-vendor`, `npm run test:tresor-crypto`, `npm run check:ui`, focused Go tests in `internal/tresor`, `internal/config` and `internal/server`, and `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run 'TestTresorTranslations|TestDesktopTresorBrowser'`. The browser fixture checks both Desktop styles at wide and narrow sizes, keyboard unlock, reduced motion, device and Desktop imports, encrypted autosave drafts, search and the inactivity lock. The focused Go tests cover browser-session boundaries, Origin, size and revision checks, backup inclusion and database restoration.
