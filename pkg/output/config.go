package output

// PluginConfig holds configuration for creating an OutputPlugin.
// Additional fields will be added as concrete plugins are implemented.
type PluginConfig struct {
	OutputDir string
	IsLive    bool
}
