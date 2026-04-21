package ui

import "embed"

//go:embed index.html config.html dashboard.html plans.html missions_v2.html setup.html login.html invasion_control.html cheatsheets.html gallery.html media.html knowledge.html containers.html truenas.html skills.html config_help.json site.webmanifest sw.js tailwind.min.js chart.min.js shared.css shared-variables.css shared-utilities.css shared-components.css shared-animations.css shared.js *.png *.ico *.svg *.jpg cfg/*.js css/*.css js/*.js js/*/*.js js/*/*/*.js lang/*.json lang/*/*.json lang/*/*/*.json fonts/*
var Content embed.FS
