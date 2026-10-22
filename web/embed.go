package web

import "embed"

//go:embed index.html app.js style.css third-party-notices.txt
var Assets embed.FS
