package web

import (
	"embed"
	_ "embed"
)

//go:embed static/css/report.css
var CSS string

//go:embed static/js/report.js
var JS string

//go:embed templates/*.html
var Templates embed.FS
