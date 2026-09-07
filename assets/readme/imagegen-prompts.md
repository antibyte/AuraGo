# README illustration sources

Playful, geeky illustrations generated with the built-in ImageGen tool. The original [AuraGo mascot](../../ui/aurago_logo.png) is the character identity reference. These are illustrations, not UI screenshots. Screenshots and the original logo files remain unchanged.

- [Masthead](gopher-masthead.webp): the original mascot's face and body translated into the feature map's inked style, with a wider home-lab workbench composition; displayed at 600px maximum width.
- [Feature map](gopher-feature-map.webp): six capability groups connected to the gopher; cables show grouping, not exact runtime data flow.
- [Memory map](gopher-memory-map.webp): four parallel context sources; retrieved context and the current request feed the model. One corrected edit removed invented business examples and tool names.
- [System wiring](system-wiring.svg): editable, hand-authored SVG. Labels and arrows follow the agent/runtime contracts, memory, co-agent and Virtual Computers guides. Personality does not grant permissions; the vault supplies service credentials, not model context. Integration and channel scopes remain distinct.

WebP exports use quality 94 with no crop or visual changes. Inspect labels and connections at full size, then check the README at 900px and 360px in light and dark modes. Essential explanations remain in Markdown. The prior abstract workstation/creation illustrations have been replaced.

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
