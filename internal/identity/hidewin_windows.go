//go:build windows

package identity

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// Quiet keeps a console child (git, ssh) from flashing a window when cestafet
// runs without a console, as the MCP server does.
func Quiet(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd
}
