package scl

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type diskFileSystem struct {
	basePath string
}

/*
NewDiskSystem creates a filesystem that uses the local disk, at an optional
base path. The default base path is the current working directory.
*/
func NewDiskSystem(basePath ...string) FileSystem {

	base := ""

	if len(basePath) > 0 {
		base = basePath[0]
	}

	return &diskFileSystem{base}
}

func (d *diskFileSystem) path(path string) string {
	return filepath.Join(d.basePath, strings.TrimPrefix(path, d.basePath))
}

func (d *diskFileSystem) Glob(pattern string) (out []string, err error) {
	return filepath.Glob(d.path(pattern))
}

func (d *diskFileSystem) ReadCloser(path string) (data io.ReadCloser, lastModified time.Time, err error) {

	reader, err := os.Open(d.path(path))

	if err != nil {
		return nil, time.Time{}, err
	}

	stat, err := reader.Stat()

	if err != nil {
		return nil, time.Time{}, err
	}

	return reader, stat.ModTime(), nil
}
