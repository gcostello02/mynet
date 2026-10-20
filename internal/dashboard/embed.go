package dashboard

import (
	"embed"
	"io/fs"
)

// Dist is populated by `npm run build` in web/admin (output: internal/dashboard/dist).
//
//go:embed dist/*
var dist embed.FS

// FS returns the embedded dashboard filesystem rooted at dist/.
func FS() (fs.FS, error) {
	return fs.Sub(dist, "dist")
}
