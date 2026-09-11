# README illustration sources

Playful, geeky illustrations generated with the built-in ImageGen tool. The original [AuraGo mascot](../../ui/aurago_logo.png) is the character identity reference. These are illustrations, not UI screenshots. Screenshots and the original logo files remain unchanged.

- [Masthead](gopher-masthead.webp): the original mascot's face and body translated into the feature map's inked style, with a wider home-lab workbench composition; displayed at 600px maximum width.
- [Persona party](persona-party.webp): all ten characters from the [desktop group wallpaper](../../ui/img/wallpapers/groupshoot.jpg), redrawn in the feature map's inked style. The original wallpaper remains unchanged; heading and tagline stay in Markdown.
- [Language flags](language-flags.webp): the 16 flag choices and order from `injectLanguageSwitcher()` in `ui/js/shared/shared-core.js`. All 16 locales have matching English translation-key coverage across `ui/lang/` catalogs. Empty vertical margins were cropped with explicit user approval; flag artwork is unchanged.
- [Feature map](gopher-feature-map.webp): six capability groups connected to the gopher; cables show grouping, not exact runtime data flow.
- [Memory map](gopher-memory-map.webp): four parallel context sources; retrieved context and the current request feed the model. One corrected edit removed invented business examples and tool names.
- [Hardware requirements](gopher-hardware.webp): the gopher with a Linux mini PC and a Proxmox LXC host, plus optional upgrade parts. Hardware specifications remain in the README's Markdown table.
- [System wiring](system-wiring.svg): editable, hand-authored SVG. Labels and arrows follow the agent/runtime contracts, memory, co-agent and Virtual Computers guides. Personality does not grant permissions; the vault supplies service credentials, not model context. Integration and channel scopes remain distinct.

WebP exports use quality 94 with no visual changes; only the language strip has its empty margins cropped. Inspect labels and connections at full size, then check the README at 900px and 360px in light and dark modes. Essential explanations remain in Markdown. The prior abstract workstation/creation illustrations have been replaced.

## Language flags — generation prompt

Built-in ImageGen, with `gopher-masthead.webp` as the palette/texture reference. Final export crops the first 1983 × 793 output to `(0, 310, 1983, 460)`, producing a 1983 × 150 strip. A subsequent ImageGen framing edit still retained large blank margins and was not used.

```text
Create ONE extremely wide, very shallow horizontal FLAG STRIP for the AuraGo GitHub README. Canvas target 3200 x 320 pixels (10:1 panoramic ratio). This must be a single narrow ribbon, NOT a poster, NOT a square, NOT a multi-row grid.

Exactly SIXTEEN distinct rectangular national flags in ONE straight horizontal row, each shown once, equal visual size and uniform gaps. All flags fully visible and geometrically accurate, no clipping, no overlapping. Order left to right is strictly:
1 United Kingdom (Union Jack)
2 Germany
3 France
4 Spain
5 China
6 Japan
7 Netherlands
8 Portugal
9 Poland
10 Czechia
11 Italy
12 Sweden
13 Norway
14 Denmark
15 Greece
16 India.

These are the exact flag choices of AuraGo's language selector: English, German, French, Spanish, Chinese, Japanese, Dutch, Portuguese, Polish, Czech, Italian, Swedish, Norwegian, Danish, Greek, Hindi.

Style: tasteful tiny hand-inked flag tiles in a retro adventure-game instruction booklet, crisp flat flag colors with VERY subtle paper grain and dark ink outlines. Midnight navy background matching the reference image. Each flag must be faithful to its real design, including the UK diagonals, Spain and Portugal emblems, Chinese stars, Nordic cross colors and offsets, Greek canton/stripes and India's navy Ashoka Chakra. Keep bright recognizable colors. Do not tint the actual flags amber. No weathering that damages symbols. No waving fabric or poles.

Use the reference image ONLY for dark navy color and restrained illustrated texture. Do NOT include its characters, workbench, gadgets or words. Fill nearly the entire canvas width with the single centered row, with small outer margins and very little vertical empty space. No title, no country names, no text, no slogans, no extra symbols, no watermark. Output the wide strip itself.
```

## Persona party — generation prompt

References: `ui/img/wallpapers/groupshoot.jpg` for character identities and composition; `gopher-feature-map.webp` for style. Generated with built-in ImageGen; exported at 1676 × 939 without cropping.

```text
Use case: identity-preserve / style-transfer.
Create one landscape illustration for the AuraGo README by redrawing reference image 1, the existing desktop persona group portrait, in the EXACT visual family of reference image 2. Reference 1 is the content and character-identity source; reference 2 is style and palette ONLY. Do not add the gopher from reference 2.

Preserve ALL TEN original characters, their individual recognizable faces, hair, clothing, expressions, props, relative positions and two-row group arrangement. Back row, left to right: black-tuxedo butler with white glove; short silver-haired woman in a white turtleneck; smiling brown-haired man in a blue suit holding a clipboard; dark-haired woman with bun in a black uniform with gold trim; elderly white-bearded thinker with round glasses and book; tall black-haired mischievous vampire with a red-lined black cape; brunette secretary with bun, glasses, white blouse and notebook. Front row, left to right: wild green-haired comic character with tongue out and tan strapped jacket; magenta-haired punk woman with piercings and studded black leather jacket; smiling brown-haired friend in a blue hoodie. Exactly these ten, no duplication, no additional people, no gophers. Preserve their original clothing without making it more revealing. Everyone is a playful fictional character.

Change the rendering from smooth 3D animation to beautiful hand-inked retro adventure-game instruction booklet art: confident dark outlines, textured matte cel shading, warm ivory highlights, subtle aged-paper grain, midnight navy shadows with turquoise, muted amber and purple accents. Keep the distinctive green and magenta hair. Match reference 2's tactile illustrated world, not a glossy 3D render, not photorealism. Preserve lively expressive faces; keep each face clearly separated and readable at README width.

Landscape group portrait approximately 2200x1230, with a quiet dark navy and warm muted brick backdrop, subtle cozy light, no busy scenery. Keep all heads fully inside the canvas with generous breathing room above and beside the group. Maintain original waist-up/back row and seated/front row framing, no accidental cropped faces or hands. No text, headings, names, slogans, logos, watermarks or UI. The heading and tagline will be real Markdown outside this picture.
```

## Masthead — generation prompt

References: `ui/aurago_logo_dark.png` for identity and `gopher-feature-map.webp` for illustration style. Generated with built-in ImageGen; final export retains the complete 1677 × 938 composition.

```text
Use case: identity-preserve / style-transfer.
Create ONE wide illustrated masthead for the AuraGo GitHub README, approximately 1800 x 672, a horizontal composition. Reference image 1 is the ORIGINAL MASCOT and has absolute priority for character identity. Reference image 2 is STYLE ONLY: match its hand-inked retro adventure-game manual drawing, warm ivory accents, subtle paper grain, midnight navy background, turquoise, mint and muted amber, soft cel shading.

Keep the original gopher as unchanged as possible: exact recognizable tall rounded turquoise body silhouette, huge round white eyes with thick dark rims, same pupil spacing and gaze, small round ears, tan double-lobed muzzle, oval dark nose, TWO separate white buck teeth, tiny tan hands and feet, friendly slightly surprised expression. Front-facing full body, no clothing, no glasses, no hat, no change to facial proportions, no added arms. Translate rendering into the inked illustrated style; do not redesign the character. No glowing transparent anatomy or circuitry on its skin.

The gopher stands large and centered, occupying about 75% of image height, with its entire silhouette safely inside generous margins. Extend the scene horizontally on both sides with a restrained little home-lab workbench: left a compact server and coiled turquoise patch cable, right a small notebook with a simple node sketch and a tiny terminal. These are subordinate background accents, dimmer and smaller than the mascot. Calm dark navy textured space, a little cozy amber light, clear silhouette. Match the visual family of reference 2 without repeating its dense feature-map layout.

No text, no wordmark, no slogans, no labels, no watermarks, no neural sphere or halo. Make the CANVAS wider, not the gopher's body. The mascot is the hero, nearly identical to reference 1 in face and body, drawn in the style of reference 2.
```

## Feature map — generation prompt

```text
Create a playful geeky illustrated FEATURE MAP for the open-source project AuraGo. This is a richly detailed, beautifully drawn hacker's field guide / retro adventure-game map of REAL SOFTWARE FEATURES. NOT a corporate landing-page hero, NOT photorealistic designer objects. Landscape 2400x1500 composition. One complete cohesive poster.
REFERENCE: the attached original AuraGo logo defines the mascot identity. Preserve its friendly turquoise Go gopher, gigantic round white eyes with dark round outlines, little rounded ears, tan muzzle, small black nose and two buck teeth. Use this SAME recognizable gopher as an enthusiastic tinkerer at the center, plugging patch cables into its homemade computer. Do not reproduce the reference background, surrounding neural circle or wordmark layout. Do not copy a watermark.
STYLE: clever hand-inked technical comic, richly colored flat cel shading with subtle grain, chunky readable silhouettes, hand-made home-lab details, playful mini-scenes, deep midnight-navy background, warm ivory lettering, cyan, orange, mint and lavender accents. Think an excellent indie-game instruction booklet crossed with a hacker's annotated workbench. Detailed, characterful, witty, but cleanly organized. A few tiny rubber ducks and terminal stickers as easter eggs. Zero sales slogans.
COMPOSITION: large title "AuraGo" across top left, subtitle "One gopher. A ridiculous toolbox." below it. Six clear illustrated feature islands arranged in two rows of three around a modestly sized central gopher hub. Generous gutters and visually traceable colored patch cables, not spaghetti. These cables mean capabilities connected to the agent, not exact backend data-flow arrows. Each island must have a highly readable heading and TWO lines of exact short text. Main headings at least 70px equivalent at 2400px; detail labels at least 42px. Use clean condensed comic/sans lettering, not handwriting. Do not add other prose.
TOP LEFT:
heading "HOME LAB"
line 1 "Docker · Proxmox · TrueNAS"
line 2 "Home Assistant · SSH"
scene: tiny server rack, container blocks, a house with a lamp, terminal plug.
TOP CENTER:
heading "MEMORY"
line 1 "RAG · Knowledge Graph"
line 2 "Core Memory · Personality"
scene: a librarian's card drawers connected to a node-and-edge knowledge graph and a notebook.
TOP RIGHT:
heading "MAKE STUFF"
line 1 "Code · Websites · Games"
line 2 "Images · Music · Documents"
scene: a code editor specimen, a tiny platform game with a jumping gopher sprite, musical notes and a page.
BOTTOM LEFT:
heading "TALK & RADIO"
line 1 "Telegram · Discord · SIP"
line 2 "Speech Lab · MeshCore"
scene: a chat bubble, rotary phone with cable, a little mesh-radio device with antenna and a voice waveform.
BOTTOM CENTER:
heading "AUTOPILOT"
line 1 "Missions · Co-agents"
line 2 "Schedules · Webhooks"
scene: an alarm clock, a small mission rocket on a clipboard and two miniature helper gophers carrying task notes.
BOTTOM RIGHT:
heading "YOUR PLAYGROUND"
line 1 "Virtual Desktop · Workspaces"
line 2 "MCP · Python Skills"
scene: a retro browser desktop with tiny music-player window, a sealed VM terrarium containing a terminal, and plug-in puzzle modules.
Tiny central hub label, only if space permits: "AGENT".
All six islands must be present exactly once, clearly separated and instantly distinguishable. Every listed label must be spelled correctly. Keep title and outer labels safely inside frame. The illustration must communicate the breadth of AuraGo at a glance. Avoid random abstract geometry, fake analytics, business language, gloss-metal hardware beauty shots, generic glowing brains, humanoid robots, app logo imitations, dense illegible tiny text and decorative circuits that have no meaning.
```

## Memory map — generation prompt

```text
Create a beautiful playful technical-comic infographic explaining AuraGo's REAL MEMORY SYSTEM. This is a companion to a geeky open-source README, not a marketing illustration. Landscape about 2000 x 1500, generous spacing and large readable English text. Use the attached original mascot as identity reference: turquoise Go gopher, giant round white eyes with dark rims, little round ears, tan muzzle, black nose, two front buck teeth. Here the same gopher is a nerdy librarian selecting index cards, holding a tiny magnifying glass. Keep mascot smaller than the actual explanatory diagram.
STYLE: crisp inked indie-game manual illustration, navy paper background, warm ivory lettering, cyan/orange/mint/lavender color coding, subtle print grain, lively physical library objects. No business aesthetics, chrome showroom hardware or meaningless abstract forms. Diagram first, charm second. No watermarks.
EXACT TITLE: "How the gopher remembers"
EXACT SUBTITLE: "Different shelves. One useful context."
Lay out this precise diagram with clear separated zones and straight or carefully routed connectors, no crossing arrows:
TOP: a broad illustrated question card labeled "CURRENT REQUEST". Below it a single downward arrow leads to a center lane labeled "CONTEXT ASSEMBLY".
MIDDLE: four equally important shelves in one horizontal row, or a clean 2x2 grid if more readable. Each shelf has its exact title and one short exact explanatory line:
1. "RECENT HISTORY" / "Messages · tool results" — stack of conversation cards.
2. "CORE MEMORY" / "Facts you want to keep" — small brass-bound index-card cabinet with a star tab.
3. "SEMANTIC MEMORY" / "Documents · embeddings · RAG" — a bookshelf with a magnifying glass matching related pages.
4. "KNOWLEDGE GRAPH" / "Entities + relationships" — a corkboard with three labeled node cards "Server", "Service", "Host" and simple connecting edges, not random neurons.
All FOUR shelves must have a visible outgoing line converging on a single bottom center tray labeled "CONTEXT ASSEMBLY"; the CURRENT REQUEST top arrow must also reach this tray through a clear side lane without cutting across the shelf content. Do NOT draw sequential arrows between shelves: they are parallel sources of context, not a required ingestion pipeline.
The bottom tray includes the gopher librarian choosing just a few cards. Its exact short caption: "Recent turns + relevant memories".
One clear outgoing arrow from that tray goes to a distinct lower-right terminal labeled "MODEL REQUEST".
At the bottom left a small separate sticky note says exactly: "Recall tools can fetch more."
Ensure all title/shelf labels are readable at README desktop width: very large typography, no fine print. No extra paragraphs or invented text. The wiring must show current request and all four sources feeding context assembly, then a model request, not the model getting an unrestricted dump of every memory. Never include vault secrets, passwords, tool permissions or personality as part of the four memory shelves. This is an illustrated overview of retrieval, not a screenshot of a real interface.
```

## Memory map — correction prompt

```text
Edit this AuraGo memory infographic. Preserve the excellent hand-inked geeky library style, palette, four parallel shelf panels, their exact existing headings and short descriptions, the turquoise gopher librarian, bottom context assembly tray, the Model Request box, and all FOUR shelf-to-tray arrows. Correct ONLY the factual content and request wiring as follows:
1. Replace ALL small narrative/example writing on papers, books, drawers and cards with simple geometric horizontal ink lines or blank colored tabs. Absolutely remove every reference to billing, business, FinOps, SLIs, SLOs, postmortems, dates, host-17, k8s.get_pods, logs.search, errors or invented tool names. Do not introduce replacement tool names or other pseudo-content. Keep the three node labels "Server", "Service", "Host" in the knowledge graph.
2. Inside the top CURRENT REQUEST card replace the existing question with this exact friendly home-lab example: "Where did we put the NAS backup?" Keep only this question and the CURRENT REQUEST heading.
3. Remove the UPPER green CONTEXT ASSEMBLY sign entirely and remove the short downward arrow immediately under CURRENT REQUEST. Only the BOTTOM TRAY should be called CONTEXT ASSEMBLY.
4. Correct the current request path: it must connect DIRECTLY to the bottom CONTEXT ASSEMBLY tray, not to the KNOWLEDGE GRAPH shelf. Route the pale blue cable from the RIGHT edge of CURRENT REQUEST through generous negative space along the FAR RIGHT outer edge of the image, entirely outside the four shelf boxes, then turn left and terminate with an arrow into the RIGHT UPPER LIP of the bottom tray. Expand canvas margin on the right a little if needed to keep this cable clearly outside all shelf panels and above the arrow from tray to MODEL REQUEST. Remove the arrow pointing at the top of KNOWLEDGE GRAPH.
5. Preserve the outgoing arrow from bottom tray to MODEL REQUEST as a separate path. Keep all four memory shelves feeding the tray in parallel. The blue RECENT HISTORY arrow tip must be visible entering the tray rather than disappearing behind the mascot.
6. Preserve title "How the gopher remembers", subtitle "Different shelves. One useful context.", four shelf titles and descriptions, bottom tray label "CONTEXT ASSEMBLY" and "Recent turns + relevant memories", and sticky note "Recall tools can fetch more." with exact spelling.
No new prose, no duplicate assembly node, no watermark. This diagram is about retrieving relevant information from four parallel memory sources to build ONE model request, guided by the current user request.
```

## Hardware requirements — generation prompt

Built-in ImageGen, with `gopher-masthead.webp` for mascot identity and `gopher-feature-map.webp` for the illustration style. The complete generated composition is exported as WebP at quality 94.

```text
Use case: stylized-concept.
Asset type: ONE companion illustration for the AuraGo README's hardware requirements section.
Input images: image 1 (gopher-masthead.webp) defines the EXACT mascot identity and hand-inked rendering; image 2 (gopher-feature-map.webp) is the palette, tactile illustration style and home-lab object reference only. Create a NEW scene, do not replicate either composition.
Primary request: the same friendly turquoise AuraGo gopher setting up a tiny Linux home lab on a wooden workbench, showing a compact mini PC and a Proxmox LXC container as two hosting options.
Style/medium: beautiful hand-inked retro adventure-game instruction booklet art, bold dark ink contours, matte textured cel shading, subtle aged-paper grain. Midnight navy #071522 background, turquoise #43acb8 mascot, warm ivory #f1dfb4 highlights, restrained mint #9ebe96 and amber #c89251 details. Charming and geeky, consistent with both references.
Mascot identity: tall rounded turquoise body, huge round ivory-white eyes with thick dark rims, small circular ears, tan double-lobed muzzle, oval black nose, exactly TWO separate buck teeth, tiny tan hands and feet. No clothing or redesign.
Composition: wide landscape about 2000 x 1100. A single coherent scene with generous margins: gopher slightly left of center proudly resting one paw on a palm-sized dark metal mini PC with front USB ports and a softly lit power button. Small cream equipment label on this PC reads exactly "MINI PC". To the right, a compact open-front home-lab server contains three tidy container compartments. One highlighted compartment holds a small terminal and has one large readable cream label exactly "PROXMOX LXC". Other compartments have blank tabs. The container is visibly INSIDE its host. In the foreground, one modest GPU card, a RAM stick and an SSD lie neatly on the bench as optional upgrade parts; do not imply the GPU is installed in or fits inside the mini PC. A small Linux penguin sticker on the mini PC hints at Linux. One short coiled turquoise patch cable, warm desk lamp light, understated workshop details.
Keep main hardware silhouettes readable when reduced to README width. No dense infographic panels, no specification text or numbers, no extra labels, no title, no branding slogans, no watermark, no photorealism, no glossy 3D, no glowing neural brains. Essential hardware requirements will be written in Markdown outside the image.
```
