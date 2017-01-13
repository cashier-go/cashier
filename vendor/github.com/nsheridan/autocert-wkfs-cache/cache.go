package wkfscache

import (
	"os"
	"path/filepath"

	"go4.org/wkfs"

	"golang.org/x/crypto/acme/autocert"
	"golang.org/x/net/context"
)

type Cache string

// Get reads a certificate data from the specified file name.
func (d Cache) Get(ctx context.Context, name string) ([]byte, error) {
	name = filepath.Join(string(d), name)
	var (
		data []byte
		err  error
		done = make(chan struct{})
	)
	go func() {
		data, err = wkfs.ReadFile(name)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
	}
	if os.IsNotExist(err) {
		return nil, autocert.ErrCacheMiss
	}
	return data, err
}

// Put writes the certificate data to the specified file name.
// The file will be created with 0600 permissions.
func (d Cache) Put(ctx context.Context, name string, data []byte) error {
	if err := wkfs.MkdirAll(string(d), 0700); err != nil {
		return err
	}

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		if err := wkfs.WriteFile(filepath.Join(string(d), name), data, 0600); err != nil {
			return
		}
		// prevent overwriting the file if the context was cancelled
		if ctx.Err() != nil {
			return // no need to set err
		}
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}
	return err
}

// Delete removes the specified file name.
func (d Cache) Delete(ctx context.Context, name string) error {
	name = filepath.Join(string(d), name)
	var (
		err  error
		done = make(chan struct{})
	)
	go func() {
		err = wkfs.Remove(name)
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
