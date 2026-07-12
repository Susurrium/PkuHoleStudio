package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist/* dist/assets/*
var embedded embed.FS

func Dist() (fs.FS, error) {
	return fs.Sub(embedded, "dist")
}
