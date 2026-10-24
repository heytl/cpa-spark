package web

import "embed"

//go:embed index.html app.js style.css third-party-notices.txt logo.svg sidebar-icon.svg
var Assets embed.FS
