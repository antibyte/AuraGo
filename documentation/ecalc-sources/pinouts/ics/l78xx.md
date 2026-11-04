# l78xx — L78xx fixed regulators (TO-220) / L78xx Festspannungsregler (TO-220)

Kind: authoritative maker document
Pins: 3; types: power 2, ground 1

## View
Shape `to`, package `to220`: label side towards the viewer, leads pointing down, pins left to right `1`, `2`, `3`. Basis: ST's Figure 2 is captioned "Pin connections (top view)". For TO-220 it draws the package on its side, tab with hole on the left and leads to the right, with OUTPUT at the top, GROUND in the middle, INPUT at the bottom. Turning that drawing 90° clockwise, so that the leads point down, moves the top lead to the right: left to right INPUT, GROUND, OUTPUT. I rendered both revisions' pages to images and looked at them.

## Sources
1. STMicroelectronics, L78 Positive voltage regulator ICs, DocID2143 Rev 34 (November 2016), section 2, Figure 2. Read from a copy at https://protostack.com.au/download/l7805.pdf, page 4, accessed 2026-11-04. Covers TO-220.
2. STMicroelectronics, L78xx, L78xxC Positive voltage regulators, Rev 19 (March 2008), section 2, Figure 2. Read from a copy at https://static.rapidonline.com/pdf/47-3550.pdf, page 6, accessed 2026-11-04. Covers TO-220.

Not reachable: ST's own https://www.st.com/resource/en/datasheet/l78.pdf timed out or reset from here in every attempt (curl over HTTP/2 and HTTP/1.1, WebFetch), so the newest revision was not read. A search result names Rev 38 of February 2025; that excerpt does not count as read. Both copies above are ST's own PDFs, hosted by others.

## Cross-check
Both revisions give the same lead order for TO-220. It confirms the order in the task (in, ground, out).

One difference, the tab: Rev 34 prints `GND` next to the TO-220 tab, while Rev 19 prints no label on the TO-220 tab, only on D²PAK and DPAK. I use Rev 34, the newer one; Rev 19 does not contradict it. The tab connection therefore rests on one revision.

Neither revision numbers the TO-220 leads. Nothing UNVERIFIED about names or order.

## Choices
- Numbers `1`, `2`, `3` are positions from left to right in the front view, because ST names the leads without numbers. The notes say so.
- Names follow ST's figure: `INPUT`, `GROUND`, `OUTPUT`.
- Types: `INPUT` and `OUTPUT` are `power` by the catalog schema; `GROUND` is `ground`.
- The TO-220FP has the same lead order in both revisions, so the notes mention it. Its tab is not labelled in either figure, so the page says nothing about it.
- Left out: D²PAK, DPAK and the TO-3 of Rev 19. Other makers' 78xx parts were not checked, and the page names ST's series only.
