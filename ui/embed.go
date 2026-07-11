package ui

import "embed"

//go:embed index.html 404.html config.html dashboard.html desktop.html plans.html missions_v2.html setup.html login.html invasion_control.html cheatsheets.html gallery.html media.html knowledge.html containers.html truenas.html skills.html config_help.json site.webmanifest sw.js tailwind.min.js chart.min.js shared.css shared-variables.css shared-utilities.css shared-components.css shared-animations.css *.png *.ico *.svg *.jpg 3d/* cfg/*.js css/*.css js/*.js js/*/*.js js/*/*/*.js js/*/*/*/*.js js/*/*/*.json js/*/*/*.txt js/*/*/*.mjs js/*/*/*.wasm lang/*.json lang/*/*.json lang/*/*/*.json fonts/* img/* img/*/*
var Content embed.FS
