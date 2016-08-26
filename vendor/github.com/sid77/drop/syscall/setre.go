// +build !linux

package syscall

import (
	"syscall"
)

func Setuid(uid int) error {
	err := syscall.Setreuid(uid, uid)
	return err
}

func Setgid(gid int) error {
	err := syscall.Setregid(gid, gid)
	return err
}
