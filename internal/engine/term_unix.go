//go:build !windows

package engine

import (
	"os"
	"syscall"
	"unsafe"
)

// termWidth asks the terminal for its width (TIOCGWINSZ); 0 when f is not one.
func termWidth(f *os.File) int {
	var ws struct{ row, col, x, y uint16 }
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws))); e != 0 {
		return 0
	}
	return int(ws.col)
}
