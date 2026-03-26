package ui

import "embed"

//go:embed index.html config.html dashboard.html missions_v2.html setup.html login.html invasion_control.html cheatsheets.html gallery.html media.html knowledge.html containers.html truenas.html config_help.json manifest.json sw.js tailwind.min.js chart.min.js shared.css shared.js *.png *.ico cfg/*.js css/*.css js/*/*.js js/*/*/*.js lang/*.json lang/*/*.json lang/*/*/*.json
var Content embed.FS
