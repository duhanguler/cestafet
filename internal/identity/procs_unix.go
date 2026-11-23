//go:build !windows

package identity

import (
	"os/exec"
	"strconv"
	"strings"
)

func procInfo(pid int) (Proc, bool) {
	out, err := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return Proc{}, false
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) < 2 {
		return Proc{}, false
	}
	ppid, err := strconv.Atoi(fields[0])
	if err != nil {
		return Proc{}, false
	}
	return Proc{PID: pid, PPID: ppid, Name: strings.Join(fields[1:], " ")}, true
}

// ExePath is not needed outside Windows: claude is found on PATH there.
func ExePath(pid int) string { return "" }
