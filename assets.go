package ohmymc

import (
	"embed"
	"io/fs"
)

// Assets are embedded for both MyGO's secure frontend origin and local browser development.
//
//go:embed web/*
var assets embed.FS

func Frontend() fs.FS {
	sub, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	return sub
}
