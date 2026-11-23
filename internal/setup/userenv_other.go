//go:build !windows

package setup

// userEnvConfigDirs returns CLAUDE_CONFIG_DIR from the persistent user
// environment; only Windows keeps one outside shell files.
func userEnvConfigDirs() []string { return nil }
