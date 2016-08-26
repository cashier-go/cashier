package drop

import (
	"os/user"
	"strconv"

	"github.com/sid77/drop/syscall"
)

func DropPrivileges(runAsUser string) (err error) {
	usr, err := user.Lookup(runAsUser)
	if err != nil {
		return err
	}

	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return err
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return err
	}

	if err = syscall.Setgid(gid); err != nil {
		return err
	}

	if err = syscall.Setuid(uid); err != nil {
		return err
	}

	return nil
}
