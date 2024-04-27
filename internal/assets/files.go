package assets

import (
	"embed"
	"io/fs"
)

//go:embed static
var _publicFS embed.FS

//go:embed templates
var _templateFS embed.FS

var PublicFS, _ = fs.Sub(_publicFS, "static")
var TemplateFS, _ = fs.Sub(_templateFS, "templates")
