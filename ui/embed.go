package ui

import "embed"

//go:embed index.html config.html dashboard.html missions_v2.html setup.html login.html invasion_control.html cheatsheets.html gallery.html config_help.json manifest.json shared.css shared.js *.png cfg/*.js css/*.css js/*/*.js lang/*.json lang/*/*.json lang/*/*/*.json
var Content embed.FS
