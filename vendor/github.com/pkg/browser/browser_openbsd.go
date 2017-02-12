package browser

import (
	"errors"
	"os/exec"
)

var errNoXdgOpen = errors.New("xdg-open: command not found - install xdg-utils from ports(8)")

func openBrowser(url string) error {
	err := runCmd("xdg-open", url)
	if e, ok := err.(*exec.Error); ok && e.Err == exec.ErrNotFound {
		return errNoXdgOpen
	}
	return err
}
