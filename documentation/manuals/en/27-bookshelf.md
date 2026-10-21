# Chapter 27: Bookshelf – library and ebook reader

Bookshelf is the library and ebook reader of the Virtual Desktop. It gathers books from a folder in the Desktop
workspace, from files you upload, and from connected WebDAV, Koofr or SMB libraries, and lets you read them in the
browser. It is for books you own that are free of DRM. It does not remove DRM, never changes your original files, and
does not send book content to an AI model or a metadata service. It is not a bookstore and does not sync with e-readers.

Technical details for administrators (catalog, source adapters, the converter sandbox, build checks) are in the
[operator reference](../../bookshelf.md).

## Table of Contents

1. [What You Need](#what-you-need)
2. [First Steps](#first-steps)
3. [Adding Books](#adding-books)
4. [Reading](#reading)
5. [Notes, Bookmarks and Progress](#notes-bookmarks-and-progress)
6. [Collections and Reading Status](#collections-and-reading-status)
7. [Optional EPUB Copies](#optional-epub-copies)
8. [Good to Know](#good-to-know)
9. [Privacy and Safety](#privacy-and-safety)
10. [When Something Goes Wrong](#when-something-goes-wrong)

---

## What You Need

- **Virtual Desktop on.** Bookshelf runs inside the Virtual Desktop. Under **Config → Virtual Desktop**, the switch
  **Enable Virtual Desktop** (`virtual_desktop.enabled`) must be on. While it is off, the desktop is unavailable and
  the Bookshelf switch has no effect.
- **Bookshelf on.** Under **Config → Virtual Desktop**, the switch **Enable Bookshelf** (`virtual_desktop.bookshelf_enabled`).
  It is on by default: `config_template.yaml` sets `bookshelf_enabled: true`, and the config loader also enables it when
  the key is missing. The help text under the switch still says "off by default". That text is wrong.
- **No restart needed.** Saving the switch takes effect at once. Turning Bookshelf off cancels its running scans and
  conversions and revokes open reader links. Your catalog stays on disk.
- **Read-only mode.** With **Read-only mode** (`virtual_desktop.readonly`) on, you can still open and read books. You
  cannot add books, scan, edit details, set a reading status, add notes or bookmarks, or save your reading position.
- **Administrators only.** Bookshelf needs Desktop administrator access (the `desktop:admin` scope). The agent has no
  Bookshelf tool, so it cannot see or change your library.
- **Optional:** an ebook converter, only for EPUB copies of a few formats. See
  [Optional EPUB Copies](#optional-epub-copies).

## First Steps

1. Open **Bookshelf** from the start menu, in the **Office** group. An empty shelf says *Your shelf is waiting* and
   shows the **Add books** button.
2. Click **Add books**. Choose **Connect a library source** for a folder or a remote library, or **Upload books** for
   files from this device.
3. For a source, pick a **Source type**, enter a **Source name**, choose the folder and click **Add and scan**. For
   uploads, choose the files and click **Upload**.
4. Click a cover to see its details, then click **Read book**.

## Adding Books

Bookshelf only reads your source files. It keeps its own catalog of titles, covers, collections and reading positions.

| Source | Where the books are | What you set up |
|---|---|---|
| **Local folder** | A folder in the Desktop workspace (by default `agent_workspace/virtual_desktop`) | Choose the Desktop workspace under **Location**, then the folder. |
| **Upload** | A managed copy in Bookshelf's import folder | Pick files from this device. The originals are not touched. |
| **WebDAV** | A folder on your WebDAV integration | **Config → WebDAV**: **Enabled** and **Show in File Manager** on. |
| **Koofr** | A folder on a Koofr mount | **Config → Koofr**: **Enabled** and **Show in File Manager** on. Choose the mount under **Location**. |
| **SMB share** | A folder on an SMB share you name | **Server**, **Share**, **Username** and **Password**. No network discovery, no guest access, no SMB1. |

- **Scans start on purpose.** Adding a folder starts a scan (**Add and scan**). Later scans run when you click **Scan now**
  in **Sources**, or on the schedule under **Refresh schedule**: **Manual only** (the default), **Every hour**,
  **Every 6 hours** or **Daily**. Subfolders are included unless you untick **Include subfolders**.
- **Files are recognized by extension.** Other files in a folder are skipped without a message. A scan reports how many
  files it saw, added and skipped.
- **Missing files.** When a complete scan no longer finds a book, the book stays in the catalog, marked **Unavailable**.
  A scan that could not reach every folder never removes books.
- **Files app.** Open an EPUB, MOBI, AZW, AZW3, FB2, FB2.ZIP, CBZ or CBR file in **Files** and it opens in Bookshelf,
  unless you chose another default app for that type. For other formats, right-click the file, choose **Open with...**
  and pick **Bookshelf**. **Always open with Bookshelf** makes it the default for that file type. The file is added
  to the catalog where it is. It is not copied.
- **Uploads.** Each file is checked against the size limit before it is sent, then copied into Bookshelf's import
  folder. If an upload stops halfway, the books already sent are kept. Retry with the same selection; only the rest
  is sent.
- **Remove source** in **Sources** disconnects it. Its books stay in the catalog as **Unavailable**, with their notes
  and reading positions, until you remove each one with **Forget from Bookshelf**. No file is touched.

## Reading

Click **Read book** to open a book in the reader. The toolbar above the page offers **← Library**, **‹** and **›**
(previous and next page), **Contents**, **Page thumbnails**, **Find in book**, **Notes and bookmarks**,
**Reading settings** and **Fullscreen**. Page thumbnails appear only for fixed-layout books, PDFs and comics.

- **Turning pages.** Use the buttons, the arrow keys, or **Page Up** and **Page Down**. In right-to-left books the
  arrow keys run the other way.
- **Contents** lists the chapters or pages. Click an entry to jump to it.
- **Text books** (flowing EPUB, TXT, HTML) offer **Text size**, **Line spacing**, **Page width** (Narrow, Comfortable,
  Wide) and **Page theme** (Paper, Sepia, Dark). TXT and HTML files also offer **Text encoding** (Automatic, UTF-8,
  Windows-1252, Windows-1251, UTF-16 LE or UTF-16 BE). There is no font picker.
- **Fixed-layout books, PDFs and comics** offer **Page fit** (Fit page or Fit width), **Zoom** (Automatic, or 75 to
  250 %), **Two-page spread** and **Reading direction**.
- **Find in book** searches the text. Scanned PDFs and image-only comics have no text, so nothing is found there.
  Bookshelf does not run OCR.
- **Password-protected PDFs** ask for the password first.
- **Links** that lead outside the book ask first (**Open external link?**). Some links are refused.
- Reading settings are saved for each book in this browser.

## Notes, Bookmarks and Progress

- **Reading position** is saved as you read. The reader toolbar shows **Position saved**, **Saving position…** or
  **Position not saved**. If saving fails, choose **Retry**.
- **Bookmark:** open **Notes and bookmarks** and choose **Add bookmark**.
- **Highlight:** select some text, then choose **Highlight**. Without a selection you see *Select some text in the
  book first.* In PDFs, highlights and notes are tied to the selected area of the page.
- **Note:** choose **Add note**, type your text and click **Save**.
- Entries are listed under **Notes and bookmarks**. Click one to jump to it, or **×** to delete it.
- **Changed files.** If a book file changes on disk, Bookshelf treats it as a new edition. The saved position is dropped
  and the status returns to **Unread**, unless you set the status yourself. Notes stay, marked *From an earlier edition;
  this position may have changed.* Their jump link is switched off.
- **Two windows.** If another window saved a newer position, Bookshelf asks you to choose **Use the saved position** or
  **Keep my current position**.

## Collections and Reading Status

- **Reading status** is **Unread**, **Reading** or **Finished**. Set it in the book's details. The shelf filters by
  status and sorts by **Recently added**, **Title**, **Author** or **Reading progress**.
- **Collections:** **New collection** creates one, and **Add to collection** puts a book into it. Books and collections
  can be moved up or down with their arrow buttons.
- **Edit book details** changes the title, author, series, description and tags. The file itself is not changed.
- **Replace cover** sets your own PNG or JPEG cover. This is a catalog override; the book file stays as it is.
- **Views:** cover art, book spines or a list. The shelf shows 36 books per page. Search and sorting cover the whole
  library.
- **Forget from Bookshelf** removes a book from the catalog. The source file stays where it is.

## Optional EPUB Copies

Some formats cannot be read directly. If the server has an ebook converter, Bookshelf can make an EPUB copy of them.
Reading EPUB, PDF, MOBI and the other supported formats does not need a converter.

- **Formats:** CBR, RTF, DOCX and ODT. LIT, PDB and LRF appear in the conversion code, but the catalog does not accept
  those files in this version, so they cannot be converted yet.
- **For administrators:** install Calibre's `ebook-convert`, `bubblewrap` (`bwrap`) and `prlimit` on a Linux host or
  container, through your usual system administration. Then set **Ebook converter executable**
  (`virtual_desktop.bookshelf_converter_path`) to `/usr/bin/ebook-convert` or `/opt/calibre/ebook-convert`. An empty
  field disables conversion. Bookshelf installs and downloads nothing.
- **For you:** in a book's details, choose **Create EPUB copy**, then **Read EPUB copy** when it is ready. The copy is
  kept on the server. Your original is unchanged, but the layout may differ from the original.
- **Limits:** a conversion stops after three minutes. The input may be up to 256 MiB.
- **Sandbox:** each conversion runs in a private work folder, with a read-only copy of the input, no network, a cleared
  environment and resource limits. If the host refuses the sandbox, the conversion fails. There is no fallback that runs
  without it.
- **Windows and macOS** cannot run the sandbox. Convert the book on another computer and upload the EPUB instead.
- DRM is never removed. ACSM license files are not books.

## Good to Know

| Format | Readable in Bookshelf | Notes |
|---|---|---|
| EPUB, flowing or fixed layout | Yes | Scripts, external images and styles, embedded frames and forms are removed |
| MOBI, AZW, AZW3 (DRM-free, KF8) | Yes | Standard MOBI and KF8 files |
| FB2, FB2.ZIP | Yes | |
| CBZ (comics) | Yes | Page images: PNG, JPEG, WebP, GIF or AVIF |
| TXT, HTML, HTM | Yes | The text encoding can be corrected |
| PDF | Yes | Password prompt if needed. No OCR. |
| CBR, RTF, DOCX, ODT | As an EPUB copy only | Needs the converter, see [Optional EPUB Copies](#optional-epub-copies) |
| DjVu, Kindle KFX, ACSM | No | They can appear in the library but cannot be opened |
| LIT, PDB, LRF | No | Refused on upload and skipped by scans in this version |

- **Size per book:** the smaller of 256 MiB and **Max file size (MB)** in the Virtual Desktop settings (50 MB by
  default). Larger files are refused on upload and skipped by scans.
- **Scans:** up to 10,000 files per scan, folders up to 64 levels deep, and up to four scans at the same time.
- **Reader archives:** an EPUB, FB2.ZIP or CBZ file may hold at most 10,000 entries, 32 MiB per entry and 256 MiB
  unpacked.
- **Cover images:** PNG or JPEG, at most 8 MB in the dialog.
- **Read cache:** 1 GiB by default for prepared copies. **Keep on this AuraGo server** keeps a copy that stays readable
  if the source goes offline. The cache summary shows used space, kept copies and the limit.
- **Platforms:** only conversion is Linux-only. The library and the reader work wherever the Virtual Desktop runs.
- **Not available:** DRM removal, OCR, online metadata lookup, e-reader or store sync, a font picker, and an agent tool
  for Bookshelf.

## Privacy and Safety

- **What leaves the machine:** nothing about your books or library goes to an AI model or a metadata service. The only
  outside connections are the sources you add (WebDAV, Koofr, SMB) and the links you choose to open.
- **Where it lives:** everything is under `data/bookshelf/`:
  - `catalog.sqlite`: titles, collections, reading status, reading positions, notes, bookmarks and sources.
  - `covers/` (cover images), `imports/` (uploads), `renditions/` (EPUB copies) and `cache/` (prepared and kept copies).
  Back up `catalog.sqlite` and the managed folders. `cache/` can be deleted; it is rebuilt when needed. Before a database
  upgrade, Bookshelf writes a backup copy of the catalog next to it.
- **Reading settings** are stored per book in your browser, not on the server.
- **SMB passwords** are kept in the AuraGo Vault under `bookshelf_smb_<source-id>_password`. Removing the source deletes
  that password. Python tools cannot read keys with the `bookshelf_` prefix.
- **Your originals** are never changed, moved or deleted by Bookshelf, whether you upload, remove, disconnect or forget
  a book.
- **Reader sandbox.** A book opens in its own frame without network access. Its scripts and forms are removed. External
  links need your confirmation.
- **Access checks.** Every request for a book checks your permission again. Book responses are sent with
  `Cache-Control: no-store`. A reader link expires after five minutes if it is never used, and after 24 idle hours
  otherwise.
- **Agent.** The agent has no Bookshelf tool. It cannot list, read, add, change or remove books, notes or reading positions.

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "Bookshelf is turned off" | Switch on **Enable Virtual Desktop** and **Enable Bookshelf** under **Config → Virtual Desktop**. |
| "Could not load your library." | Check that AuraGo is running and that your sign-in is still valid, then choose **Try again**. |
| "Could not browse this location." | The folder or connection is missing or refuses access. Check the source settings. |
| "The library scan could not be completed." | Check that the source is reachable and scan again. Up to four scans may run at once, so wait for one to finish. |
| "Scan finished with some folders unavailable. The books that were found were kept." | Fix the unreachable folder or connection, then scan again. |
| "The job was canceled. Any books already found were kept." | Start the scan again. |
| "Could not upload "…". Please try again." | Try again, and check the free disk space on the server. |
| "…" exceeds the maximum file size of 50 MB. | Use a smaller file, or raise **Max file size (MB)** (the hard cap is 256 MiB). |
| "…" exceeds the allowed file size. Check the file-size limit in the Virtual Desktop settings. | Same as above. This appears when the limit is not known yet. |
| "Already uploaded: 2 of 5. Retrying sends only the remaining books." | Retry with the same selection. |
| "This book is currently unavailable." | The file or the source is offline or was removed. Reconnect the source or scan again. |
| "This book cannot be opened here." | The format cannot be read here. See the format table and [Optional EPUB Copies](#optional-epub-copies). |
| "Could not open this book." | The file may be damaged or encrypted. |
| "Conversion is not available on this server." | The server needs Linux, the converter path, `bwrap` and `prlimit`. |
| "Could not create the EPUB copy." | The conversion failed or took longer than three minutes. Try again; the original is untouched. |
| "That password did not unlock the PDF. Try again." | Enter the password again. |
| "Reading position changed elsewhere" | Another window saved a newer position. Choose **Use the saved position** or **Keep my current position**. |
| "Position not saved" | Choose **Retry**. Otherwise the latest position is lost when you leave. |
| "The source file changed. Update the source or add the current file to Bookshelf again, then try reading it." | The file changed after it was added. Scan or update the source, then read again. |
| "Add this location as a source before opening its book." | Add the folder as a source first, under **Add books**. |
| "This file is not in a connected library." | Add the folder with **Add books**, then open the book again. |
| "This link cannot be opened from Bookshelf." | Some links are refused. Do not try to open them. |
| "The cover image must be smaller than 8 MB." | Use a smaller PNG or JPEG image. |
| `unsupported`, `limit_exceeded`, `source_readonly`, `source_access_denied`, `revision_conflict` | Shown as the raw code when there is no text for it. `unsupported`: the format is not accepted. `limit_exceeded`: a size or scan limit. `source_readonly`: the source or the Desktop is read-only. `source_access_denied`: check **Show in File Manager** for WebDAV or Koofr. `revision_conflict`: the book changed elsewhere, so reload. |
| `bookshelf_archive_limit`, `bookshelf_invalid_archive`, `bookshelf_unsupported_format`, `bookshelf_reader_timeout` | Shown as they are. The book is too large inside, damaged, not a format the reader handles, or it took too long to open. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 8: Integrations](./08-integrations.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 26: Video Studio](./26-video-studio.md)
**Next Chapter:** [Chapter 28: Mailer](./28-mailer.md)
