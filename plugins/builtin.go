package bundled

import "embed"

//go:embed */manifest.json */main.js
var FS embed.FS
