package scl

import (
	"io"
	"time"
)

/*
A FileSystem is a representation of entities with names and content that can be
listed using stangard glob syntax and read by name. The typical implementation
for this is a local disk filesystem, but it could be anything – records in a
database, objects on AWS S3, the contents of a zip file, virtual files stored
inside a binary, and so forth. A FileSystem is required to instantiate the
standard Parser implementation.
*/
type FileSystem interface {
	Glob(pattern string) ([]string, error)
	ReadCloser(path string) (content io.ReadCloser, lastModified time.Time, err error)
}
