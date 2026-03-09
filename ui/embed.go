package ui

import "embed"

//go:embed index.html config.html dashboard.html mission.html missions_v2.html setup.html login.html invasion_control.html config_help.json shared.css shared.js *.png cfg/*.js css/*.css js/*.js js/*/*.js lang/*.json lang/*/*.json lang/*/*/*.json
var Content embed.FS
