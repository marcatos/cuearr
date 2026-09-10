package web

import "embed"

// Content holds the embedded web UI assets.
//
//go:embed index.html app.js styles.css favicon.svg
var Content embed.FS
