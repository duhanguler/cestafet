//go:build !windows

package identity

import "os/exec"

// Quiet is a no-op outside Windows.
func Quiet(cmd *exec.Cmd) *exec.Cmd { return cmd }
