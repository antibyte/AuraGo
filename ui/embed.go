package ui

import "embed"

//go:embed index.html config.html dashboard.html mission.html missions_v2.html setup.html login.html invasion_control.html config_help.json shared.css shared.js *.png
var Content embed.FS
