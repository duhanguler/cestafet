//go:build windows

package identity

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func procInfo(pid int) (Proc, bool) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return Proc{}, false
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if int(e.ProcessID) == pid {
			return Proc{PID: pid, PPID: int(e.ParentProcessID), Name: windows.UTF16ToString(e.ExeFile[:]),
				Created: created(uint32(pid))}, true
		}
	}
	return Proc{}, false
}

// created returns the process creation time, or zero when the process cannot
// be opened (access denied, or it has exited).
func created(pid uint32) time.Time {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return time.Time{}
	}
	defer windows.CloseHandle(h)
	var c, x, k, u windows.Filetime
	if err := windows.GetProcessTimes(h, &c, &x, &k, &u); err != nil {
		return time.Time{}
	}
	return time.Unix(0, c.Nanoseconds())
}
