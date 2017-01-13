package vaultfs

import (
	"bytes"
	"errors"
	"os"
	"path"
	"time"

	"github.com/nsheridan/cashier/server/config"
	"github.com/nsheridan/cashier/server/helpers/vault"
	"go4.org/wkfs"
)

// Register the /vault/ filesystem as a well-known filesystem.
func Register(vc *config.Vault) {
	if vc == nil {
		registerBrokenFS(errors.New("no vault configuration found"))
		return
	}
	client, err := vault.NewClient(vc.Address, vc.Token)
	if err != nil {
		registerBrokenFS(err)
		return
	}
	wkfs.RegisterFS("/vault/", &vaultFS{
		client: client,
	})
}

func registerBrokenFS(err error) {
	wkfs.RegisterFS("/vault/", &vaultFS{
		err: err,
	})
}

type vaultFS struct {
	err    error
	client *vault.Client
}

// Open opens the named file for reading.
func (fs *vaultFS) Open(name string) (wkfs.File, error) {
	secret, err := fs.client.Read(name)
	if err != nil {
		return nil, err
	}
	return &file{
		name:   name,
		Reader: bytes.NewReader([]byte(secret)),
	}, nil
}

func (fs *vaultFS) Stat(name string) (os.FileInfo, error) { return fs.Lstat(name) }
func (fs *vaultFS) Lstat(name string) (os.FileInfo, error) {
	secret, err := fs.client.Read(name)
	if err != nil {
		return nil, err
	}
	return &statInfo{
		name: path.Base(name),
		size: int64(len(secret)),
	}, nil
}

func (fs *vaultFS) MkdirAll(path string, perm os.FileMode) error { return nil }

func (fs *vaultFS) OpenFile(name string, flag int, perm os.FileMode) (wkfs.FileWriter, error) {
	return nil, errors.New("not implemented")
}

func (fs *vaultFS) Remove(path string) error {
	return fs.client.Delete(path)
}

type statInfo struct {
	name    string
	size    int64
	isDir   bool
	modtime time.Time
}

func (si *statInfo) IsDir() bool        { return si.isDir }
func (si *statInfo) ModTime() time.Time { return si.modtime }
func (si *statInfo) Mode() os.FileMode  { return 0644 }
func (si *statInfo) Name() string       { return path.Base(si.name) }
func (si *statInfo) Size() int64        { return si.size }
func (si *statInfo) Sys() interface{}   { return nil }

type file struct {
	name string
	*bytes.Reader
}

func (*file) Close() error   { return nil }
func (f *file) Name() string { return path.Base(f.name) }
func (f *file) Stat() (os.FileInfo, error) {
	return nil, errors.New("Stat not implemented on /vault/ files")
}
