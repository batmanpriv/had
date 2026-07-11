//go:build linux || darwin || freebsd || openbsd || netbsd || android

package core

import (
	"syscall"
	"unsafe"
)

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func getTermWidth() (int, error) {
	var ws winsize
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		1,
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 {
		return 0, errno
	}
	if ws.Col == 0 {
		return 0, syscall.EINVAL
	}
	return int(ws.Col), nil
}
