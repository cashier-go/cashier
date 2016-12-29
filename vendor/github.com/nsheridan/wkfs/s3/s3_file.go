package s3

import (
	"bytes"
	"errors"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3file represents a file in S3.
type S3file struct {
	bucket string
	name   string
	offset int
	closed bool

	s3api *s3.S3
}

// NewS3file initializes an S3file.
func NewS3file(bucket, name string, s3api *s3.S3) (*S3file, error) {
	return &S3file{
		bucket: bucket,
		name:   name,
		offset: 0,
		closed: false,
		s3api:  s3api,
	}, nil
}

// Write len(p) bytes to the file in S3.
// It returns the number of bytes written and an error, if any.
func (f *S3file) Write(p []byte) (n int, err error) {
	if f.closed {
		panic("read after close")
	}
	if f.offset != 0 {
		return 0, errors.New("Offset cannot be > 0")
	}
	readSeeker := bytes.NewReader(p)
	size := int(readSeeker.Size())
	obj := &s3.PutObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(f.name),
		Body:   readSeeker,
	}
	if _, err := f.s3api.PutObject(obj); err != nil {
		return 0, err
	}
	f.offset += size
	return size, nil
}

// Close the file, rendering it unusable.
func (f *S3file) Close() error {
	f.closed = true
	return nil
}
