//go:build windows

package setup

import "golang.org/x/sys/windows/registry"

// userEnvConfigDirs returns CLAUDE_CONFIG_DIR from HKCU\Environment: a variable
// set with setx or the System Properties dialog is not visible in a terminal
// that was already open.
func userEnvConfigDirs() []string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	v, _, err := k.GetStringValue("CLAUDE_CONFIG_DIR")
	if err != nil || v == "" {
		return nil
	}
	return []string{v}
}
