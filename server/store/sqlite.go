//go:build cgo
// +build cgo

package store

import (
	_ "github.com/mattn/go-sqlite3" // required by sql driver
)
