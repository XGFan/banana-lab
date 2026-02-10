package webui

import (
	"embed"
	"io/fs"
)

//go:embed web/dist/*
var Dist embed.FS

func FS() (fs.FS, error) {
	return fs.Sub(Dist, "web/dist")
}
