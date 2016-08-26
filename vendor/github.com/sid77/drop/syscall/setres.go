// +build linux

package syscall

import (
	"syscall"
)

func Setuid(uid int) error {
	err := syscall.Setresuid(uid, uid, uid)
	return err
}

func Setgid(gid int) error {
	err := syscall.Setresgid(gid, gid, gid)
	return err
}
