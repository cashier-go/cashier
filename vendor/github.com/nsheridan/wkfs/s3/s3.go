package s3

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"go4.org/wkfs"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// Options for registering the S3 wkfs.
// None of these are required and can be supplied to the aws client by other means.
type Options struct {
	Region    string
	AccessKey string
	SecretKey string
}

var _ wkfs.FileSystem = (*s3FS)(nil)

// Register the /s3/ filesystem as a well-known filesystem.
func Register(opts *Options) {
	if opts == nil {
		opts = &Options{}
	}
	config := &aws.Config{}
	// If region is unset the SDK will attempt to read the region from the environment.
	if opts.Region != "" {
		config.Region = aws.String(opts.Region)
	}
	// Attempt to use supplied credentials, otherwise fall back to the SDK.
	if opts.AccessKey != "" && opts.SecretKey != "" {
		config.Credentials = credentials.NewStaticCredentials(opts.AccessKey, opts.SecretKey, "")
	}
	s, err := session.NewSession(config)
	if err != nil {
		registerBrokenFS(err)
		return
	}
	sc := s3.New(s)
	if aws.StringValue(sc.Config.Region) == "" {
		registerBrokenFS(errors.New("could not find region configuration"))
		return
	}
	wkfs.RegisterFS("/s3/", &s3FS{
		sc: sc,
	})
}

func registerBrokenFS(err error) {
	wkfs.RegisterFS("/s3/", &s3FS{
		err: err,
	})
}

type s3FS struct {
	sc  *s3.S3
	err error
}

func (fs *s3FS) parseName(name string) (bucket, fileName string, err error) {
	if fs.err != nil {
		return "", "", fs.err
	}
	name = strings.TrimPrefix(name, "/s3/")
	i := strings.Index(name, "/")
	if i < 0 {
		return name, "", nil
	}
	return name[:i], name[i+1:], nil
}

// Open opens the named file for reading.
func (fs *s3FS) Open(name string) (wkfs.File, error) {
	bucket, fileName, err := fs.parseName(name)
	if err != nil {
		return nil, err
	}
	obj, err := fs.sc.GetObject(&s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &fileName,
	})
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case "NoSuchKey", "NoSuchBucket":
				return nil, os.ErrNotExist
			}
		}
		return nil, err
	}
	defer obj.Body.Close()
	slurp, err := ioutil.ReadAll(obj.Body)
	if err != nil {
		return nil, err
	}
	return &file{
		name:   name,
		Reader: bytes.NewReader(slurp),
	}, nil
}

func (fs *s3FS) Stat(name string) (os.FileInfo, error) { return fs.Lstat(name) }
func (fs *s3FS) Lstat(name string) (os.FileInfo, error) {
	bucket, fileName, err := fs.parseName(name)
	if err != nil {
		return nil, err
	}
	obj, err := fs.sc.GetObject(&s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &fileName,
	})
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "NoSuchKey" {
				return nil, os.ErrNotExist
			}
		}
	}
	if err != nil {
		return nil, err
	}
	return &statInfo{
		name: path.Base(fileName),
		size: *obj.ContentLength,
	}, nil
}

func (fs *s3FS) MkdirAll(path string, perm os.FileMode) error {
	_, err := fs.OpenFile(fmt.Sprintf("%s/", filepath.Clean(path)), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	return err
}

func (fs *s3FS) OpenFile(name string, flag int, perm os.FileMode) (wkfs.FileWriter, error) {
	bucket, filename, err := fs.parseName(name)
	if err != nil {
		return nil, err
	}
	switch flag {
	case os.O_WRONLY | os.O_CREATE | os.O_EXCL:
	case os.O_WRONLY | os.O_CREATE | os.O_TRUNC:
	default:
		return nil, fmt.Errorf("Unsupported OpenFlag flag mode %d on S3", flag)
	}
	if flag&os.O_EXCL != 0 {
		if _, err := fs.Stat(name); err == nil {
			return nil, os.ErrExist
		}
	}
	return NewS3file(bucket, filename, fs.sc)
}

func (fs *s3FS) Remove(name string) error {
	var err error
	bucket, filename, err := fs.parseName(name)
	if err != nil {
		return err
	}
	_, err = fs.sc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(filename),
	})
	return err
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
	panic("Stat not implemented on /s3/ files yet")
}
