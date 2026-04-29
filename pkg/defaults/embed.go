package defaults

import "embed"

//go:embed clients.json source_profiles.json settings.json
var Assets embed.FS
